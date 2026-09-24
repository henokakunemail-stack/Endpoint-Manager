# Architectural Blueprint — Fase 9: Alerting & Notification Engine

**Dokumen Versi:** 1.0.0  
**Tanggal:** 2026-09-24  
**Status:** PROPOSED & READY FOR IMPLEMENTATION

---

## 1. Analisis Kebutuhan & Tujuan

Dalam operasional IT enterprise lintas cabang, insiden perangkat (seperti kehabisan ruang disk, server cabang mati mendadak, atau akumulasi patch kritikal yang belum terpasang) harus segera terdeteksi dan dikomunikasikan secara proaktif kepada tim IT Helpdesk.

Modul **Alerting & Notification Engine** bertujuan:
1. **Evaluasi Otomatis & Cerdas**: Mengevaluasi metrik armada endpoint terhadap aturan ambang batas (*threshold rules*) seperti kapasitas disk rendah (`disk_low`), RAM kritis (`ram_high`), perangkat offline (`device_offline`), dan patch keamanan kritis yang hilang (`critical_patch_missing`).
2. **Siklus Hidup Insiden (Alert Lifecycle)**:
   - `open`: Kondisi anomalitas terdeteksi pertama kali.
   - `acknowledged`: Teknisi telah mengonfirmasi sedang menangani insiden.
   - `resolved`: Kondisi telah kembali normal atau diselesaikan secara manual oleh operator.
3. **Pencegahan Badai Notifikasi (Deduplikasi & Cooldown)**:
   - Jika anomali yang sama terdeteksi berulang kali pada perangkat yang sama, sistem tidak membuat record baru melainkan memperbarui `trigger_count` dan `last_triggered_at`.
4. **Notifikasi Multi-Channel**:
   - **In-App Notification Feed**: Menyediakan endpoint query untuk notifikasi realtime bagi operator di Web Console.
   - **Outbound Webhook Delivery**: Mengirimkan webhook HTTP POST dengan payload JSON terstruktur ke saluran eksternal (Slack, Discord, MS Teams, atau ITSM/SIEM).
5. **Keamanan & RBAC Enterprise**:
   - Peran `viewer` hanya dapat melihat daftar insiden dan aturan.
   - Peran `technician` dapat mengonfirmasi (*acknowledge*) dan menyelesaikan (*resolve*) insiden.
   - Peran `admin` memiliki hak penuh untuk menambah, mengedit, dan menghapus aturan alerting (*rules*).

---

## 2. Skema Basis Data (Database Migration `0008_alerting.sql`)

```sql
-- 0008_alerting.sql
-- Alert rules, active incidents, and notification logs

CREATE TABLE IF NOT EXISTS alert_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    rule_type TEXT NOT NULL, -- 'disk_low', 'ram_high', 'device_offline', 'critical_patch'
    threshold_val REAL NOT NULL, -- e.g. 10.0 for disk_free_pct < 10%
    severity TEXT NOT NULL, -- 'info', 'warning', 'critical'
    webhook_url TEXT NOT NULL DEFAULT '',
    is_enabled INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_incidents (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    severity TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open', -- 'open', 'acknowledged', 'resolved'
    trigger_count INTEGER NOT NULL DEFAULT 1,
    acknowledged_by TEXT,
    acknowledged_at DATETIME,
    resolved_by TEXT,
    resolved_at DATETIME,
    first_triggered_at DATETIME NOT NULL,
    last_triggered_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_incidents_status ON alert_incidents(status);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_device ON alert_incidents(device_id);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_rule ON alert_incidents(rule_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_dedup ON alert_incidents(rule_id, device_id) WHERE status != 'resolved';
```

---

## 3. Rute HTTP REST API

| Method | Endpoint | Akses Minimum | Keterangan |
|---|---|---|---|
| `GET` | `/api/alerts/rules` | `viewer` | Menampilkan seluruh aturan alerting terkonfigurasi. |
| `POST` | `/api/alerts/rules` | `admin` | Membuat aturan alerting baru. |
| `PUT` | `/api/alerts/rules/{id}` | `admin` | Memperbarui aturan alerting. |
| `DELETE` | `/api/alerts/rules/{id}` | `admin` | Menghapus aturan alerting. |
| `GET` | `/api/alerts/incidents` | `viewer` | Menampilkan daftar insiden aktif / terselesaikan dengan filter status & severity. |
| `POST` | `/api/alerts/incidents/{id}/acknowledge` | `technician` | Mengubah status insiden menjadi `acknowledged`. |
| `POST` | `/api/alerts/incidents/{id}/resolve` | `technician` | Mengubah status insiden menjadi `resolved`. |
| `POST` | `/api/alerts/evaluate` | `technician` | Memicu evaluasi seluruh aturan secara on-demand. |

---

## 4. Evaluator Engine

Engine pengecekan berjalan di latar belakang (*background worker*) secara periodik dan dapat dipicu secara *on-demand*:
1. **Disk Check**: Mencari perangkat dengan `device_inventory.hw_disk_free_pct < rule.threshold_val`.
2. **Device Offline Check**: Mencari perangkat dengan `devices.status = 'offline'` atau durasi `last_seen_at` melebihi threshold.
3. **Critical Patch Check**: Mencari perangkat dengan `device_patches.severity = 'critical'` dan `installed_state = 'missing'`.
4. **Webhook Dispatcher**: Jika `webhook_url` terkonfigurasi, kirimkan HTTP POST dengan timeout 5 detik secara non-blocking (*goroutine*).

---

## 5. Rencana Pengujian

1. **Integration Test (`tests/integration/alerting_test.go`)**:
   - Verifikasi evaluasi aturan disk rendah, deduplikasi insiden berulang, transisi status `open -> acknowledged -> resolved`, dan penegakan hak akses RBAC.
2. **Live E2E Test (`scripts/e2e-alerting.ps1`)**:
   - Membangun biner server, mengotentikasi admin, technician, dan viewer.
   - Membuat rule baru via Admin API.
   - Memicu evaluasi dan memverifikasi insiden otomatis terbuat.
   - Menguji deduplikasi (trigger berulang meningkatkan `trigger_count`).
   - Teknisi meng-acknowledge dan me-resolve insiden.
   - Memvalidasi pembatasan RBAC dan rekaman jejak audit.
