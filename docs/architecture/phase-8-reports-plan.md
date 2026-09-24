# Fase 8 — Architectural Blueprint: Reports & Export Engine

**Versi:** 1.0.0  
**Tanggal:** 2026-09-24  
**Status:** DRAFT (READY FOR IMPLEMENTATION)  
**Target Modul:** `server/modules/reports`, `web-console/src/pages/ReportsPage.tsx`

---

## 1. Latar Belakang & Kebutuhan Bisnis

Enterprise IT dan tim audit keamanan memerlukan laporan periodik dan ekspor data kepatuhan (*compliance reporting*) untuk keperluan regulasi (ISO 27001, SOC 2, audit lisensi, dan manajemen aset).

Tantangan utama modul ini:
1. **Streaming CSV & JSON Exports**: Menghasilkan file ekspor berukuran besar secara *streaming* tanpa memuat seluruh dataset ke memori server.
2. **Kategori Laporan yang Komprehensif**:
   - **Hardware Inventory**: Daftar seluruh endpoint dengan rincian CPU, RAM, disk usage, OS version, serial number, dan site.
   - **Software Asset & License**: Agregasi seluruh software terinstal di seluruh armada untuk analisis lisensi dan shadow IT.
   - **Patch Compliance**: Daftar patch yang hilang per perangkat, diurutkan berdasarkan severitas (Critical/Security).
   - **Deployment History**: Riwayat instalasi software, tingkat keberhasilan, dan log kegagalan.
   - **Audit Trail Export**: Ekspor jejak audit lengkap untuk analisis forensik keamanan.
3. **Kontrol Akses RBAC**: Laporan teknis dapat diunduh oleh `technician` dan `admin`, sedangkan laporan audit forensik hanya dapat diakses oleh `admin`.

---

## 2. REST API Endpoints

| Method | Endpoint | Akses RBAC | Deskripsi |
|---|---|---|---|
| `GET` | `/api/reports/inventory` | All (Viewer+) | Ekspor inventaris perangkat (query: `format=csv\|json`, `site`, `os`) |
| `GET` | `/api/reports/software` | All (Viewer+) | Agregasi software terinstal di armada (query: `format=csv\|json`) |
| `GET` | `/api/reports/patches` | All (Viewer+) | Ekspor kepatuhan patch armada (query: `format=csv\|json`, `severity`, `state`) |
| `GET` | `/api/reports/deployments` | All (Viewer+) | Riwayat tugas software deployment (query: `format=csv\|json`) |
| `GET` | `/api/reports/audit` | Admin only | Ekspor jejak audit lengkap (query: `format=csv\|json`, `from`, `to`, `action`) |

---

## 3. Rencana Eksekusi Bertahap

1. **Step 1**: Buat modul backend `server/modules/reports` (repository aggregations, CSV streamer, JSON formatter, handler).
2. **Step 2**: Daftarkan router `/api/reports/*` di `server/cmd/server/main.go`.
3. **Step 3**: Tulis unit & integration tests `tests/integration/reports_test.go`.
4. **Step 4**: Buat skrip E2E komprehensif `scripts/e2e-reports.ps1` dan uji langsung pada live binary.
5. **Step 5**: Buat laporan audit `docs/readiness-reports/phase-8-reports.md` dan perbarui scorecard.
