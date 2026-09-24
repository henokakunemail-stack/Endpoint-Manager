# Fase 3 — Dashboard (Versi Awal) & Web Console

**Modul:** Dashboard & Web Console (Central Administration UI)
**Tanggal:** 2026-09-23
**Status:** `IN PROGRESS`

---

## 1. Brainstorming

### 1.1 Posisi Fase 3 dalam Siklus Hidup Produk

Fase 1 (Core Server, Transport, Auth, RBAC) dan Fase 2 (Device Management, Inventory, Groups) telah menyediakan API backend yang kuat dan teruji secara live E2E dengan data mesin nyata. Namun, sampai saat ini seluruh interaksi dengan sistem hanya dapat dilakukan melalui REST API dan PowerShell test scripts. 

Fase 3 adalah jembatan krusial:
1. Menyediakan **Dashboard backend API** (`/server/modules/dashboard`) untuk agregasi metrik armada secara real-time (bukan sekadar daftar raw).
2. Menyediakan **Web Console production-grade** (`/web-console`) berbasis React + TypeScript + Vite.
3. Mengintegrasikan web console ke dalam **single binary** `emserver.exe` via Go `embed.FS`, sehingga server dapat langsung menyajikan Web Console di browser tanpa memerlukan Node.js runtime di server produksi.

### 1.2 Ruang Lingkup Fase 3

Yang dibangun di fase ini (nyata, bukan mock/dummy):
1. **Backend Dashboard Module** (`/server/modules/dashboard`):
   - `GET /api/dashboard/summary`: Agregasi ringkasan fleet (total devices, online, offline, retired, persentase online, disk health alerts).
   - `GET /api/dashboard/sites`: Breakdown per cabang/site (total, online, offline, ratio) untuk monitoring multi-cabang.
   - `GET /api/dashboard/os`: Distribusi sistem operasi (Windows, Linux, macOS) beserta versi terbanyak.
   - `GET /api/dashboard/alerts`: Deteksi proaktif masalah armada (disk space rendah < 15%, device offline berkepanjangan > 24 jam, hardware drift baru).
   - `GET /api/dashboard/activity`: Aliran aktivitas audit terbaru (enrollment, command, login, retire).
2. **Web Console UI** (`/web-console`):
   - **Autentikasi & Sesi**: Login page dengan JWT storage aman, auto-refresh token saat expired, role badge (Admin / Technician / Viewer).
   - **Executive Dashboard**: KPI Cards interaktif, bar breakdown per-cabang, distribusi OS, warning list armada.
   - **Device Explorer**: Tabel perangkat dengan pagination SQL server-side, filtering site/status, modal detail hardware/software/OS lengkap dengan data dari CIM/registry nyata.
   - **Audit Log Viewer**: Tampilan audit log interaktif untuk kepatuhan compliance dan pelacakan teknisi.
   - **Real-time Refresh**: Auto-poll status setiap 10-30 detik dengan indikator koneksi live.
3. **Single Binary Deployment**:
   - `web-console` dibuild ke aset statis (`dist/`).
   - Server Go meng-embed aset ini via `embed.FS` dan melayani SPA routing (fallback ke `index.html` untuk non-API routes).

Yang **TIDAK** di fase ini:
- Patch Compliance % dilaporkan sebagai `null` / "Module not enabled", karena Patch Management dijadwalkan pada Fase 5. Tidak ada angka fiktif.
- Software Deployment tracking dijadwalkan pada Fase 4.

### 1.3 Opsi Arsitektur & Trade-off

| Keputusan | Pilihan | Alternatif | Alasan & Trade-off |
|---|---|---|---|
| **Distribusi Web UI** | **Single Binary via Go `embed.FS`** | Microservices terpisah (Node backend + Go API) | Di enterprise on-premise, deployment 1 binary `.exe` jauh lebih stabil, tidak perlu mengelola Node process manager (PM2/systemd) di server pusat. Pengembang tetap bisa menjalankan Vite dev server saat coding. |
| **Frontend Framework** | **React + Vite + TypeScript** | Server-rendered HTML (Go html/template); Vue/Svelte | React + TS memberikan ekosistem komponen kaya, strong typing yang cocok dengan DTO Go, dan performa SPA cepat untuk dashboard operasional 500+ device. |
| **Styling & Design System** | **Tailwind CSS / Pure Enterprise CSS Modern** | Berat UI libraries (Material UI, AntD) | Kecepatan build cepat, bundle size kecil (<300KB gz), desain bersih ala Datadog/ManageEngine dengan support Dark/Light mode dan zero-dependency bloat. |
| **Data Fetching & State** | **Custom Hook + Native Fetch + Auto-Polling** | Redux / TanStack Query | Menjaga dependency minimal, overhead memori rendah, dan logika retry/refresh token terpusat dan transparan. |
| **Agregasi Metrik** | **Direct Indexed SQL Queries** | Pre-computed cron / Redis cache | Pada skala 500-2000 endpoint, SQLite dengan indeks `idx_devices_status`, `idx_devices_site`, dan `idx_inventory_disk` mengeksekusi kueri agregasi dalam waktu < 2ms. Menggunakan Redis atau cron agregasi menambahkan kompleksitas tanpa justifikasi performa pada skala ini. |

---

## 2. Writing Plan

### 2.1 Struktur Folder & File yang Dibuat

```
server/
  modules/
    dashboard/
      model.go               # Structs: Summary, SiteMetrics, OSMetrics, Alert, Activity
      repository.go          # SQL queries terindeks untuk agregasi
      handler.go             # HTTP handlers untuk /api/dashboard/*
      dashboard_test.go      # Unit & integration tests dashboard
  cmd/
    server/
      web_embed.go           # Go embed.FS wrapper untuk /web-console/dist
      main.go                # Mount dashboard handler & web console SPA handler
web-console/
  package.json
  tsconfig.json
  vite.config.ts
  index.html
  src/
    main.tsx
    App.tsx
    types/api.ts             # TypeScript interfaces mencerminkan DTO Go
    services/api.ts          # API client dengan JWT auto-refresh & error handling
    context/AuthContext.tsx  # Global auth state (user, token, role, logout)
    components/
      Navbar.tsx             # Header navigasi, user role, status server
      KPICard.tsx            # Stat card modular
      SiteDistribution.tsx   # Visual breakdown cabang
      OSDistribution.tsx     # Visual breakdown OS
      AlertsList.tsx         # Health & security alerts
      DeviceDetailModal.tsx  # Modal detail inventory lengkap (HW/SW/OS)
    pages/
      LoginPage.tsx          # Form login dengan feedback validasi
      DashboardPage.tsx      # Tampilan utama monitoring eksekutif
      DevicesPage.tsx        # Manajemen armada dengan filter & pagination server
      AuditPage.tsx          # Riwayat audit lengkap
tests/
  integration/
    dashboard_test.go        # E2E API integration test untuk seluruh dashboard endpoint
scripts/
  e2e-dashboard.ps1          # Live E2E script memvalidasi API + UI rendering
```

### 2.2 API Contract — Dashboard Module

Semua endpoint dilindungi oleh `jwtSvc.RequireAuth` dan role `viewer` (Viewer, Technician, Admin berhak melihat dashboard).

#### 1. `GET /api/dashboard/summary`
```json
{
  "total_devices": 12,
  "online_devices": 9,
  "offline_devices": 3,
  "retired_devices": 1,
  "online_pct": 75.0,
  "low_disk_alerts": 1,
  "recent_hw_changes_24h": 0,
  "sites_count": 3
}
```

#### 2. `GET /api/dashboard/sites`
```json
[
  { "site": "hq", "total": 8, "online": 7, "offline": 1, "online_pct": 87.5 },
  { "site": "cabang-surabaya", "total": 3, "online": 2, "offline": 1, "online_pct": 66.7 },
  { "site": "cabang-medan", "total": 1, "online": 0, "offline": 1, "online_pct": 0.0 }
]
```

#### 3. `GET /api/dashboard/os`
```json
[
  { "os_name": "windows", "count": 9, "pct": 75.0 },
  { "os_name": "linux", "count": 2, "pct": 16.7 },
  { "os_name": "macos", "count": 1, "pct": 8.3 }
]
```

#### 4. `GET /api/dashboard/alerts`
```json
[
  {
    "type": "low_disk",
    "severity": "warning",
    "device_id": "dev-123",
    "hostname": "PC-FINANCE-01",
    "site": "hq",
    "message": "Disk free space is below 15% (11.2% free on C:)",
    "timestamp": "2026-09-23T10:15:00Z"
  }
]
```

---

## 3. Urutan Eksekusi

1. **Implementasi Backend Dashboard**:
   - `server/modules/dashboard/model.go`
   - `server/modules/dashboard/repository.go`
   - `server/modules/dashboard/handler.go`
   - Registrasi di `server/cmd/server/main.go`
2. **Testing Backend**:
   - Unit tests & integration tests untuk dashboard module (`go test ./...`)
3. **Implementasi Frontend Web Console**:
   - Setup project Vite React TS di `/web-console`
   - API client, auth context, layout, pages (Login, Dashboard, Devices, Audit)
   - Build frontend production (`npm run build` menghasilkan `dist/`)
4. **Integrasi Single Binary (SPA Fallback)**:
   - Buat `server/cmd/server/web_embed.go` menggunakan `embed.FS`
   - Tambahkan SPA handler di `main.go` untuk menyajikan `index.html` dan asset statis
5. **E2E Testing Nyata**:
   - Script live E2E (`scripts/e2e-dashboard.ps1`) menguji API backend dan verifikasi respons HTML/JS dari server port aktif
6. **Audit & Readiness Report**:
   - Verifikasi bukti test dan perbarui Production Readiness Scorecard
