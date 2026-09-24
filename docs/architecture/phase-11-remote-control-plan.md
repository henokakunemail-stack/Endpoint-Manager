# Architectural Blueprint — Fase 11: Remote Control (Pure-Go Screen Capture & Input Relay)

**Dokumen Versi:** 1.0.0  
**Tanggal:** 2026-09-24  
**Status:** PROPOSED & READY FOR IMPLEMENTATION

---

## 1. Analisis Kebutuhan & Kendala Teknis

Modul **Remote Control (Desktop Sharing & Input Relay)** adalah fitur paling kritikal dalam solusi Enterprise Endpoint Management untuk dukungan teknis jarak jauh (*remote helpdesk assistance*).

### Batasan Arsitektur:
1. **Zero CGO / Pure-Go**: Lingkungan server dan agen tidak memiliki kompilator C (`cgo` dilarang). Seluruh penangkapan layar (*screen capture*), pengkodean *frame* citra (*image encoding*), dan *input synthesis* harus murni Go tanpa ketergantungan GCC/Clang.
2. **Outbound-Only Agent Connection**: Komputer cabang berada di belakang NAT/Firewall ketat tanpa port terbuka inbound. Seluruh koneksi remote control di-*relay* melalui server menggunakan WebSocket full-duplex (`/api/devices/{id}/remotecontrol/ws`).
3. **Dual Mode Kontrol**:
   - `view_only`: Teknisi hanya dapat melihat layar pengguna (observasi/training).
   - `full_control`: Teknisi dapat mengirimkan input mouse dan keyboard secara interaktif.

---

## 2. Arsitektur Komponen

```
+-------------------+             +-----------------------+             +-------------------+
|    Web Console    | <--- WSS ---> |   Enterprise Server   | <--- WSS ---> |    Remote Agent   |
| (React 19 Canvas) |  Web Browser  | (RemoteControl Relay) |  Branch Agent |  (GDI / BitBlt /  |
|                   |               |  & Session Authority  |               |    SendInput)     |
+-------------------+             +-----------------------+             +-------------------+
```

### 1. Protokol Pesan Remote Control (WebSocket Envelope)
- **`rc.init`**: Negosiasi resolusi, FPS, dan kualitas kompresi JPEG (`{ "fps": 10, "quality": 60, "mode": "full_control" }`).
- **`rc.frame`**: Pengiriman *frame* citra JPEG (disertai koordinat kursor, timestamp, dan dimensi layar).
- **`rc.mouse`**: Pengiriman aksi mouse teknisi: `{ "action": "move"|"down"|"up"|"wheel", "x": 1024, "y": 768, "button": "left"|"right"|"middle", "delta": 0 }`.
- **`rc.key`**: Pengiriman tombol keyboard: `{ "action": "down"|"up", "key": "Enter", "code": 13 }`.
- **`rc.stop`**: Penghentian sesi kontrol jarak jauh secara tertib.

### 2. Implementasi Agent Screen Capture & Input Synthesis (Windows):
- **Screen Capture**: Menggunakan Windows Win32 API (`GetDC`, `CreateCompatibleDC`, `CreateCompatibleBitmap`, `BitBlt`, `GetDIBits`, `ReleaseDC`) melalui syscall Go murni (`user32.dll` dan `gdi32.dll`).
- **JPEG Compression**: Menggunakan paket standar Go `image/jpeg` dengan parameter `Quality` adaptif (30-80%).
- **Input Synthesis**: Menggunakan Win32 API `SendInput` / `mouse_event` / `keybd_event` untuk memproyeksikan pergerakan mouse dan klik secara fisik pada desktop Windows target.

---

## 3. Skema Basis Data (Database Migration `0010_remote_control.sql`)

```sql
-- 0010_remote_control.sql
-- Remote control session management and auditing

CREATE TABLE IF NOT EXISTS remote_control_sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    session_mode TEXT NOT NULL DEFAULT 'full_control', -- 'full_control', 'view_only'
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'ended', 'rejected'
    frames_transmitted INTEGER NOT NULL DEFAULT 0,
    bytes_transmitted INTEGER NOT NULL DEFAULT 0,
    input_events_count INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rc_sessions_device ON remote_control_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_rc_sessions_operator ON remote_control_sessions(operator_id);
CREATE INDEX IF NOT EXISTS idx_rc_sessions_status ON remote_control_sessions(status);
```

---

## 4. Rute HTTP REST & WebSocket API

| Method | Endpoint | Akses Minimum | Keterangan |
|---|---|---|---|
| `POST` | `/api/devices/{id}/remotecontrol/session` | `technician` | Memulai sesi remote control baru dan membuat session ID. |
| `GET` | `/api/devices/{id}/remotecontrol/ws` | `technician` | WebSocket interaktif untuk teknisi (menerima frame, mengirim input mouse/keyboard). |
| `GET` | `/api/agent/devices/{id}/remotecontrol/ws` | Agent Secret | WebSocket streaming untuk agen perangkat. |
| `POST` | `/api/devices/{id}/remotecontrol/sessions/{sessionId}/stop` | `technician` | Mengakhiri sesi remote control. |
| `GET` | `/api/devices/{id}/remotecontrol/sessions` | `viewer` | Menampilkan riwayat sesi remote control perangkat. |

---

## 5. Rencana Pengujian

1. **Integration Test (`tests/integration/remote_control_test.go`)**:
   - Pembuatan dan penutupan sesi remote control di database.
   - Perekaman metrik sesi (`frames_transmitted`, `bytes_transmitted`, `input_events_count`).
   - Penegakan batasan hak akses RBAC (Technician vs Viewer).
2. **Live E2E Test (`scripts/e2e-remote-control.ps1`)**:
   - Membangun biner server dan mengotentikasi teknisi.
   - Inisialisasi sesi remote control.
   - Menghubungkan client WebSocket teknisi dan streaming mock JPEG frame.
   - Mengirimkan event mouse click dan keyboard press melalui WebSocket relay.
   - Mengakhiri sesi dan memverifikasi jejak audit forensik (`remotecontrol.session_start`, `remotecontrol.session_stop`).
