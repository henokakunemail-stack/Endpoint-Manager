# Fase 13 — Architectural Blueprint: Agent Self-Update & Rollout Management

**Status:** APPROVED FOR IMPLEMENTATION  
**Tanggal:** 2026-09-24  
**Arsitek:** Principal Systems Architect & Security Engineer  

---

## 1. Analisis Kebutuhan & Spesifikasi Enterprise

Di lingkungan enterprise terdistribusi dengan ribuan agen yang tersebar di kantor cabang dan perangkat lapangan (laptop karyawan mobile), melakukan pembaruan versi biner agen secara manual melalui SSH atau login lokal teknisi adalah hal yang mustahil dan memiliki risiko downtime tinggi.

Modul **Agent Self-Update & Rollout Management** harus memenuhi konstrain arsitektur ketat:
1. **Zero CGO / Pure Go Cross-Compilation**:
   - Seluruh logika pembaruan di server maupun agen harus dibangun dengan murni Go tanpa ketergantungan toolchain C eksternal (`CGO_ENABLED=0`) untuk 5 target arsitektur: `windows/amd64`, `linux/amd64`, `linux/arm64`, `darwin/amd64`, dan `darwin/arm64`.
2. **Kriptografi & Integritas Biner Ketat**:
   - Setiap rilis versi agen yang diunggah ke server harus memiliki metadata: `version`, `os_name`, `arch`, `sha256_checksum`, `file_size`, dan `changelog`.
   - Agen wajib memverifikasi kecocokan hash SHA-256 biner yang diunduh sebelum melakukan penggantian file. Jika hash tidak cocok (misal terkorupsi atau terjadi man-in-the-middle), agen membatalkan pembaruan dan melaporkan status `failed`.
3. **Penggantian Biner Atomik Lintas-OS (Atomic Binary Swap)**:
   - **Windows Quirk**: Pada sistem operasi Windows, proses yang sedang berjalan mengunci file eksekusinya (`ERROR_SHARING_VIOLATION` saat ditimpa). Namun, Windows mengizinkan me-rename file eksekusi yang sedang aktif!
     - Strategi Windows: Rename `agent.exe` menjadi `agent.exe.old`, lalu pindahkan biner baru `agent.new` ke `agent.exe`.
   - **Linux / macOS**: Atomic rename `os.Rename(newBinary, currentBinary)` dan pengaturan izin eksekusi `0755`.
4. **Mekanisme Rollback Otomatis Saat Gagal**:
   - Jika biner baru gagal dijalankan atau gagal lulus verifikasi kesehatan dasar (self-test probe), agen secara otomatis mengembalikan biner cadangan (`.old`) ke lokasi semula sehingga endpoint tidak pernah kehilangan koneksi ke manajemen pusat (*self-healing protection against bricking*).
5. **Manajemen Kampanye Rollout Bertahap (Staggered Batching)**:
   - Mencegah *thundering herd* dan lonjakan bandwidth WAN cabang saat seluruh armada mengunduh biner secara bersamaan.
   - Mendukung rollout bertahap: penentuan target (semua perangkat, grup cabang tertentu, atau perangkat tunggal pilot), ukuran batch (misal 10-25 perangkat per gelombang), dan interval jeda (*cooldown delay*).
6. **Penegakan RBAC & Jejak Audit Lengkap**:
   - Mengunggah rilis biner baru dan meluncurkan kampanye update dibatasi hanya untuk peran `admin`.
   - Teknisi (`technician`) dapat memicu update on-demand ke perangkat spesifik.
   - Peran `viewer` diblokir (403 Forbidden).
   - Seluruh aktivitas dicatat dalam log audit forensik (`agent_update.release_upload`, `agent_update.campaign_start`, `agent_update.device_dispatched`).

---

## 2. Skema Basis Data (`server/core/db/migrations/0012_agent_self_update.sql`)

```sql
-- 1. Repositori rilis biner agen
CREATE TABLE IF NOT EXISTS agent_releases (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    os_name TEXT NOT NULL,
    arch TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    sha256_checksum TEXT NOT NULL,
    changelog TEXT NOT NULL DEFAULT '',
    is_active INTEGER NOT NULL DEFAULT 1,
    uploaded_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    UNIQUE(version, os_name, arch)
);

-- 2. Kampanye peluncuran pembaruan bertahap (Rollout Campaigns)
CREATE TABLE IF NOT EXISTS update_campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    target_version TEXT NOT NULL,
    target_type TEXT NOT NULL, -- 'all', 'group', 'device'
    target_id TEXT NOT NULL DEFAULT '',
    batch_size INTEGER NOT NULL DEFAULT 20,
    stagger_interval_sec INTEGER NOT NULL DEFAULT 30,
    status TEXT NOT NULL DEFAULT 'draft', -- 'draft', 'in_progress', 'completed', 'cancelled'
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- 3. Pelacakan status pembaruan per perangkat
CREATE TABLE IF NOT EXISTS device_update_tasks (
    id TEXT PRIMARY KEY,
    campaign_id TEXT REFERENCES update_campaigns(id) ON DELETE SET NULL,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    from_version TEXT NOT NULL,
    target_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'downloading', 'verifying', 'swapping', 'success', 'rollback', 'failed'
    error_message TEXT NOT NULL DEFAULT '',
    dispatched_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_releases_lookup ON agent_releases(os_name, arch, is_active);
CREATE INDEX IF NOT EXISTS idx_device_update_tasks_dev ON device_update_tasks(device_id);
```

---

## 3. Komponen Arsitektur & Alur Kerja

```
[Administrator] 
       │ 1. POST /api/agent-updates/releases (Upload new binary + SHA256)
       ▼
[Server: AgentUpdate Module]
       │ 2. POST /api/devices/{id}/update/dispatch (or via Campaign)
       │    Compile target release for device OS/Arch
       ▼
[WebSocket Hub] ── Command: "update.apply" ──► [Agent Endpoint]
                                                       │ 3. Download binary from server
                                                       │ 4. Verify SHA-256 checksum
                                                       │ 5. Backup current binary (.old)
                                                       │ 6. Atomic swap into place
                                                       │ 7. Healthcheck verify
                                                       ▼
[Server: AgentUpdate Module] ◄── Status Report ────────┘
   - updates device_update_tasks
   - updates devices(agent_version)
   - creates audit log
```

---

## 4. Rencana Implementasi Bertahap

1. **Database Migration**: `server/core/db/migrations/0012_agent_self_update.sql`.
2. **Server Repository & Handler**: `server/modules/agentupdate/repository.go` dan `server/modules/agentupdate/handler.go`.
3. **Agent Self-Update Engine**: `agent/shared/update/engine.go` (download, checksum verify, atomic swap, backup & rollback).
4. **Server & Agent Wiring**: Integrasikan ke `server/cmd/server/main.go` dan `agent/cmd/agent/main.go`.
5. **Integration Tests**: `tests/integration/agent_update_test.go`.
6. **Live E2E Verification Harness**: `scripts/e2e-agent-update.ps1`.
7. **Readiness Report & Scorecard**: `docs/readiness-reports/phase-13-agent-self-update.md` dan `docs/readiness-reports/README.md`.
