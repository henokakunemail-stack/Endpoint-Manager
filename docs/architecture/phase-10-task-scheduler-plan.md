# Architectural Blueprint — Fase 10: Task Scheduler & Script Repository

**Dokumen Versi:** 1.0.0  
**Tanggal:** 2026-09-24  
**Status:** PROPOSED & READY FOR IMPLEMENTATION

---

## 1. Analisis Kebutuhan & Tujuan

Pemeliharaan sistem endpoint enterprise (seperti pembersihan cache/temp files, rotasi log lokal, defragmentasi disk, backup konfigurasi, atau verifikasi kepatuhan keamanan) memerlukan otomasi terjadwal yang terpusat.

Modul **Task Scheduler & Script Repository** bertujuan untuk:
1. **Pustaka Skrip Terpusat (Script Repository)**:
   - Menyimpan repositori skrip pemeliharaan terverifikasi (`powershell`, `cmd`, `bash`, `sh`).
   - Setiap skrip memiliki hash SHA-256 untuk memvalidasi integritas sebelum dieksekusi.
   - Parameter default dan batasan timeout eksekusi per skrip.
2. **Penjadwalan Otomatis (Task Scheduler Engine)**:
   - Mendukung penargetan fleksibel: `device` (perangkat tunggal), `group` (grup cabang), atau `all` (seluruh armada).
   - Mendukung tipe jadwal:
     - `cron` (ekspresi waktu standar, misal pemeliharaan malam hari `0 2 * * *`).
     - `interval` (eksekusi berkala setiap N menit).
     - `once` (eksekusi terjadwal satu kali pada waktu tertentu).
3. **Eksekusi Asinkron via WebSocket Hub**:
   - Saat jadwal terpicu, server membuat record `scheduled_task_run` dan mendistribusikan perintah eksekusi ke agen online melalui WebSocket menggunakan envelope `transport.TypeCommand` dengan `command: "exec.run"`.
   - Agen mengeksekusi skrip di lingkungan terisolasi dengan timeout yang ketat, mengumpulkan `stdout`, `stderr`, dan `exit_code`.
4. **Riwayat & Pelacakan Status Tugas**:
   - Setiap perangkat mencatat status `pending`, `running`, `success`, `failed` beserta log keluaran lengkap untuk analisis dan pelaporan audit.
5. **Keamanan & Penegakan RBAC**:
   - Peran `viewer` hanya dapat melihat daftar skrip, jadwal, dan riwayat run.
   - Peran `technician` dapat memicu eksekusi skrip secara on-demand.
   - Peran `admin` memiliki hak penuh untuk menambah, mengubah, dan menghapus skrip serta jadwal pemeliharaan.
   - Seluruh aksi operator terekam dalam log audit forensik.

---

## 2. Skema Basis Data (Database Migration `0009_task_scheduler.sql`)

```sql
-- 0009_task_scheduler.sql
-- Script repository, task schedules, and scheduled execution runs

CREATE TABLE IF NOT EXISTS script_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    script_type TEXT NOT NULL, -- 'powershell', 'cmd', 'bash', 'sh'
    script_content TEXT NOT NULL,
    sha256_hash TEXT NOT NULL,
    default_args TEXT NOT NULL DEFAULT '',
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS task_schedules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    script_id TEXT NOT NULL REFERENCES script_templates(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL, -- 'device', 'group', 'all'
    target_id TEXT NOT NULL DEFAULT '',
    schedule_type TEXT NOT NULL, -- 'interval', 'cron', 'once'
    schedule_expr TEXT NOT NULL, -- cron expression or interval minutes
    is_enabled INTEGER NOT NULL DEFAULT 1,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS scheduled_task_runs (
    id TEXT PRIMARY KEY,
    schedule_id TEXT NOT NULL REFERENCES task_schedules(id) ON DELETE CASCADE,
    script_id TEXT NOT NULL REFERENCES script_templates(id),
    status TEXT NOT NULL DEFAULT 'running', -- 'running', 'completed', 'failed'
    triggered_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS scheduled_task_device_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES scheduled_task_runs(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id),
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'dispatched', 'success', 'failed'
    exit_code INTEGER,
    output_log TEXT,
    error_message TEXT,
    started_at DATETIME,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_task_schedules_enabled ON task_schedules(is_enabled);
CREATE INDEX IF NOT EXISTS idx_scheduled_runs_sched ON scheduled_task_runs(schedule_id);
CREATE INDEX IF NOT EXISTS idx_sched_device_runs_dev ON scheduled_task_device_runs(device_id);
```

---

## 3. Rute HTTP REST API

| Method | Endpoint | Akses Minimum | Keterangan |
|---|---|---|---|
| `GET` | `/api/scripts` | `viewer` | Menampilkan seluruh skrip dalam repositori. |
| `POST` | `/api/scripts` | `admin` | Menambahkan skrip baru dengan hash SHA-256. |
| `GET` | `/api/scripts/{id}` | `viewer` | Mengambil detail skrip tertentu. |
| `PUT` | `/api/scripts/{id}` | `admin` | Memperbarui skrip yang ada. |
| `DELETE` | `/api/scripts/{id}` | `admin` | Menghapus skrip dari repositori. |
| `GET` | `/api/schedules` | `viewer` | Menampilkan seluruh jadwal tugas pemeliharaan. |
| `POST` | `/api/schedules` | `admin` | Membuat jadwal tugas pemeliharaan baru. |
| `GET` | `/api/schedules/{id}` | `viewer` | Mengambil detail jadwal tertentu. |
| `PUT` | `/api/schedules/{id}` | `admin` | Memperbarui jadwal tugas pemeliharaan. |
| `DELETE` | `/api/schedules/{id}` | `admin` | Menghapus jadwal tugas pemeliharaan. |
| `POST` | `/api/schedules/{id}/trigger` | `technician` | Memicu eksekusi jadwal tugas secara manual/on-demand. |
| `GET` | `/api/schedules/{id}/runs` | `viewer` | Menampilkan riwayat eksekusi jadwal tugas. |
| `GET` | `/api/schedules/runs/{runId}` | `viewer` | Menampilkan detail hasil eksekusi per perangkat. |

---

## 4. Rencana Pengujian

1. **Integration Test (`tests/integration/task_scheduler_test.go`)**:
   - Pembuatan dan manipulasi repositori skrip dengan validasi SHA-256 hash.
   - Pembuatan jadwal tugas dan pemicuan eksekusi run.
   - Perekaman status tugas perangkat (`pending -> success`).
   - Penegakan batasan hak akses RBAC (Admin vs Technician vs Viewer).
2. **Live E2E Test (`scripts/e2e-task-scheduler.ps1`)**:
   - Pengujian biner live server dan agen aktif.
   - Pembuatan skrip pemeliharaan PowerShell/Sh.
   - Pemicuan run jadwal dan pengiriman perintah `exec.run` ke agen via WebSocket.
   - Validasi eksekusi non-interaktif agen, penangkapan exit code 0 dan output stdout.
   - Validasi jejak audit forensik (`script.create`, `schedule.create`, `schedule.run`).
