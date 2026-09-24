# Fase 4 — Arsitektur & Perencanaan: Software Deployment

**Status:** APPROVED FOR EXECUTION  
**Tanggal:** 2026-09-23  
**Target:** Modul Distribusi & Instalasi Perangkat Lunak Jarak Jauh (Enterprise RMM)

---

## 1. Kebutuhan & Spesifikasi Teknis

Sistem Endpoint Management Enterprise membutuhkan kapabilitas untuk mendistribusikan dan menginstal paket perangkat lunak secara otomatis ke ratusan endpoint yang tersebar di berbagai kantor cabang tanpa membuka port inbound pada jaringan lokal endpoint.

### Fitur Utama:
1. **Package Repository**:
   - Mendukung penyimpanan paket installer: `.msi`, `.exe`, `.deb`, `.rpm`, `.pkg`, `.zip`, `.ps1`, `.sh`.
   - Perhitungan otomatis SHA-256 hash saat upload untuk menjamin integritas biner dan anti-tampering.
   - Parameter instalasi silent kustom (misal `/qn /norestart` untuk MSI, `/S` atau `/silent` untuk EXE).
   - Parameter uninstalasi dan deteksi versi terpasang.
2. **Staggered Job Dispatcher & Queue**:
   - Pengiriman pekerjaan instalasi ke perangkat individual atau grup perangkat (static groups dari Fase 2).
   - *Staggered rollout* (batch 10–25 perangkat per gelombang) untuk mencegah saturasi bandwidth WAN kantor cabang.
   - Timeout execution dan retry mechanism jika perangkat offline saat jadwal dimulai.
3. **Agent Installer Engine (Multi-OS)**:
   - Pengunduhan paket via HTTP/HTTPS chunked transfer langsung dari server pusat dengan autentikasi agent token.
   - Verifikasi SHA-256 sebelum eksekusi; jika hash tidak cocok, paket langsung dihapus dan error `ChecksumMismatch` dilaporkan.
   - Eksekusi instalasi di background dengan hak akses administratif (SYSTEM di Windows, root di Linux/macOS).
   - Penangkapan stdout/stderr dan exit code proses installer.
   - Pembersihan biner sementara (*cleanup temp file*) setelah instalasi selesai.
4. **Web Console UI**:
   - Manajemen katalog paket (tambah paket, konfigurasi silent flags, upload file biner).
   - Deployment wizard: pilih paket, pilih target (perangkat atau grup), jadwalkan sekarang atau nanti.
   - Live status tracker: pemantauan status per target (*Pending*, *Downloading*, *Installing*, *Success*, *Failed*) dengan progress bar dan detail error log.

---

## 2. Skema Database (`0004_software_deployment.sql`)

```sql
-- Tabel Paket Perangkat Lunak di Repositori
CREATE TABLE IF NOT EXISTS software_packages (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    os_target TEXT NOT NULL, -- 'windows', 'linux', 'macos'
    package_type TEXT NOT NULL, -- 'msi', 'exe', 'deb', 'rpm', 'pkg', 'script'
    file_name TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    install_args TEXT NOT NULL DEFAULT '',
    uninstall_args TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Tabel Deployment Job (Kumpulan tugas deployment)
CREATE TABLE IF NOT EXISTS software_deployments (
    id TEXT PRIMARY KEY,
    package_id TEXT NOT NULL REFERENCES software_packages(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL, -- 'device', 'group', 'all'
    target_id TEXT NOT NULL,
    created_by TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running', -- 'running', 'completed', 'failed', 'cancelled'
    created_at DATETIME NOT NULL,
    completed_at DATETIME
);

-- Tabel Tugas per Endpoint
CREATE TABLE IF NOT EXISTS deployment_tasks (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL REFERENCES software_deployments(id) ON DELETE CASCADE,
    package_id TEXT NOT NULL REFERENCES software_packages(id),
    device_id TEXT NOT NULL REFERENCES devices(id),
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'dispatched', 'downloading', 'installing', 'success', 'failed'
    exit_code INTEGER,
    output_log TEXT,
    error_message TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_deploy_tasks_device ON deployment_tasks(device_id);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_deploy ON deployment_tasks(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_status ON deployment_tasks(status);
```

---

## 3. Protokol & Interaksi Antara Server dan Agent

1. **Job Dispatch via WebSocket Command**:
   Server mengirim envelope command ke agent yang sedang online:
   ```json
   {
     "type": "command",
     "id": "cmd-uuid",
     "command": "software.install",
     "payload": {
       "task_id": "task-uuid",
       "package_id": "pkg-uuid",
       "package_name": "AgentUtility",
       "download_url": "/api/agent/packages/pkg-uuid/download",
       "file_name": "installer.msi",
       "package_type": "msi",
       "sha256": "abcdef123456...",
       "install_args": "/qn /norestart"
     }
   }
   ```

2. **Agent Progress Reporting**:
   Agent melaporkan status secara bertahap ke server via WebSocket telemetry:
   - Status `downloading` (persentase)
   - Status `installing`
   - Status `completed` (exit code, output log) atau `failed` (error reason)

3. **Database & Audit Update**:
   Server mengupdate tabel `deployment_tasks`, memperbarui agregasi di `software_deployments`, dan mencatat entri di `audit_logs`.

---

## 4. Rencana Implementasi Bertahap

1. **Database Schema**: `server/core/db/migrations/0004_software_deployment.sql`.
2. **Backend Engine**:
   - `server/modules/software-deployment/model.go`
   - `server/modules/software-deployment/repository.go`
   - `server/modules/software-deployment/handler.go` (Package CRUD, binary upload/download streaming, deployment dispatch)
3. **Agent Installer**:
   - `agent/shared/software/installer.go`
   - `agent/windows/installer_windows.go`
   - `agent/linux/installer_linux.go`
   - `agent/macos/installer_darwin.go`
4. **Web Console UI**:
   - `web-console/src/pages/SoftwarePage.tsx`
   - Update `Navbar.tsx` dan `App.tsx`
5. **Testing & Audit**:
   - Unit tests Go
   - Live E2E script `scripts/e2e-software-deployment.ps1`
