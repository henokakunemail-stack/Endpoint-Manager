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
| `go test ./...` | **33 PASS** (30 fungsi test + 3 subtest), 0 gagal, 0 skip |
| `go vet ./...` | **Bersih** — 0 warning |
| Build server (`./server/cmd/server`) | OK |
| Build agent Windows | OK |
| Cross-compile agent **Linux** | ❌ **GAGAL KOMPILASI** (lihat catatan Agent — Linux) |
| Cross-compile agent **macOS** (amd64) | OK |
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
| Auth (JWT, bcrypt, login) | `TESTED (STAGING)` | ✅ Ya | Login + token replay ditolak (401) terverifikasi live. **TLS belum diuji** — semua tes plaintext. Password bootstrap default `admin12345` hardcode di `main.go:162`. |
| RBAC | `TESTED (STAGING)` | ✅ Ya | Hierarki viewer<technician<admin terverifikasi (403/201 live). |
| Transport (WS, hub, offline) | `TESTED (STAGING)` | ✅ Ya | Outbound-only; offline detection cepat; command queue survive disconnect. Hub **in-memory** → single-node only, belum bisa horizontal scale. |
| Audit Log | `TESTED (STAGING)` | ✅ Ya | 6 aksi terverifikasi berurutan termasuk pasangan connect+disconnect. |
| Agent — Windows | `TESTED (STAGING)` | ✅ Ya — binary asli | Enroll + connect + command nyata. RAM 16GB & CPU i5-1135G7 **cocok dengan query CIM independen**. |
| Agent — Linux | ❌ **BROKEN (compile error)** | ❌ Tidak | **Koreksi klaim lama.** Bukan "build saja" — **tidak kompilasi sama sekali**: `agent/linux/inventory_linux.go:174` `var disks []Disk` (seharusnya `inventory.Disk`). Plus `utsString([65]int8)` akan break di ARM64 (`uint8`), dan parser dpkg abaikan field `Status:` → software yang sudah uninstall tetap dilaporkan. |
| Agent — macOS | `CODE COMPLETE (UNTESTED)` | ❌ Build saja | Kompilasi OK. Bug runtime belum teruji: parsing output `mount` dengan `SplitN(line," ",4)` memotong volume bernama spasi (`/Volumes/Macintosh HD` → `/Volumes/Macintosh`). |
| TLS / WSS | `NOT STARTED` | ❌ | **Risiko tertinggi, dan kodenya memang belum ada** — bukan sekadar "belum diuji". Server `ListenAndServe()` plain; config punya zero field TLS; credentials/device secret di header plaintext. **Blokir deployment ke cabang.** |
| Device Management | `TESTED (STAGING)` | ✅ Ya — live binary | 12/12 live E2E lulus hari ini: 32 software entries nyata, serial Dell Latitude 3420 terekam, on-demand collect terbukti me-refresh snapshot, group + retire/restore jalan. Pagination masih in-memory slice (bukan SQL LIMIT) — OK di skala saat ini, perlu diubah sebelum puluhan ribu device. |
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
 tapi 11 modul masih `NOT STARTED` dan 3 modul kritikal (Remote Control, Patch,
 Software Deployment) belum dimulai. Klaim "production ready" belum bisa dibuat
 sesuai kriteria di atas. Selain itu **TLS belum ada kodenya sama sekali**, dan
 **agent Linux saat ini rusak** — dua hal yang harus diperbaiki sebelum lulus
 berikutnya.

---

## Yang menggantung / belum selesai (daftar eksplisit)

**A. Hutang teknis nyata (bisa dikerjakan sekarang, tanpa biaya):**

1. **Agent Linux tidak kompilasi** — `inventory_linux.go:174`. Ini regressi yang
   membuat klaim "multi-OS" platform belum benar. *Quick fix, high credibility gain.*
2. **`utsString([65]int8)`** (`inventory_linux.go:275`) → pecah build di ARM64 Linux.
3. **Parser dpkg** (`inventory_linux.go:298`) abaikan field `Status:` → melaporkan
   software yang sudah di-`deinstall`/`purge`.
4. **Parser `mount` macOS** (`inventory_darwin.go:153`) → volume dengan spasi
   dipotong; `Statfs` gagal diam-diam dan disk hilang dari laporan.
5. **Password bootstrap `admin12345`** hardcode — wajib dipaksa ganti saat first
   login sebelum production.
6. **Pagination in-memory** (`handler.go:90`) → pindahkan LIMIT/OFFSET ke SQL
   sebelum skala besar.
7. **`README.md` baris 7 & 52 sudah usang** — masih bilang "Fase 0, menunggu
   approve" dan "git belum terinstal" (git ada, Fase 2 selesai).

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
| **TLS** | Kode belum ditulis sama sekali. Bisa dikerjakan sekarang (crypto/tls pure-Go, atau reverse proxy nginx/Caddy). |
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
| 2026-09-23 | 3 | **Verifikasi ulang menyeluruh.** 33 test + `go vet` bersih. **Live E2E dijalankan ulang: 12/12 (Fase 2) + 8/8 (Fase 1 regresi), semua dengan data nyata mesin ini** (RAM 16GB, i5-1135G7, Dell Latitude 3420, 32 software). **Temuan koreksi:** (1) **agent Linux tidak pernah kompilasi** — `inventory_linux.go:174` `undefined: Disk`; klaim lama "Build saja" salah. (2) **TLS bukan "belum diuji" tapi kodenya belum ada sama sekali** — diturunkan ke `NOT STARTED`. (3) Bug runtime teridentifikasi di macOS mount parser, utsString ARM64, dpkg Status. Status keseluruhan tetap `IN PROGRESS`; jangkar credibilitas dipertahankan dengan hanya mengklaim yang terbukti. |
