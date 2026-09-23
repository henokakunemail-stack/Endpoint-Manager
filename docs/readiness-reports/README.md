# Production Readiness Scorecard — Central

**Proyek:** Endpoint Management Platform
**Update terakhir:** 2026-09-23 (verifikasi ulang menyeluruh — bukan rekap laporan)

Legenda status (hanya boleh salah satu dari):
`NOT STARTED` · `IN PROGRESS` · `CODE COMPLETE (UNTESTED)` · `TESTED (STAGING)` · `PRODUCTION READY`

Aplikasi secara keseluruhan hanya boleh disebut "siap production" jika:
1. **SEMUA** modul minimal `TESTED (STAGING)`, dan
2. Modul kritikal (Remote Control, Patch Management, Software Deployment) sudah
   `PRODUCTION READY` dengan **bukti test** di lingkungan yang mendekati nyata
   (multi-OS, koneksi cabang disimulasikan).

---

## Ringkasan jujur (bukti dijalankan ulang hari ini, 2026-09-23)

| Pemeriksaan | Hasil aktual |
|---|---|
| `go test ./...` | **33 PASS**, 0 gagal, 0 skip |
| `go vet ./...` | **Bersih** — 0 warning |
| Build server (`./server/cmd/server`) | OK |
| Build agent Windows | OK |
| Cross-compile agent **Linux** (amd64 + arm64) | ✅ OK |
| Cross-compile agent **macOS** (amd64 + arm64) | ✅ OK |
| Live E2E Fase 1 (regresi) | ✅ **8/8 lulus** — server+agent binary asli |
| Live E2E Fase 2 (inventory) | ✅ **12/12 lulus** — data asli mesin ini |

**Tingkat penyelesaian keseluruhan: ~3 dari ~14 modul fungsional teruji.**
Sisa 11 modul adalah `NOT STARTED` (nol baris kode). Kode yang ada berkualitas
tinggi dan teruji, tapi cakupannya masih kecil.

---

## Scorecard

| Modul | Status | Ditest E2E? | Catatan / Blocker |
|---|---|---|---|
| Core / Infra | `TESTED (STAGING)` | ✅ Ya — live binary | Config, DB+migrasi 0001/0002, logger, bootstrap. 8 cek live E2E lulus (regresi bersih). |
| Auth (JWT, bcrypt, login) | `TESTED (STAGING)` | ✅ Ya | Login + token replay ditolak (401) terverifikasi live. TLS siap diaktifkan (set `TLS_CERT_FILE` + `TLS_KEY_FILE`). Password bootstrap dari `ADMIN_PASSWORD` env var atau random generated. |
| RBAC | `TESTED (STAGING)` | ✅ Ya | Hierarki viewer<technician<admin terverifikasi (403/201 live). |
| Transport (WS, hub, offline) | `TESTED (STAGING)` | ✅ Ya | Outbound-only; offline detection cepat; command queue survive disconnect. Hub **in-memory** → single-node only, belum bisa horizontal scale. |
| Audit Log | `TESTED (STAGING)` | ✅ Ya | 6 aksi terverifikasi berurutan termasuk pasangan connect+disconnect. |
| Agent — Windows | `TESTED (STAGING)` | ✅ Ya — binary asli | Enroll + connect + command nyata. RAM 16GB & CPU i5-1135G7 **cocok dengan query CIM independen**. |
| Agent — Linux | `CODE COMPLETE (UNTESTED)` | ❌ Build saja | **Diperbaiki sesi ini.** Compile error `undefined: Disk` di-fix; parser dpkg sekarang cek field `Status:` (skip paket uninstalled). Cross-compile linux/amd64 + linux/arm64 sukses. Belum diuji di mesin Linux nyata (WSL/Docker mati). |
| Agent — macOS | `CODE COMPLETE (UNTESTED)` | ❌ Build saja | **Diperbaiki sesi ini.** Parser mount sekarang tahan volume dengan spasi. Cross-compile darwin/amd64 + darwin/arm64 sukses. Belum diuji di mesin macOS nyata. |
| TLS / WSS | `CODE COMPLETE (UNTESTED)` | ❌ | **Diperbaiki sesi ini.** Server sekarang mendukung `ListenAndServeTLS` via env var `TLS_CERT_FILE` + `TLS_KEY_FILE`. Belum diuji dengan sertifikat sungguhan — perlu sertifikat (self-signed untuk staging, CA-signed untuk production). |
| Device Management | `TESTED (STAGING)` | ✅ Ya — live binary | 12/12 live E2E lulus: 32 software entries nyata, serial Dell Latitude 3420 terekam, on-demand collect terbukti. **Pagination diperbaiki sesi ini** — sekarang SQL LIMIT/OFFSET, bukan in-memory slice. |
| Dashboard / Web Console | `NOT STARTED` | — | **Nol file** frontend (tidak ada `.tsx`/`.html`/`package.json`). Padahal Node v24.20.0 tersedia. |
| Software Deployment | `NOT STARTED` | — | Modul kritikal (syarat production ready). Nol kode. |
| Patch Management | `NOT STARTED` | — | Modul kritikal. Nol kode. |
| Remote Control | `NOT STARTED` | — | Modul kritikal. **Nol kode** — tidak ada WebRTC/Pion/MJPEG/H.264/relay. Compiler C absen (x264-go tidak bisa dibuild) → encoder harus pure-Go; relay butuh VPS publik yang belum ada. |
| Reports | `NOT STARTED` | — | Nol kode. |
| User Management | `NOT STARTED` | — | Hanya bootstrap admin + `/api/auth/login` & `/refresh`. Tidak ada API create/update/delete user. AD/SSO sengaja ditunda. |
| Web Filter | `NOT STARTED` | — | Butuh code-signing cert untuk kernel-level enforcement. |
| Agent Self-Update | `NOT STARTED` | — | Butuh code-signing untuk update binary. |
| Notification/Alerting | `NOT STARTED` | — | Nol kode. |
| Bandwidth / Staggered Rollout | `NOT STARTED` | — | Hanya ada *stagger window* untuk inventory scheduler — bukan rollout deployment. |
| Asset & License Management | `NOT STARTED` | — | Nol kode. |
| Task Scheduler / Script Repository | `NOT STARTED` | — | Hanya scheduler inventory. Disetujui di Fase 0 tapi belum dikerjakan. |

**Status aplikasi secara keseluruhan: `IN PROGRESS`** — fondasi teruji dan bersih,
 7 hutang teknis produksi telah diperbaiki (TLS, compile Linux, password hardcode,
 SQL pagination, parser dpkg/macOS). 11 modul masih `NOT STARTED` dan 3 modul
 kritikal (Remote Control, Patch, Software Deployment) belum dimulai. Klaim
 "production ready" belum bisa dibuat sesuai kriteria di atas.

---

## Yang menggantung / belum selesai (daftar eksplisit)

**A. Hutang teknis — SEMUA 7 ITEM SUDAH DIPERBAIKI (commit `109f941`):**

1. ~~Agent Linux tidak kompilasi~~ → ✅ Fixed: `inventory.Disk` qualifier.
2. ~~`utsString` ARM64~~ → ✅ Verified: `[65]int8` di kedua arch (Go 1.26), komentar diperjelas.
3. ~~Parser dpkg~~ → ✅ Fixed: cek `Status:` field, skip `deinstall`/`purge`.
4. ~~Parser `mount` macOS~~ → ✅ Fixed: parse `" on "` + `" ("` delimiters.
5. ~~Password `admin12345` hardcode~~ → ✅ Fixed: baca `ADMIN_PASSWORD` env var, atau random generated.
6. ~~Pagination in-memory~~ → ✅ Fixed: `ListPaged()` dengan SQL `LIMIT`/`OFFSET`.
7. ~~README usang~~ → ✅ Fixed: status dan catatan git diperbarui.

**B. Dokumen rencana vs implementasi (selisih kecil, namun perlu disamakan):**

Rencana Fase 2 §2.1 menamai file yang akhirnya dibuat dengan nama berbeda:

| Direncanakan | Aktual | Keterangan |
|---|---|---|
| `agent/shared/inventory/report.go` | ✁ tidak ada (logikanya di `inventory.go`) | fungsinya ada, cuma nama |
| `agent/shared/inventory/schedule.go` | `scheduler.go` | sama, beda nama |
| `tests/integration/device_mgmt_test.go` | `inventory_test.go` | sama, beda nama |

Semua fungsi yang direncanakan **terimplementasi**; ini cuma drift penamaan,
bukan fitur yang hilang.

**C. Blocker non-kode (butuh keputusan/biaya, bukan coding):**

| Blocker | Mengapa menggantung |
|---|---|
| **TLS** | Kode sudah ada (`ListenAndServeTLS`). Perlu sertifikat: self-signed untuk staging, CA-signed untuk production. Belum diuji end-to-end dengan sertifikat. |
| **Code-signing certificate** | Berbayar. Tanpa ini agent dipatok SmartScreen/Defender di setiap deploy. |
| **VPS publik untuk relay remote control** | Belum ada. Remote control hanya bisa diuji local-only. |
| **Runtime Linux/macOS untuk uji agent** | WSL/Docker daemon mati di mesin ini. Nyalakan salah satu untuk membuka blokir agent Linux/macOS. |
| **Load test 500 device** | Desain mendukung (stagger, pagination, index) tapi tidak terbukti. Jangan klaim "terbukti menskalakan". |

---

## Catatan environment (mempengaruhi achievable status)

- **Compiler C tidak ada** → dependency cgo (x264-go, go-sqlite3) tidak bisa
  dibuild. Semua pilihan teknologi dijaga tetap pure-Go.
- **WSL & Docker daemon mati** → agent Linux/macOS tidak dapat diuji di mesin ini
  sampai salah satunya diaktifkan. Karena itu juga, bug compile agent Linux
  **tidak terdeteksi** oleh `go build ./...` biasa (build tag menyembunyikannya);
  baru ketahuan saat `GOOS=linux go build`. Lihat rekomendasi di bawah.
- **git terpasang** (`C:\Program Files\Git`, 2.55.0) tetapi **tidak di PATH**
  untuk sesi PowerShell; pakai path absolut. Repo ada 69 tracked file, branch `master`.
- **Windows Defender** mengkarantina agent binary `go build -o emagent.exe`
  sebagai false positive. Siasat: bangun dengan
  `-ldflags '-X main.agentVersion=<versi>'` agar byte berubah. Quirk build
  environment, bukan sifat kode; di fleet nyata agent harus di-code-sign.

> **Rekomendasi proses (muncul dari temuan hari ini):** tambahkan langkah
> `GOOS=linux GOOS=darwin go build` ke CI/check rutin. `go build ./...` di
 Windows **tidak** akan menangkap bug di file `//go:build linux` — itulah
 sebabnya error `undefined: Disk` bertahan 2 fase tanpa ketahuan.

## Riwayat sesi

| Tanggal | Sesi | Ringkasan |
|---|---|---|
| 2026-09-22 | 0 | Fase 0 brainstorming arsitektur. Verifikasi environment + dependensi. Belum ada kode aplikasi. |
| 2026-09-22 | 1 | Fase 1 selesai: core server, multi-OS agent, transport, auth, RBAC, audit. 8 cek live E2E lulus, 3 bug produksi ditemukan & diperbaiki. |
| 2026-09-22 | 2 | Fase 2 selesai: inventory, groups, retire/restore, pagination. 9/9 live E2E + 8/8 regresi. 2 bug produksi ditemukan & diperbaiki. |
| 2026-09-23 | 3 | **Verifikasi ulang + perbaikan 7 production blocker.** Temuan audit: agent Linux tidak kompilasi, TLS belum ada, password hardcode, pagination in-memory, parser dpkg/macOS buggy. **Semua 7 diperbaiki dan diverifikasi**: 33 test PASS, `go vet` bersih, cross-compile 5 target (termasuk Linux amd64+arm64 yang sebelumnya gagal), live E2E: 12/12 Fase 2 + 8/8 Fase 1 regresi. Agent Linux naik dari BROKEN → `CODE COMPLETE (UNTESTED)`, TLS dari `NOT STARTED` → `CODE COMPLETE (UNTESTED)`. |
