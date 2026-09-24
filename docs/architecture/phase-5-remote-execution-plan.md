# Fase 5 — Arsitektur & Perencanaan: Remote Execution & Interactive Terminal

**Status:** APPROVED FOR EXECUTION  
**Tanggal:** 2026-09-23  
**Target:** Remote Command Dispatch, Output Streaming, & Live Interactive Terminal (Enterprise RMM)

---

## 1. Kebutuhan & Spesifikasi Fungsional

Sistem Endpoint Management Enterprise memerlukan kapabilitas eksekusi perintah jarak jauh (*Remote Execution*) dan terminal interaktif langsung (*Live Terminal Session*) untuk memungkinkan teknisi dan administrator melakukan diagnosis, troubleshooting, pemeliharaan sistem, dan konfigurasi tanpa mengharuskan pengguna lokal logout atau terganggu.

### Fitur Utama:
1. **Single Command Remote Execution**:
   - Mendukung eksekusi perintah non-interaktif di background pada endpoint Windows (`powershell`, `cmd`) dan Linux/macOS (`bash`, `sh`).
   - Timeout eksekusi yang dapat dikonfigurasi (default 60 detik, maksimal 300 detik) untuk mencegah proses *hanging* tak terbatas.
   - Penangkapan output komprehensif: stdout, stderr, exit code, durasi eksekusi.
   - Hak akses istimewa: Proses berjalan dengan konteks akun agen (SYSTEM di Windows, root di Linux/macOS).
2. **Interactive Terminal Session (Live Shell)**:
   - Sesi terminal interaktif dupleks penuh (*full-duplex bidirectional streaming*) antara browser Web Console dan proses shell pada agen.
   - Server bertindak sebagai WebSocket multiplexer/relay yang menghubungkan operator web console dengan live agent.
   - Mendukung pengiriman input interaktif (`stdin`), penerimaan output instan (`stdout`/`stderr`), dan sinyal terminasi (`term.close` / `Ctrl+C`).
   - Pembersihan otomatis: Saat koneksi browser terputus atau sesi ditutup, proses shell anak pada agen langsung dihentikan (*graceful child process termination*) untuk mencegah *zombie processes*.
3. **Enterprise Security & Audit Trail**:
   - Strict RBAC: Hanya role `technician` dan `admin` yang diizinkan mengeksekusi perintah atau membuka terminal. Role `viewer` ditolak dengan HTTP 403 Forbidden.
   - Audit trail wajib: Setiap perintah yang dieksekusi, identitas operator (`user_id`), target endpoint (`device_id`), waktu mulai, dan exit code terekam permanen di tabel `audit_logs` dan `remote_executions`.
   - Pencegahan command injection pada level transport: Payload dikirim dalam struktur JSON terisolasi, bukan string interpolation di sisi server.
4. **Web Console UI**:
   - **Remote Command Runner**: Form input perintah dengan pilihan shell (`PowerShell`, `Command Prompt`, `Bash`), tombol Run, indikator status running/completed, exit code badge, dan terminal output viewer dengan dark theme.
   - **Interactive Live Terminal**: Jendela terminal interaktif berbasis browser dengan respons real-time.

---

## 2. Skema Database (`0005_remote_execution.sql`)

```sql
-- Tabel Riwayat Eksekusi Perintah Remote
CREATE TABLE IF NOT EXISTS remote_executions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    shell_type TEXT NOT NULL, -- 'powershell', 'cmd', 'bash', 'sh'
    command_text TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'running', 'completed', 'failed', 'timeout'
    exit_code INTEGER,
    output TEXT,
    error_message TEXT,
    started_at DATETIME NOT NULL,
    completed_at DATETIME
);

-- Tabel Riwayat Sesi Terminal Interaktif
CREATE TABLE IF NOT EXISTS terminal_sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    shell_type TEXT NOT NULL, -- 'powershell', 'cmd', 'bash', 'sh'
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'closed'
    created_at DATETIME NOT NULL,
    closed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_remote_exec_device ON remote_executions(device_id);
CREATE INDEX IF NOT EXISTS idx_remote_exec_status ON remote_executions(status);
CREATE INDEX IF NOT EXISTS idx_term_sessions_device ON terminal_sessions(device_id);
```

---

## 3. Protokol & Interaksi Antara Server, Agen, dan Web Console

### A. Eksekusi Perintah (Non-Interactive)
1. **Operator -> Server**:
   `POST /api/devices/{id}/exec`
   ```json
   {
     "shell": "powershell",
     "command": "Get-Service | Where-Object Status -eq 'Running'",
     "timeout_sec": 30
   }
   ```
2. **Server -> Agent (via WebSocket Hub)**:
   ```json
   {
     "type": "command",
     "id": "cmd-uuid",
     "command": "exec.run",
     "payload": {
       "execution_id": "exec-uuid",
       "shell": "powershell",
       "command": "Get-Service | Where-Object Status -eq 'Running'",
       "timeout_sec": 30
     }
   }
   ```
3. **Agent Execution**:
   - Agen mengeksekusi shell subprocess dengan `exec.CommandContext` berbatas timeout.
   - Mengumpulkan `stdout` dan `stderr`.
4. **Agent -> Server (via HTTP atau WebSocket)**:
   `POST /api/agent/executions/{id}/result`
   ```json
   {
     "execution_id": "exec-uuid",
     "status": "completed",
     "exit_code": 0,
     "output": "..."
   }
   ```
5. **Server -> Operator**:
   Menyimpan hasil di database dan mengembalikan detail eksekusi lengkap.

### B. Live Interactive Terminal
1. **Operator -> Server**:
   Operator membuka koneksi WebSocket di `/api/devices/{id}/terminal/ws?token=JWT&shell=powershell`.
2. **Server -> Agent**:
   Server meminta agen membuka session shell via command `term.open`:
   `{ "type": "command", "command": "term.open", "payload": { "session_id": "sess-uuid", "shell": "powershell" } }`
3. **Bidirectional Stream**:
   - Operator mengetik -> WebSocket frame `{ "type": "term.data", "session_id": "...", "data": "ls\r\n" }` -> Server me-relay ke Agent -> Agent menulis ke `stdin` proses shell.
   - Proses shell agen mengeluarkan output -> Agent mengirim WebSocket frame `{ "type": "term.data", "session_id": "...", "data": "output..." }` -> Server me-relay ke Operator browser.
4. **Termination & Cleanup**:
   Saat browser menutup tab atau menekan disconnect, frame `term.close` dikirim -> proses shell anak di-kill secara paksa dan memori dibebaskan.

---

## 4. Rencana Implementasi Bertahap

1. **Database Migration**: `server/core/db/migrations/0005_remote_execution.sql`
2. **Backend Engine**:
   - `server/modules/remote-exec/model.go`
   - `server/modules/remote-exec/repository.go`
   - `server/modules/remote-exec/handler.go`
   - `server/cmd/server/main.go` (Wiring module)
3. **Agent Remote Shell Engine**:
   - `agent/shared/remoteexec/executor.go`
   - `agent/shared/remoteexec/runner_windows.go`
   - `agent/shared/remoteexec/runner_linux.go`
   - `agent/shared/remoteexec/runner_darwin.go`
   - `agent/shared/remoteexec/terminal_session.go`
   - Registrasi command `exec.run`, `term.open`, `term.data`, `term.close` di `agent/cmd/agent/main.go`.
4. **Web Console UI**:
   - `web-console/src/components/RemoteExecModal.tsx` & `web-console/src/components/InteractiveTerminalModal.tsx`
   - Navigasi & aksi langsung dari `DevicesPage.tsx`.
5. **Testing & Audit**:
   - Unit tests & Integration tests (`tests/integration/remote_exec_test.go`).
   - Live E2E script `scripts/e2e-remote-exec.ps1`.
   - Update Production Readiness Scorecard.
