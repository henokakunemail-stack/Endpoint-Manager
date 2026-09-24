# Fase 14 — Architectural Blueprint: Asset & License Management

**Status:** APPROVED FOR IMPLEMENTATION  
**Tanggal:** 2026-09-24  
**Arsitek:** Principal Systems Architect & Financial Compliance Specialist  

---

## 1. Analisis Kebutuhan & Spesifikasi Enterprise

Dalam tata kelola TI korporat berskala besar, pengelolaan perangkat fisik (*hardware assets*) dan lisensi perangkat lunak (*software licenses*) merupakan fondasi penting untuk kepatuhan hukum (*regulatory compliance*), efisiensi pengeluaran anggaran TI, dan kesiapan audit vendor (seperti Microsoft, Oracle, Adobe).

Modul **Asset & License Management** dirancang untuk menghubungkan data inventaris teknis perangkat yang dikumpulkan oleh agen dengan data finansial, operasional, dan kepemilikan lisensi.

Modul ini mencakup:
1. **Pengelolaan Siklus Hidup Aset Fisik (Hardware Asset Lifecycle)**:
   - Pelacakan metadata aset: nomor tag aset (`asset_tag`), nomor seri perangkat, model, vendor/pemasok, tanggal pembelian, biaya perolehan (`purchase_cost`), masa garansi (`warranty_expires_at`), lokasi kantor cabang/site, dan departemen penanggung jawab.
   - Status siklus hidup: `in_use` (aktif digunakan), `in_stock` (cadangan/gudang), `in_repair` (dalam perbaikan), `retired` (purna tugas), dan `disposed` (dihapusbukukan).
   - Penautan langsung ke entitas perangkat aktif di tabel `devices`.
2. **Kepatuhan Lisensi Perangkat Lunak (Software License Compliance Engine)**:
   - Pelacakan kontrak lisensi: nama aplikasi, penerbit (*publisher*), tipe lisensi (`per_device`, `per_user`, `site_license`, `subscription`), jumlah kursi/kapasitas yang dibeli (`total_seats`), tanggal kadaluarsa/perpanjangan, dan biaya lisensi tahunan.
   - **Rekonsiliasi Otomatis (Automated Fleet Reconciliation)**:
     - Mesin pencocokan otomatis antara nama perangkat lunak terinstal di seluruh armada agen dengan entri lisensi terdaftar.
     - Menghitung kursi terpakai (`used_seats`) secara real-time.
     - Menentukan status kepatuhan:
       - `compliant`: Jumlah terinstal <= jumlah kursi dibeli.
       - `over_allocated`: Pelanggaran audit lisensi (jumlah terinstal > jumlah kursi dibeli).
       - `expiring_soon`: Lisensi akan kadaluarsa dalam 30 hari.
       - `expired`: Lisensi telah melewati tanggal jatuh tempo.
3. **Agregasi Metrik Finansial & Audit Readiness**:
   - `GET /api/assets/summary`: Valuasi total aset perangkat, jumlah perangkat mendekati masa habis garansi, dan skor kepatuhan lisensi armada (*license compliance score*).
   - `GET /api/licenses/compliance`: Ringkasan audit kursi per aplikasi untuk laporan ke pimpinan dan vendor audit.
4. **Penegakan RBAC Ketat**:
   - Mutasi data aset dan lisensi (`POST/PUT/DELETE /api/assets`, `/api/licenses`) mewajibkan peran minimum `technician` atau `admin`.
   - Peran `viewer` dibatasi hanya melihat laporan dan status kepatuhan (read-only).
5. **Jejak Audit Forensik**:
   - Seluruh mutasi dicatat secara permanen (`asset.create`, `asset.update`, `asset.delete`, `license.create`, `license.update`, `license.delete`).

---

## 2. Skema Basis Data (`server/core/db/migrations/0013_asset_license_management.sql`)

```sql
-- 1. Aset Fisik Perangkat Keras
CREATE TABLE IF NOT EXISTS hardware_assets (
    id TEXT PRIMARY KEY,
    asset_tag TEXT NOT NULL UNIQUE,
    device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    model_name TEXT NOT NULL,
    serial_number TEXT NOT NULL DEFAULT '',
    vendor TEXT NOT NULL DEFAULT '',
    site TEXT NOT NULL DEFAULT '',
    department TEXT NOT NULL DEFAULT '',
    assigned_user TEXT NOT NULL DEFAULT '',
    purchase_date DATETIME,
    purchase_cost REAL NOT NULL DEFAULT 0.0,
    warranty_expires_at DATETIME,
    status TEXT NOT NULL DEFAULT 'in_use', -- 'in_use', 'in_stock', 'in_repair', 'retired', 'disposed'
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- 2. Kontrak & Hak Lisensi Perangkat Lunak
CREATE TABLE IF NOT EXISTS software_licenses (
    id TEXT PRIMARY KEY,
    software_name TEXT NOT NULL,
    publisher TEXT NOT NULL DEFAULT '',
    license_key TEXT NOT NULL DEFAULT '',
    license_type TEXT NOT NULL DEFAULT 'per_device', -- 'per_device', 'per_user', 'site_license', 'subscription'
    total_seats INTEGER NOT NULL DEFAULT 1,
    cost REAL NOT NULL DEFAULT 0.0,
    purchased_at DATETIME,
    expires_at DATETIME,
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- 3. Alokasi Lisensi Eksplisit ke Perangkat
CREATE TABLE IF NOT EXISTS license_allocations (
    id TEXT PRIMARY KEY,
    license_id TEXT NOT NULL REFERENCES software_licenses(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    allocated_at DATETIME NOT NULL,
    allocated_by TEXT NOT NULL,
    UNIQUE(license_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_hardware_assets_dev ON hardware_assets(device_id);
CREATE INDEX IF NOT EXISTS idx_hardware_assets_status ON hardware_assets(status);
CREATE INDEX IF NOT EXISTS idx_software_licenses_name ON software_licenses(software_name);
```

---

## 3. Rencana Implementasi

1. **Migration SQL**: `server/core/db/migrations/0013_asset_license_management.sql`.
2. **Server Repository**: `server/modules/assetlicense/repository.go`.
3. **Server Handler & Reconciliation Engine**: `server/modules/assetlicense/handler.go`.
4. **Server Wiring**: Registrasi modul di `server/cmd/server/main.go`.
5. **Integration Tests**: `tests/integration/asset_license_test.go`.
6. **Live E2E Verification Script**: `scripts/e2e-asset-license.ps1`.
7. **Readiness Report & Scorecard**: `docs/readiness-reports/phase-14-asset-license-management.md` dan `docs/readiness-reports/README.md`.
