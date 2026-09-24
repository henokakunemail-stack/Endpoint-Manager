# Fase 6 — Architectural Blueprint: Patch Management & OS Updates

**Versi:** 1.0.0  
**Tanggal:** 2026-09-23  
**Status:** DRAFT (REVIEWED & READY FOR IMPLEMENTATION)  
**Target Modul:** `server/modules/patch-management`, `agent/shared/patch`, `web-console/src/pages/PatchesPage.tsx`

---

## 1. Latar Belakang & Kebutuhan Bisnis

Patch Management adalah kapabilitas kritikal dalam manajemen endpoint enterprise untuk menjamin kepatuhan keamanan (*security compliance*), menutup celah kerentanan (*CVE mitigation*), dan menjaga stabilitas sistem operasi di ratusan endpoint cabang tanpa membebani bandwidth jaringan secara berlebihan.

Tantangan utama yang harus diselesaikan pada modul ini:
1. **Heterogenitas Sistem Operasi**: Mendukung pemindaian dan instalasi patch pada Windows (Windows Update Agent / WUA), Linux (APT dan DNF/YUM), serta macOS (`softwareupdate`).
2. **Konektivitas Outbound-Only**: Perintah pemindaian (*scan*) dan instalasi (*install*) harus dikirim melalui koneksi persisten WebSocket yang sudah dibuka oleh agen, tanpa port masuk (*no inbound ports*).
3. **Visibilitas Tingkat Keparahan (Severity & Classification)**: Membedakan patch berdasarkan tingkat urgensi (*Critical*, *Security*, *Definition*, *Rollup*, *Normal*) agar administrator dapat memprioritaskan pembaruan paling berisiko.
4. **Kontrol Reboot (Reboot Management)**: Mendeteksi apakah sistem memerlukan restart setelah instalasi patch, serta memberikan opsi kebijakan reboot (`no_reboot`, `reboot_if_needed`).
5. **Audit Forensik & RBAC**: Seluruh pemindaian, persetujuan patch, dan instalasi harus tercatat dalam tabel audit dan hanya dapat dieksekusi oleh peran `technician` dan `admin`.

---

## 2. Skema Basis Data (`0006_patch_management.sql`)

```sql
-- 0006_patch_management.sql
-- Fase 6: Patch Management & OS Updates

-- Katalog patch yang ditemukan pada seluruh endpoint armada
CREATE TABLE IF NOT EXISTS device_patches (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    patch_id TEXT NOT NULL,         -- e.g. KB5034441 (Windows) or curl-7.88.1 (Linux)
    title TEXT NOT NULL,
    description TEXT,
    severity TEXT NOT NULL DEFAULT 'unspecified', -- critical, important, moderate, low, unspecified
    category TEXT NOT NULL DEFAULT 'security',    -- security, critical, definition, updates, feature
    kb_id TEXT,                     -- Windows KB identifier if applicable
    size_bytes INTEGER DEFAULT 0,
    installed_state TEXT NOT NULL DEFAULT 'missing', -- missing, installed, pending
    reboot_required INTEGER NOT NULL DEFAULT 0,
    discovered_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE(device_id, patch_id)
);

CREATE INDEX IF NOT EXISTS idx_patches_device ON device_patches(device_id);
CREATE INDEX IF NOT EXISTS idx_patches_severity ON device_patches(severity);
CREATE INDEX IF NOT EXISTS idx_patches_state ON device_patches(installed_state);
CREATE INDEX IF NOT EXISTS idx_patches_patchid ON device_patches(patch_id);

-- Riwayat tugas instalasi patch
CREATE TABLE IF NOT EXISTS patch_install_jobs (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    patch_ids TEXT NOT NULL,        -- JSON array string: ["KB5034441", "KB5034123"]
    status TEXT NOT NULL DEFAULT 'pending', -- pending, dispatched, installing, completed, failed
    reboot_policy TEXT NOT NULL DEFAULT 'no_reboot', -- no_reboot, reboot_if_needed
    reboot_required INTEGER NOT NULL DEFAULT 0,
    output_log TEXT,
    error_message TEXT,
    started_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_patch_jobs_device ON patch_install_jobs(device_id);
CREATE INDEX IF NOT EXISTS idx_patch_jobs_operator ON patch_install_jobs(operator_id);
CREATE INDEX IF NOT EXISTS idx_patch_jobs_status ON patch_install_jobs(status);
```

---

## 3. Protokol Envelope Transport & REST API

### 3.1 Envelope WebSocket

1. **`patch.scan`** (Server -> Agent):
   ```json
   {
     "type": "cmd",
     "id": "job-uuid-1",
     "command": "patch.scan",
     "payload": {}
   }
   ```

2. **`patch.install`** (Server -> Agent):
   ```json
   {
     "type": "cmd",
     "id": "job-uuid-2",
     "command": "patch.install",
     "payload": {
       "job_id": "job-uuid-2",
       "patch_ids": ["KB5034441"],
       "reboot_policy": "no_reboot"
     }
   }
   ```

### 3.2 REST API Endpoints

| Method | Endpoint | Akses RBAC | Deskripsi |
|---|---|---|---|
| `GET` | `/api/patches/summary` | All (Viewer+) | Metrik armada: total missing patches, critical CVEs, reboot pending |
| `GET` | `/api/devices/{id}/patches` | All (Viewer+) | Daftar patch pada device tertentu (filter missing/installed) |
| `POST` | `/api/devices/{id}/patches/scan` | Technician, Admin | Memicu pemindaian patch on-demand ke agen |
| `POST` | `/api/devices/{id}/patches/install` | Technician, Admin | Memicu instalasi patch tertentu ke agen |
| `GET` | `/api/devices/{id}/patches/jobs` | All (Viewer+) | Riwayat tugas instalasi patch pada endpoint |
| `POST` | `/api/agent/patches/scan-report` | Agent (Token Auth) | Agen mengunggah hasil pemindaian patch terbaru |
| `POST` | `/api/agent/patches/install-result` | Agent (Token Auth) | Agen melaporkan hasil eksekusi instalasi patch |

---

## 4. Arsitektur Komponen Agen (`agent/shared/patch`)

### 4.1 Windows Scanner & Installer (`scanner_windows.go`, `installer_windows.go`)
- **Scanner**: Menggunakan skrip PowerShell terisolasi yang memanfaatkan COM API `Microsoft.Update.Session` untuk menanyakan pembaruan yang belum terinstal:
  ```powershell
  $Session = New-Object -ComObject Microsoft.Update.Session
  $Searcher = $Session.CreateUpdateSearcher()
  $Criteria = "IsInstalled=0 and Type='Software'"
  $SearchResult = $Searcher.Search($Criteria)
  # Mengambil Title, KBArticleIDs, MsrcSeverity, RebootRequired, Categories
  ```
  Menghasilkan JSON terstruktur berisi daftar patch yang belum terpasang.
- **Installer**: Mengunduh dan menginstal update terpilih menggunakan `Microsoft.Update.Installer`:
  ```powershell
  $Session = New-Object -ComObject Microsoft.Update.Session
  # Download dan install spesifik KB atau patch
  ```

### 4.2 Linux Scanner & Installer (`scanner_linux.go`, `installer_linux.go`)
- **Scanner**:
  - Debian/Ubuntu: `apt-get -s dist-upgrade` atau `apt list --upgradable`
  - RHEL/Fedora: `dnf check-update` / `yum check-update`
- **Installer**:
  - `apt-get install -y --only-upgrade <package>`
  - `dnf update -y <package>`

### 4.3 macOS Scanner & Installer (`scanner_darwin.go`, `installer_darwin.go`)
- **Scanner**: `softwareupdate -l`
- **Installer**: `softwareupdate -i <label> --no-scan`

---

## 5. Web Console UI (`web-console/src/pages/PatchesPage.tsx`)
- Menu navigasi baru: **Patch Management** (`/patches`).
- **Fleet Patch Overview Cards**:
  - Total Missing Patches
  - Critical Security Updates
  - Endpoints Pending Reboot
  - Endpoints Up to Date
- **Patch Catalog Table**:
  - Nama Patch / KB ID
  - Tingkat Keparahan (*Critical* merah, *Important* oranye, *Moderate* kuning, *Low* biru)
  - Kategori (*Security*, *Critical*, dll.)
  - Endpoint terdampak
  - Aksi: "Scan Fleet", "Install Patches"
- **Per-Device Patch Inspector Modal**:
  - Membuka daftar patch lengkap per mesin dari halaman `/devices` atau `/patches`.

---

## 6. Rencana Eksekusi Bertahap

1. **Step 1**: Buat skrip migrasi database `server/core/db/migrations/0006_patch_management.sql`.
2. **Step 2**: Implementasikan modul backend `server/modules/patch-management` (model, repository, handler, routing).
3. **Step 3**: Implementasikan modul agen `agent/shared/patch` (scanner & installer multi-OS) dan daftarkan handler di `agent/cmd/agent/main.go`.
4. **Step 4**: Integrasikan antarmuka Web Console `PatchesPage.tsx` dan tambahkan tab patch di navigasi & device detail.
5. **Step 5**: Tulis unit & integration tests `tests/integration/patch_management_test.go`.
6. **Step 6**: Buat skrip E2E komprehensif `scripts/e2e-patch-management.ps1` dan uji langsung pada live binary.
7. **Step 7**: Buat laporan audit kesiapan `docs/readiness-reports/phase-6-patch-management.md` dan perbarui central scorecard.
