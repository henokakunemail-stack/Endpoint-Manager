# Fase 0 — Brainstorming Arsitektur Keseluruhan

**Status:** AWAITING APPROVAL — belum ada kode aplikasi yang ditulis.
**Tanggal:** 2026-09-22

Dokumen ini adalah output Fase 0 (Brainstorming arsitektur keseluruhan). Berdasarkan
instruksi proyek, **tidak ada kode aplikasi yang ditulis sampai dokumen ini di-approve.**

---

## 1. Verifikasi Environment (faktual, bukan asumsi)

Semua item di bawah dicek langsung di mesin pengembangan sebelum rekomendasi dibuat.

### 1.1 Yang tersedia

| Item | Versi / Nilai | Dampak |
|---|---|---|
| Go | 1.26.8 windows/amd64 | Backend + agent bisa langsung dimulai |
| Node.js | v24.20.0 (npm 11.19.0) | Web console bisa langsung dimulai |
| Network egress | proxy.golang.org:443 & registry.npmjs.org:443 reachable; tanpa HTTP proxy env | Boleh ambil dependency publik |
| Disk | D: 553.0 GB free (C: 187.7 GB free) | Cukup untuk repo, module cache, artifacts |
| GOPATH | platform default | Module cache standar |
| Docker CLI | terinstal (Docker Desktop) | Binary ada, tapi daemon mati (lihat 1.2) |

### 1.2 Yang TIDAK tersedia — membatasi pilihan teknologi

| Item | Status | Konsekuensi jujur |
|---|---|---|
| **git** | tidak terinstal | Tidak ada version control. **Blokir best practice.** Bisa di-install ringan; sampai itu, perubahan tidak punya history. |
| **Compiler C** (`gcc`, `cl`, `msvc`, `mingw`, `cargo`) | tidak ada | Setiap dependency yang butuh cgo **tidak bisa dibuild**. Ini mengecualikan x264-go, go-sqlite3. |
| **WSL** | tidak terinstal | **Agent Linux tidak bisa diuji di mesin ini.** |
| **Hyper-V** | nonaktif | Tidak bisa VM Windows tambahan. |
| **Docker daemon** | terinstal tapi **mati** | Postgres/Redis tidak bisa jalan sebagai kontainer saat ini. |
| **PostgreSQL / Redis server** | tidak ada | DB & queue harus embedded / pure-Go. |
| **openssl CLI, ffmpeg** | tidak ada | TLS lewat library Go (crypto/tls), bukan CLI; encode lewat library. |

### 1.3 Kapasitas uji yang realistis

Mesin ini: Dell Latitude 3420, i5-1135G7 (4 core / 8 thread), RAM 15.7 GB.

**Saya akan menguji apa yang bisa diuji di sini secara nyata**, yaitu: server + agent Windows
berjalan di mesin ini, koneksi WebSocket lokal, DB SQLite, automated test suite. Yang **tidak**
bisa saya uji di sini (NAT lintas-cabang sungguhan, multi-OS nyata, beban 500 device) akan
**ditandai sebagai blocker di Scorecard**, bukan dihapusbukan.

---

## 2. Pilihan Tech Stack

### Rekomendasi

| Lapisan | Pilihan | Alasan |
|---|---|---|
| Server backend | **Go** (net/http + `go-chi/chi/v5`) | 1 binary deployment; cross-compile native ke 3 OS; goroutine cocok untuk ribuan koneksi agent persisten; stdlib sudah cukup. |
| Agent | **Go** (repo yang sama, build tag per OS) | Satu codebase untuk 3 OS — divergensi hanya di lapisan OS-spesifik (patch detection, installer, capture, input). |
| DB | **SQLite** (`modernc.org/sqlite` — pure-Go) | Embedded, no daemon, no cgo. Migrasi ke PostgreSQL nanti via adapter layer; schema tetap portabel. |
| Job queue | **Embedded queue di server** (tabel DB + worker goroutine) | Tidak butuh Redis; cukup untuk skala 500. Tingkatkan ke Redis/NATS nanti jika perlu. |
| Agent ↔ Server transport | **WebSocket over TLS (WSS)**, agent-initiated outbound | Memenuhi konstrain: server tidak pernah membuka koneksi ke IP device cabang. |
| Remote control | **Pion WebRTC** (SFU/relay) + capture + **MJPEG pure-Go** | Real-time, NAT traversal via relay, no cgo. |
| Web console | **React + Vite + TypeScript** | Dashboard real-time; tooling ada (Node 24). |
| Testing | Go testing stdlib + HTTP/WebSocket integration test | Bisa dijalankan langsung di mesin ini tanpa infra tambahan. |

### 2.1 Kenapa Go untuk agent multi-OS (poin krusial dari spec)

Spec menegaskan tiap OS punya cara berbeda untuk patch, install software, elevasi, dan
screen capture. Struktur yang saya pilih untuk mengelolanya secara eksplisit:

```
/agent/shared/   ~80% logika umum: transport WSS, heartbeat, command dispatcher,
                 config, retry/backoff, telemetry, inventory core.
/agent/windows/  WMI/COM: Windows Update Agent API (patch), MSI/EXE (software),
                 UAC elevation (runas), DXGI/GDI capture, SendInput.
/agent/linux/    apt/dnf/pacman (patch), dpkg/rpm (software), sudo/pkexec,
                 X11/Wayland capture (pure-Go xgb), uinput/XTest.
/agent/macos/    softwareupdate + brew (patch), pkg/brew (software), sudo + TCC,
                 ScreenCaptureKit/CGDisplay (capture via cgo-free wrapper).
```

Mekanisme: interface di `shared` (mis. `PatchProvider`, `PackageInstaller`,
`ScreenCapturer`, `InputInjector`), implementasi per-OS dibungkus `//go:build` tag.
Saat compile, hanya implementasi OS target yang aktif.

**Yang harus ditegaskan:** divergensi antar-OS itu nyata dan dalam — bukan sekadar
"`if runtime.GOOS == ...`". Windows Update COM API, output parsing apt/dnf, dan
`softwareupdate -l` adalah tiga dunia berbeda. Karena itu setiap lapisan OS-spesifik
mendapat fase Eksekusi + Testing tersendiri, dan status readiness dilaporkan per-OS.

### 2.2 Trade-off yang dipertimbangkan

- **Rust agent** — safety lebih baik, tapi tidak ada `cargo` di mesin ini dan kurva
  review/audit tinggi. Tidak dipilih.
- **Node agent** — overhead besar di endpoint production untuk RMM; paket native
  berbulan. Tidak dipilih.
- **.NET agent** — `dotnet` tidak terinstal; runtime besar per endpoint. Tidak dipilih.
- **Postgres dari awal** — lebih dekat produksi, tapi daemon Docker mati & tidak ada
  binary Postgres. Dipilih: SQLite sekarang, adapter layer untuk Postgres nanti.
- **Redis/NATS eksternal** — lebih kuat untuk job queue, tapi prinsip "bisa jalan di
  mesin ini tanpa infra" diprioritaskan dulu. Queue embedded, interface-agnostic.

---

## 3. Keterbatasan teknis yang tidak bisa ditutupi (diakui sejak awal)

1. **Uji agent Linux & macOS** — tidak ada WSL, Hyper-V nonaktif, Docker daemon mati.
   Sampai salah satunya aktif, modul-modul yang punya kode OS-spesifik non-Windows
   **hanya bisa mencapai `CODE COMPLETE (UNTESTED)`**, tidak `TESTED (STAGING)`.
2. **Kompiler C tidak ada** — mengecualikan x264-go (cgo wajib, dibuktikan di bawah)
  dan go-sqlite3. Karena itu dipilih: `modernc.org/sqlite` (pure-Go) dan MJPEG pure-Go.
3. **Encoder video untuk remote control** —详见 section 4.
4. **Driver kernel / privileged enforcement** (web-filter di layer network, anti-tamper
   agent) — butuh sertifikat **code-signing** (EV untuk Windows). Tidak ada di sini,
   tidak akan dipura-pura selesai. Web filter di-phase-1 memakai enforcement level
   aplikasi (hosts/DNS resolver agent), bukan kernel filter.
5. **Relay server publik** — Anda memilih opsi "tidak ada VPS" **dan** "sediakan VPS".
   Relay dirancang agar URL-nya konfigurabel: jalan di localhost untuk dev
   (ditandai *local-only*), dan tinggal arahkan ke VPS/tunnel saat tersedia.
6. **AD/SSO** — sesuai keputusan Anda: lokal auth dulu (user/pass + JWT), LDAP/OIDC
   menyusul di fase lanjut.

---

## 4. Remote Control — analisis teknis jujur

### 4.1 Hasil verifikasi encoder (langsung ke sumbernya)

| Library | Hasil cek | Catatan |
|---|---|---|
| `github.com/pion/vpx` | **404 di proxy.golang.org** | Sering disebut di berbagai referensi, tapi **tidak ada** sebagai modul publik. |
| `github.com/pion/x264` | **404 di proxy.golang.org** | Sama — tidak ada. |
| `github.com/pion/datachannel/v2` | 404 | Versi v2 tidak tersedia. |
| `github.com/gen2brain/x264-go` v0.4.0 | Ada, tapi **tidak pure-Go** | README: *"C source code is included in package"* → **cgo + compiler C wajib**. Diblokir oleh mesin ini. |
| `github.com/icza/mjpeg` | **Ada, pure-Go, no cgo** | Diverifikasi: go.mod tanpa require; `mjpeg.go` tanpa `import "C"`. |
| `github.com/pion/webrtc/v4` v4.2.9 | Ada | Ada di proxy. |

### 4.2 Konsekuensi rencana remote control

Karena **tidak ada encoder H.264/VP8 pure-Go yang terverifikasi**, rencana remote control:

1. **Transport**: Pion WebRTC (peerconnection + datachannel) untuk real-time input/output.
2. **Video**: MJPEG over datachannel untuk tahap awal — pure-Go, **terbukti build tanpa cgo**.
   Frame JPEG dipush sebagai datachannel messages; viewer render di `<canvas>`.
3. **Kelemahan yang diakui**: MJPEG lebih boros bandwidth dibanding H.264. Untuk 500
   device dengan banyak session simultan, ini **nyata**. Pada fase lanjut, ganti ke
   H.264 begitu compiler C / VPS tersedia (path-nya disiapkan sejak awal via interface
   `VideoEncoder`).
4. **Input injection**: per-OS, pure-Go di Windows (`SendInput` via `golang.org/x/sys/windows`
   atau wrapper syscall); X11 via pure-Go xgb di Linux. **Mac input injection membutuhkan
   permission Accessibility (TCC)** — itu interaksi admin manual, bukan hal teknis yang
   bisa saya otomatisasi penuh. Akan ditandai sebagai blocker macOS.

### 4.3 Estimasi effort yang jujur

Remote control adalah modul paling kompleks. Membangun capture + encode + stream +
input injection + clipboard + file transfer + chat + recording di 3 OS **bukan hal yang
satu sesi selesai**. Rencana: sub-fase terpisah (relay signaling → capture+stream →
input → clipboard/transfer → recording), masing-masing dengan audit-nya sendiri.

---

## 5. Modul tambahan yang disetujui

Anda menyetujui 4 dari 5 usulan. Yang ini **saya tunggu dulu** (tidak dieksekusi diam-diam):

| Modul | Status | Rencana |
|---|---|---|
| Agent Self-Update | ✅ disetujui | Masuk sebagai bagian Core (blokir semua modul lain jika tidak ada). |
| Bandwidth Throttling / Staggered Rollout | ✅ disetujui | Infra inti job scheduler: batch 25-50 device, jeda konfigurabel. |
| Notification/Alerting | ✅ disetujui | Core hook (webhook/email/Telegram), dipanggil dari modul lain. |
| Asset & License Management | ✅ disetujui | Modul terpisah, fase akhir setelah Device Management matang. |
| Task Scheduler / Script Repository | ✅ **disetujui** (keputusan Fase 0) | Modul terpisah di fase menengah, setelah Software Deployment. Jalankan script PowerShell/bash ke grup device. |

---

## 6. Arsitektur High-Level (agent-initiated, NAT-friendly)

```
  [Endpoint Cabang]                          [Server Pusat]
  ┌────────────────┐                         ┌───────────────────────┐
  │  Agent (Go)    │  ── WSS outbound ────►  │  API Gateway (TLS)    │
  │  - heartbeat   │  (agent initiated)      │  - auth/RBAC          │
  │  - cmd runner  │  ◄── commands push ──── │  - command queue      │
  │  - inventory   │  ── telemetry ────────► │  - job scheduler      │
  │  - patch exec  │                         │  - DB (SQLite→PG)     │
  └────────────────┘                         └───────────────────────┘
        │                                              │
        │           ┌───────────────────────┐          │
        └── WSS ───► │  Relay / Signaling   │ ◄── WSS ─┘
        (outbound)  │  (WebRTC SFU/TURN)   │ (technician)
                    └───────────────────────┘
```

**Kunci desain (memenuhi konstrain spec):**

- **Server tidak pernah inisiasi koneksi TCP ke agent.** Semua "push" ke agent lewat
  WebSocket yang **sudah dibuka agent** (outbound dari sisi device).
- **Remote control**: agent dan technician **sama-sama** connect outbound ke relay;
  relay mem-broker ICE candidates. P2P jika memungkinkan, fallback relay TURN jika NAT
  ketat. **Tidak ada port inbound yang dibuka di kantor cabang.**
- **Skala 500+**: job queue + staggered rollout (batch 25-50, jeda konfigurabel);
  index DB pada `(device_id, created_at)`; connection pooling; ID per device.
- **Keamanan**: TLS 1.3; token unik per device saat enrollment (hash disimpan, bukan
  plaintext); JWT untuk admin; RBAC middleware; audit log untuk setiap remote session.

---

## 7. Struktur Folder (mengikuti spec)

```
/server                      # backend pusat
  /modules
    /patch-management
    /software-deployment
    /remote-control
      /signaling-relay
      /session-recording
    /reports
    /user-management
    /web-filter
    /device-management
    /dashboard
    /notification            # disetujui
    /agent-update            # disetujui (bisa di /core)
    /asset-license           # disetujui, fase akhir
  /core                      # auth, RBAC, shared infra, queue, throttle
  /db                        # migration & schema
/agent
  /shared
  /windows
  /linux
  /macos
/web-console                 # React + Vite + TS
/docs
  /architecture
  /readiness-reports
/tests
  /unit
  /integration
  /e2e
```

---

## 8. Urutan Eksekusi (sesuai spec bagian 5)

| Fase | Modul | Gate |
|---|---|---|
| 0 | Brainstorming arsitektur (dokumen ini) | **approve Anda** |
| 1 | Core & Infra: auth, RBAC, enrollment, heartbeat, transport WSS | konfirmasi |
| 2 | Device Management | konfirmasi |
| 3 | Dashboard (versi awal) | konfirmasi |
| 4 | Software Deployment | konfirmasi |
| 5 | Patch Management | konfirmasi |
| 6 | Remote Control (paling kompleks, sub-fase) | konfirmasi |
|  sesudahnya | Reports → User Management → Web Filter → Asset/License → Full E2E | konfirmasi |

---

## 9. Pertanyaan Terbuka (butuh jawab Anda di Fase 1)

1. **Task Scheduler / Script Repository** — ✅ **disetujui** di Fase 0. Diposisikan
   setelah Software Deployment.
2. **git** — mau Anda install? (`winget install Git.Git`) Tanpa git, tidak ada history
   perubahan dan tidak ada branch strategi. Saya rekomendasikan dipasang sebelum Fase 1.
3. **VPS untuk relay** — kapan akan tersedia? Menentukan apakah remote control bisa
   melewati status "local-only tested".

---

## 10. Production Readiness Scorecard — awal proyek

Semua modul belum dimulai. Scorecard ini akan di-update setiap sesi.

| Modul | Status | Ditest E2E? | Blocker |
|---|---|---|---|
| Core/Auth | NOT STARTED | — | — |
| Device Management | NOT STARTED | — | — |
| Dashboard | NOT STARTED | — | — |
| Software Deployment | NOT STARTED | — | — |
| Patch Management | NOT STARTED | — | — |
| Remote Control | NOT STARTED | — | Encoder pure-Go; relay public; compiler C untuk H.264 |
| Reports | NOT STARTED | — | — |
| User Management | NOT STARTED | — | AD/SSO sengaja ditunda |
| Web Filter | NOT STARTED | — | Code-signing cert untuk kernel-level enforcement |
| Agent Self-Update | NOT STARTED | — | Code-signing untuk update binary |
| Notification/Alerting | NOT STARTED | — | — |
| Bandwidth/Staggered Rollout | NOT STARTED | — | — |
| Asset & License Management | NOT STARTED | — | — |
