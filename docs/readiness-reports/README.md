# Production Readiness Scorecard — Central

**Proyek:** Endpoint Management Platform
**Update terakhir:** 2026-09-22 (Fase 0 — awal proyek)

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
| Core/Auth | NOT STARTED | — | — |
| Device Management | NOT STARTED | — | — |
| Dashboard | NOT STARTED | — | — |
| Software Deployment | NOT STARTED | — | — |
| Patch Management | NOT STARTED | — | — |
| Remote Control | NOT STARTED | — | Encoder pure-Go (MJPEG fallback); relay belum ada VPS publik; compiler C absen untuk H.264 |
| Reports | NOT STARTED | — | — |
| User Management | NOT STARTED | — | AD/SSO sengaja ditunda ke fase lanjut |
| Web Filter | NOT STARTED | — | Butuh code-signing cert untuk kernel-level enforcement |
| Agent Self-Update | NOT STARTED | — | Butuh code-signing untuk update binary |
| Notification/Alerting | NOT STARTED | — | — |
| Bandwidth / Staggered Rollout | NOT STARTED | — | — |
| Asset & License Management | NOT STARTED | — | — |
| Task Scheduler / Script Repository | NOT STARTED | — | — |

## Catatan environment (mempengaruhi achievable status)

- **Compiler C tidak ada** → dependency cgo (x264-go, go-sqlite3) tidak dapat dibuild.
  Semua pilihan teknologi dijaga tetap pure-Go.
- **WSL & Docker daemon mati** → agent Linux/macOS **tidak dapat** diuji di mesin ini
  sampai salah satunya diaktifkan. Modul OS-spesifik non-Windows akan terhenti di
  `CODE COMPLETE (UNTESTED)`.
- **git belum terinstal** → tidak ada version control sampai dipasang.
- **Tidak ada VPS publik** → relay remote control hanya bisa diuji local-only.

## Riwayat sesi

| Tanggal | Sesi | Ringkasan |
|---|---|---|
| 2026-09-22 | 0 | Fase 0 brainstorming arsitektur. Verifikasi environment + dependensi. Belum ada kode aplikasi. Menunggu approve. |
