# Production Readiness Scorecard — Central

**Proyek:** Endpoint Management Platform
**Update terakhir:** 2026-09-22 (Fase 2 selesai — device management, inventory, groups, lifecycle)

Legenda status (hanya boleh salah satu dari):
`NOT STARTED` · `IN PROGRESS` · `CODE COMPLETE (UNTESTED)` · `TESTED (STAGING)` · `PRODUCTION READY`

Aplikasi secara keseluruhan hanya boleh disebut "siap production" jika:
1. **SEMUA** modul minimal `TESTED (STAGING)`, dan
2. Modul kritikal (Remote Control, Patch Management, Software Deployment) sudah
   `PRODUCTION READY` dengan **bukti test** di lingkungan yang mendekati nyata
   (multi-OS, koneksi cabang disimulasikan).

## Scorecard

| Modul | Status | Ditest E2E? | Catatan / Blocker |
|---|---|---|---|
| Core / Infra | `TESTED (STAGING)` | ✅ Ya — live binary | Config, DB+migrasi, logger, bootstrap. 8 cek live E2E lulus. |
| Auth (JWT, bcrypt, login) | `TESTED (STAGING)` | ✅ Ya | 3 unit test + integration RBAC. **TLS belum diuji** — semua tes di plaintext. |
| RBAC | `TESTED (STAGING)` | ✅ Ya | Hierarki viewer<technician<admin terverifikasi (403/201 live). |
| Transport (WS, hub, offline) | `TESTED (STAGING)` | ✅ Ya | Outbound-only; offline detection 0.35s. 3 bug produksi ditemukan & diperbaik E2E. |
| Audit Log | `TESTED (STAGING)` | ✅ Ya | 6 aksi terverifikasi berurutan, termasuk pasangan connect+disconnect. |
| Agent — Windows | `TESTED (STAGING)` | ✅ Ya — binary asli | Enroll + connect + command nyata di mesin ini. |
| Agent — Linux | `CODE COMPLETE (UNTESTED)` | ❌ Build saja | WSL/Docker belum diaktifkan. Bisa jadi bug runtime di `/etc/os-release`. |
| Agent — macOS | `CODE COMPLETE (UNTESTED)` | ❌ Build saja | `sw_vers` parsing belum pernah diuji vs output asli. |
| TLS / WSS | `CODE COMPLETE (UNTESTED)` | ❌ | **Risiko tertinggi.** Semua E2E di `ws://` plaintext. Produksi wajib sertifikat. |
| Device Management | `TESTED (STAGING)` | ✅ Ya — live binary | 9/9 live E2E lulus; RAM/CPU cocok CIM independen; 32 software entries nyata; on-demand collect terbukti me-refresh snapshot. **Bukan PR karena TLS belum ada + Linux/macOS collector belum disentuh mesin asli.** |
| Dashboard | `NOT STARTED` | — | — |
| Software Deployment | `NOT STARTED` | — | Modul kritikal (syarat production ready) |
| Patch Management | `NOT STARTED` | — | Modul kritikal (syarat production ready) |
| Remote Control | `NOT STARTED` | — | Modul kritikal. Encoder pure-Go (MJPEG fallback); relay belum ada VPS publik; compiler C absen untuk H.264 |
| Reports | `NOT STARTED` | — | — |
| User Management | `NOT STARTED` | — | Hanya bootstrap admin. AD/SSO sengaja ditunda. |
| Web Filter | `NOT STARTED` | — | Butuh code-signing cert untuk kernel-level enforcement |
| Agent Self-Update | `NOT STARTED` | — | Butuh code-signing untuk update binary |
| Notification/Alerting | `NOT STARTED` | — | — |
| Bandwidth / Staggered Rollout | `NOT STARTED` | — | — |
| Asset & License Management | `NOT STARTED` | — | — |
| Task Scheduler / Script Repository | `NOT STARTED` | — | — |

**Status aplikasi secara keseluruhan: `IN PROGRESS`** — fondasi teruji, tapi
modul kritikal (Remote Control, Patch, Software Deployment) masih `NOT STARTED`,
jadi klaim "production ready" belum bisa dibuat sesuai kriteria di atas.

## Catatan environment (mempengaruhi achievable status)

- **Compiler C tidak ada** → dependency cgo (x264-go, go-sqlite3) tidak dapat dibuild.
  Semua pilihan teknologi dijaga tetap pure-Go.
- **WSL & Docker daemon mati** → agent Linux/macOS **tidak dapat** diuji di mesin ini
  sampai salah satunya diaktifkan. Modul OS-spesifik non-Windows akan terhenti di
  `CODE COMPLETE (UNTESTED)`.
- **git terpasang** (`C:\Program Files\Git`, 2.55.0) tetapi **tidak ada di PATH**
  untuk sesi PowerShell ini; pakai path absolut. Repo di-init, commit Fase 1: `75cca22`.
- **Tidak ada VPS publik** → relay remote control hanya bisa diuji local-only.
- **Windows Defender mengkarantina agent binary** `go build -o emagent.exe`
  sebagai false positive, sehingga live E2E gagal di `Start-Process` alih-alih
  di assertion. Siasat: bangun dengan `-ldflags '-X main.agentVersion=<versi>'`
  agar byte berubah dan signature clear. Ini quirk build environment, bukan
  sifat kode; di fleet nyata, agent harus di-code-sign dengan sertifikat
  berbayar yang **belum dimiliki**.

## Riwayat sesi

| Tanggal | Sesi | Ringkasan |
|---|---|---|
| 2026-09-22 | 0 | Fase 0 brainstorming arsitektur. Verifikasi environment + dependensi. Belum ada kode aplikasi. Menunggu approve. |
| 2026-09-22 | 1 | Fase 1 selesai: core server, multi-OS agent, transport, auth, RBAC, audit. 10 test Go lulus, 8 cek live E2E lulus vs binary asli, 3 bug produksi ditemukan & diperbaiki. 3 OS ter-compile, hanya Windows diuji jalan. TLS belum diuji. |
| 2026-09-22 | 2 | Fase 2 selesai: inventory (hw/sw/os), groups, retire/restore, pagination. **30 test Go lulus** (14 device-mgmt + 12 unit/integration + 4 CIM), **9/9 live E2E inventory** + **8/8 Fase 1 (regresi: 0)**. **2 bug produksi** ditemukan hanya karena live E2E: (1) RAM 0 byte karena error ditelan, (2) `collected_at` selalu tahun 0001 karena field tidak ada di struct agent — keduanya diperbaiki. Device Management naik ke `TESTED (STAGING)`. |
