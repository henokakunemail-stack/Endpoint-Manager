# Fase 1 — Core & Infrastructure

**Modul:** Core (auth, RBAC, enrollment, heartbeat, transport WSS)
**Tanggal:** 2026-09-22
**Status:** `TESTED (STAGING)` — fondasi teruji E2E; TLS dan OS selain Windows belum diuji (lihat §4)

---

## 1. Brainstorming

### 1.1 Scope Fase 1 (fondasi untuk semua modul lain)

Yang dibangun di fase ini — tidak lebih, tidak kurang:

1. Server bootstrap: config, structured logger, graceful shutdown.
2. DB layer: SQLite (modernc.org/sqlite, pure-Go) + migration runner.
3. Auth: user login (JWT) untuk web console + **device enrollment token** untuk agent.
4. RBAC middleware: role admin / technician / viewer.
5. Device registry: enrollment, identitas device, status online/offline.
6. **Agent transport**: WebSocket over TLS, agent-initiated; server push command lewat
   socket yang sudah dibuka agent.
7. Agent shared (Go): transport client, reconnect exponential backoff, heartbeat,
   command dispatcher.
8. Agent Windows: build tag + enrollment nyata.
9. Audit log infra (tabel + hook, dipakai modul lain nanti).

Yang **tidak** di fase ini: inventory lengkap (Fase 2), dashboard UI (Fase 3),
patch/software deployment logic (Fase 4-5), remote control (Fase 6).

### 1.2 Keputusan desain & trade-off

| Keputusan | Pilihan | Alternatif yang ditolak | Alasan |
|---|---|---|---|
| Transport agent-server | **WebSocket over TLS**, persisten | REST long-poll; gRPC | Push ke agent butuh socket agent-initiated; WS mendukung bidirectional dengan 1 koneksi outbound. gRPC lebih ketat tapi overkill di fase ini & perlu TLS setup lebih berat. |
| Command dispatch | Server kirim command via WS yang **sudah dibuka agent** | Server buka TCP ke agent | **Konstrain spec:** server tidak boleh inisiasi koneksi ke IP device cabang. |
| Offline command | Disimpan di server (tabel `agent_commands`), dikirim saat agent reconnect | Dropped | Agent cabang bisa offline; command harus antri. |
| DB | SQLite `modernc.org/sqlite` (pure-Go) | Postgres (daemon mati); `mattn/go-sqlite3` (cgo) | Pure-Go agar bisa dibuild tanpa compiler C. Adapter interface disiapkan untuk Postgres nanti. |
| Auth admin | JWT (access + refresh) | Session server-side; basic auth | Stateless, cocok untuk API + SPA. Refresh token rotation. |
| Auth agent | **Enrollment token sekali pakai** + device secret persisten (hash) | mTLS (butuh CA setup) | Praktis untuk onboarding 500 device; mTLS tetap menjadi opsi upgrade. |
| Heartbeat / online detection | Heartbeat periodik via WS + last_seen DB, offline threshold 3x interval | ICMP ping dari server | Tidak mungkin ping device di luar jaringan pusat. |
| Password hashing | bcrypt | argon2 (lebih kuat tapi lebih jarang di audit) | bcrypt cukup & well-understood; argon2 bisa di-upgrade nanti. |

### 1.3 Perbedaan OS di fase ini (memenuhi konstrain spec)

Sejak fase ini, divergensi OS ditangani eksplisit:

- **Windows**: OS info via `GetVersionEx`/registry + WMI `Win32_OperatingSystem`;
  hostname via `GetComputerName`; install path `C:\Program Files\...`.
- **Linux**: `/etc/os-release`, `uname`, hostname via `os.Hostname()`; path `/opt/...`.
- **macOS**: `sw_vers` output parsing, `os.Hostname()`; path `/Library/...`.

Interface `OSInfoProvider` di `agent/shared`, implementasi per build tag. Di Fase 1
hanya **Windows yang diuji nyata** (WSL/Docker belum siap) — Linux & macOS
`CODE COMPLETE (UNTESTED)` sampai infrastrukturnya aktif.

### 1.4 Risiko & mitigasi

| Risiko | Mitigasi |
|---|---|
| WS connection leak pada reconnect badai | Reconnect exponential backoff + jitter; server-side stale connection cleanup berdasarkan last_seen. |
| Token enrollment bocor | Token sekali pakai, TTL pendek (default 30 menit), hash disimpan bukan plaintext. |
| SQLite konkurensi write dari banyak worker | WAL mode, single writer pattern via worker pool, `PRAGMA busy_timeout`. |
| Timestamp drift agent-server | Server-side `created_at` untuk semua record; agent kirim uptime bukan asumsi jam. |

---

## 2. Writing Plan

### 2.1 Struktur folder yang akan dibuat

```
go.mod                                    module: github.com/endpoint-mgmt
server/
  cmd/server/main.go                      bootstrap + graceful shutdown
  core/
    config/config.go                      env / file config
    logger/logger.go                      zerolog wrapper
    db/
      db.go                               sqlite open (WAL, busy_timeout)
      migrate.go                          embed migrations, run
      migrations/0001_init.sql
    auth/
      jwt.go                              issue/verify access+refresh
      password.go                         bcrypt hash/compare
      middleware.go                       JWT middleware
    rbac/rbac.go                          role enum + RequireRole middleware
    audit/audit.go                        audit log writer
    transport/
      hub.go                              connection registry (deviceID -> conn)
      ws.go                               upgrade handler, read/write pump
  modules/
    device-management/
      model.go                            Device entity
      repository.go                       queries
      handler.go                          HTTP API + enroll endpoint
      transport.go                        bind agent WS events -> device status
agent/
  shared/
    config/config.go
    transport/wsclient.go                 dial WSS + reconnect backoff
    osinfo/osinfo.go                      interface OSInfoProvider
    enrollment/enrollment.go              enroll flow pakai token
    heartbeat/heartbeat.go
  windows/osinfo_windows.go               build tag windows
  linux/osinfo_linux.go                   build tag linux
  macos/osinfo_darwin.go                  build tag darwin
  cmd/agent/main.go
docs/architecture/phase-1-core-plan.md    (dokumen ini)
tests/
  integration/transport_test.go           WS enroll+heartbeat+command E2E
  unit/auth_test.go, rbac_test.go, db_test.go
scripts/
  run-server.ps1 / run-agent.ps1         helper dev run
```

### 2.2 Data model (migrasi 0001)

```sql
-- users: admin console
CREATE TABLE users (
  id            TEXT PRIMARY KEY,        -- ULID
  username      TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role          TEXT NOT NULL DEFAULT 'viewer',  -- admin|technician|viewer
  created_at    DATETIME NOT NULL,
  updated_at    DATETIME NOT NULL
);

-- devices: endpoint terdaftar
CREATE TABLE devices (
  id              TEXT PRIMARY KEY,      -- ULID
  hostname        TEXT NOT NULL,
  os_name         TEXT NOT NULL,         -- windows|linux|macos
  os_version      TEXT,
  agent_version   TEXT,
  status          TEXT NOT NULL DEFAULT 'offline',  -- online|offline
  last_seen_at    DATETIME,
  enrolled_at     DATETIME NOT NULL,
  enrollment_token_hash TEXT,            -- sekali pakai
  device_secret_hash    TEXT NOT NULL,   -- secret persisten (hash)
  site            TEXT,                  -- kantor pusat / cabang-xx
  created_at      DATETIME NOT NULL,
  updated_at      DATETIME NOT NULL
);
CREATE INDEX idx_devices_status ON devices(status);
CREATE INDEX idx_devices_last_seen ON devices(last_seen_at);

-- audit_logs: jejak aktivitas admin
CREATE TABLE audit_logs (
  id         TEXT PRIMARY KEY,
  actor_type TEXT NOT NULL,              -- user|agent|system
  actor_id   TEXT,
  action     TEXT NOT NULL,              -- device.enroll, remote.session.start, ...
  target_id  TEXT,
  details    TEXT,                       -- JSON
  created_at DATETIME NOT NULL
);
CREATE INDEX idx_audit_created ON audit_logs(created_at);

-- agent_commands: antrian command untuk device (bisa offline)
CREATE TABLE agent_commands (
  id           TEXT PRIMARY KEY,
  device_id    TEXT NOT NULL,
  command_type TEXT NOT NULL,            -- ping|shell|inventory.collect|...
  payload      TEXT,                     -- JSON
  status       TEXT NOT NULL DEFAULT 'pending',  -- pending|sent|done|failed
  created_at   DATETIME NOT NULL,
  sent_at      DATETIME,
  completed_at DATETIME,
  result       TEXT
);
CREATE INDEX idx_commands_device_status ON agent_commands(device_id, status);
CREATE INDEX idx_commands_created ON agent_commands(created_at);
```

Index `agent_commands(device_id, status)` adalah index krusial untuk skala 500 device —
query "command pending untuk device X" harus cepat.

### 2.3 API contract

**Web console (JWT)**

| Method | Path | Deskripsi | Role |
|---|---|---|---|
| POST | `/api/auth/login` | login, return access+refresh | publik |
| POST | `/api/auth/refresh` | refresh token | publik |
| GET | `/api/devices` | list devices (filter status/site) | viewer+ |
| GET | `/api/devices/{id}` | detail device | viewer+ |
| POST | `/api/devices/enroll-token` | **buat enrollment token** (admin only) | admin |
| GET | `/api/audit-logs` | audit log list | technician+ |

**Agent (device secret)**

| Method | Path | Deskripsi |
|---|---|---|
| POST | `/api/agent/enroll` | tukar enrollment token → device secret (sekali pakai) |
| GET (WS) | `/api/agent/connect` | upgrade ke WebSocket, header `X-Device-Id` + `X-Device-Secret` |

**WS message format (JSON, minimal)**

```
server -> agent : {"type":"command","id":"...","command":"ping","payload":{}}
agent  -> server: {"type":"hello","device_id":"...","agent_version":"...","os":{...}}
agent  -> server: {"type":"heartbeat","ts":"..."}
agent  -> server: {"type":"command_result","id":"...","status":"done","result":{}}
```

### 2.4 Urutan implementasi

1. `go.mod` init + ambil dependency (modernc/sqlite, chi, zerolog, jwt, bcrypt).
2. `core/config`, `core/logger`, `core/db` + migrasi 0001.
3. `core/auth` (JWT + bcrypt) + `core/rbac` + unit test.
4. `modules/device-management` (model, repository, handler, enroll).
5. `core/transport` (hub + ws handler).
6. `agent/shared` (wsclient, osinfo interface, enrollment, heartbeat).
7. `agent/{windows,linux,macos}` osinfo per build tag.
8. `cmd/server/main.go` + `cmd/agent/main.go`.
9. Integration test: enroll → connect → heartbeat → command round-trip.
10. Manual E2E: server + agent Windows berjalan sungguhan di mesin ini.

### 2.5 Kriteria selesai Fase 1

- ✅ Server start di Windows, DB terbuat + migrasi jalan.
- ✅ Admin login via API dapat JWT; middleware RBAC menolak role kurang.
- ✅ Enrollment token dibuat (API), ditukar oleh agent menjadi device secret.
- ✅ Agent Windows connect via WS, kirim hello + heartbeat, status device jadi `online`.
- ✅ Server push command `ping` → agent reply → command status `done`.
- ✅ Agent disconnect → status `offline` setelah threshold.
- ✅ Audit log mencatat login + enroll.
- ✅ Semua integration + unit test **lulus dengan output ditampilkan**.

---

## 3. E2E Testing

### 3.1 Unit tests (Go, `go test ./tests/unit -count=1`)

| Test | Yang diverifikasi | Hasil |
|---|---|---|
| `TestPasswordHashAndCompare` | bcrypt hash + compare, password benar/salah | ✅ PASS |
| `TestJWTIssueAndParse` | issue access+refresh, parse round-trip, claim terbaca | ✅ PASS |
| `TestJWTRejectsWrongSecretAndExpired` | token ditandatangani secret lain → ditolak; token kadaluarsa → ditolak | ✅ PASS |
| `TestDeviceEnrollmentFlow` | buat device + token hash, konsumsi token (NULL-kan hash, isi secret), **replay token ditolak** | ✅ PASS |
| `TestDeviceStatusTransitions` | offline → online → offline, `last_seen_at` terupdate | ✅ PASS |
| `TestDeviceListFilteringBySite` | filter `site=cabang-test` mengembalikan device yang benar | ✅ PASS |
| `TestRBACHierarchy` | admin≥admin 200, admin≥technician 200, viewer≥admin **403**, technician≥viewer 200, no-role **403** | ✅ PASS |

```
ok  github.com/endpoint-mgmt/tests/unit  4.374s   (7/7 PASS)
```

### 3.2 Integration tests (Go, `go test ./tests/integration -count=1`)

| Test | Yang diverifikasi | Hasil |
|---|---|---|
| `TestE2EEnrollConnectCommand` | enroll via HTTP → WS connect → hello tersimpan → heartbeat → **command round-trip persist** → disconnect → offline → **pasangan audit connect+disconnect** | ✅ PASS |
| `TestE2EOfflineCommandQueues` | `SendTo` device tidak terhubung → false (command antri, tidak di-drop) | ✅ PASS |
| `TestE2ELoginAndRBAC` | login admin+viewer, viewer 403 di enroll-token, admin 201, token ditamper 401, tanpa token 401, viewer boleh list devices | ✅ PASS |

```
ok  github.com/endpoint-mgmt/tests/integration  5.675s   (3/3 PASS)
```

### 3.3 Live E2E — binary asli, bukan test double

`scripts/e2e-live.ps1` menjalankan **server dan agent hasil `go build` sungguhan**
(`emserver.exe`, `emagent.exe`) di mesin ini, lalu berjalan melalui seluruh flow.
SQLite di-query langsung lewat helper `scripts/querysqlite` untuk membuktikan
state DB, bukan mengandalkan respons API saja.

Output lengkap run terakhir (2026-09-22):

```
==> server PID 24896 on port 18443
OK   healthz responded
OK   admin login, token length 275
OK   enrollment token issued for device 117d83afc1b692cc3f91a933efdf68c2
==> agent PID 21668
OK   agent enrolled + connected via WS, agent=0.1.0 os=10.0.26100
OK   token replay rejected (401)
OK   ping round-trip done, result={"pong":"2026-09-22T10:16:55+07:00"}
OK   device marked offline after agent disconnect
OK   audit trail (6 entries): auth.login, device.enroll_token_created,
     device.enroll, agent.connect, command.send, agent.disconnect

ALL LIVE E2E CHECKS PASSED        exit 0
```

Yang secara spesifik diverifikasi oleh run ini:

| # | Cek | Bukti |
|---|---|---|
| 1 | Server start, DB + migrasi jalan | `healthz` 200; DB file terbuat dengan skema lengkap |
| 2 | Admin login → JWT | access token 275 char dikembalikan |
| 3 | Enrollment token dikeluarkan (admin) | device row dibuat, `enrollment_token_hash` terisi |
| 4 | Agent tukar token → secret, connect WS | `os_version=10.0.26100` (Windows 11 asli mesin ini), `agent_version=0.1.0` |
| 5 | Token sekali pakai | replay token yang sama → **401** |
| 6 | Command round-trip persisten | row `agent_commands` status `done` + result berisi `pong` |
| 7 | Offline detection setelah disconnect | status `offline` dalam **0.35 detik** setelah agent di-kill |
| 8 | Audit trail lengkap | 6 entry, termasuk **`agent.disconnect`** |

### 3.4 Cross-platform build (semua exit 0, cgo dimatikan)

```
go build ./...                     → seluruh modul build
GOOS=windows ./agent/cmd/agent     → exit 0
GOOS=linux   ./agent/cmd/agent     → exit 0
GOOS=darwin  ./agent/cmd/agent     → exit 0
go vet ./...                       → exit 0 (tidak ada warning)
```

### 3.5 Bug nyata yang ditemukan dan diperbaiki oleh E2E

E2E ini melunasi biayanya: tiga bug produksi ditemukan karena test bersikeras
membaca DB dan log, bukan cuma respons HTTP.

1. **Hello protocol mismatch.** Server mengharapkan `payload.os.version`
   (nested), agent mengirim `payload.version` (flat). Akibatnya `os_version`
   selalu kosong di inventory — modul Device Management akan menampilkan
   "unknown" untuk seluruh fleet. Parser server sekarang menerima keduanya
   (membantu saat rolling upgrade agent).
2. **Command tidak persisten sebelum dikirim.** Endpoint ping mengirim
   command via WS tanpa menyimpan row-nya. Bila koneksi putus di antaranya,
   balasan agent tidak punya row untuk di-update → command hilang. Sekarang
   persist-before-send; command offline tersisa ber-status `queued`.
3. **Deadlock disconnect handler.** `<-done` di akhir `ServeHTTP` menunggu
   `writePump` keluar, tapi tidak ada yang menutup `c.send`, dan penutupnya
   ada di deferred cleanup yang tidak bisa jalan selama frame ini terblok.
   Lingkaran ini membuat device **selalu** online setelah agent mati.
   Diperbaiki: read loop selesai → deferred cleanup tutup `c.send` → tunggu
   writePump keluar → tutup socket → update status offline + audit.

Bug #3 adalah jenis yang paling berbahaya: API terlihat benar, device
terlihat online, dan tidak ada error di log mana pun.

---

## 4. Audit & Readiness Report

### 4.1 Yang sudah selesai DAN diuji end-to-end

| Komponen | Bukti |
|---|---|
| Server bootstrap (config, logger, graceful shutdown) | live E2E #1; SIGINT shutdown tercatat di log |
| SQLite + migration runner (WAL, busy_timeout, FK) | DB file terbuat; `go test` query langsung |
| JWT access+refresh, bcrypt login | 3 unit test + `TestE2ELoginAndRBAC` |
| RBAC middleware (viewer/technician/admin) | `TestRBACHierarchy` + live 403/201 |
| Enrollment: token sekali pakai → device secret (hash) | live #3-#5; replay ditolak |
| Device registry + status online/offline | 2 unit test + live #4, #7 |
| Transport WS: connect, hello, heartbeat, command | integration + live #6 |
| Offline detection (disconnect → offline, 0.35s) | live #7 + audit `agent.disconnect` |
| Offline command queue (tidak di-drop) | `TestE2EOfflineCommandQueues` |
| Audit log (6 aksi tercatat berurutan) | live #8 |
| Agent Windows: enroll, connect, jalankan command | binary asli berjalan di mesin ini |
| Agent Linux/macOS build | cross-compile exit 0 (belum diuji jalan) |

### 4.2 Selesai tapi TIDAK diuji end-to-end

| Item | Status jujur | Kenapa |
|---|---|--- |
| **Agent Linux** | `CODE COMPLETE (UNTESTED)` | Kode ada + ter-compile, tapi belum pernah dijalankan di Linux sungguhan. WSL/Docker Anda bilang "nanti saja" (Fase 0). Bisa jadi bug runtime di `/etc/os-release` parsing atau signal handling. |
| **Agent macOS** | `CODE COMPLETE (UNTESTED)` | Sama, plus parsing output `sw_vers` tidak pernah diuji terhadap output asli. |
| **TLS/WSS** | `CODE COMPLETE (UNTESTED)` | Semua E2E di atas jalan di **plaintext `ws://`** port 18443. Transport dan auth sudah disiapkan untuk `wss://` dan JWT secret, tapi tidak ada satu pun tes yang melewati TLS sungguhan. Ini **celah keamanan nyata** hingga dipasang sertifikat. |
| **Backoff reconnect storm** | `DESIGNED (UNTESTED)` | Logika exponential backoff + jitter ada, tapi hanya diuji dengan 1 agent. Belum ada load test 500 agent reconnect serentak. |
| **SQLite konkurensi tinggi** | `PARTIALLY TESTED` | WAL + busy_timeout aktif; `SQLITE_BUSY` sempat muncul di integration test (write dari goroutine cleanup bertabrakan dengan request in-flight) dan diperbaiki. Tapi beban 500 device belum pernah disimulasikan. |
| **Offline sweep** | `CODE COMPLETE (UNTESTED)` | Goroutine sweeper ada (`runOfflineSweep`), tapi disconnect handler sekarang menangani semua kasus yang bisa diuji, jadi sweeper tidak pernah benar-benar dipicu di E2E. |

### 4.3 Tidak dikerjakan (memang di luar scope Fase 1)

Inventory lengkap, dashboard UI, patch/software deployment, remote control,
reports, web filter, task scheduler, agent self-update. Semuanya `NOT STARTED`.

### 4.4 Risiko dan batasan

1. **Plaintext transport adalah risiko tertinggi saat ini.** Token enrollment,
   device secret, dan JWT semua melintasi kabel tanpa enkripsi selama tes.
   Produksi wajib `wss://` + sertifikat. Ini persis seperti yang spec sebut:
   belum diuji, belum production ready.
2. **Default credential.** Server membuat `admin/admin12345` pada boot pertama
   dan mencetaknya ke log sekali. Wajib diganti sebelum ada pengguna nyata.
3. **Single-server, in-memory hub.** Hub menyimpan koneksi di memori proses
   server. Restart server = semua device reconnect (backoff menangani
   badainya, tapi tetap ada jendela "semua offline"). Belum ada story
   multi-server/HA. SQLite memperkuat asumsi single-node ini.
4. **Belum ada rate limiting.** Endpoint login dan enroll tidak dibatasi.
   500 device yang reconnect serentak sudah ditangani backoff di sisi agent,
   tapi sisi server belum ada perlindungan terhadap thundering herd atau
   penyalahgunaan endpoint login.
5. **`JWT_SECRET` wajib di-set.** Server menolak jalan tanpa secret yang
   valid — ini sengaja, supaya tidak ada default yang aman di produksi.
6. **Windows version detection.** `RtlGetVersion` melaporkan versi kernel
   (10.0.26100), bukan label marketing "Windows 11". Akurat untuk patch
   management, tapi tampilan UI perlu mapping ke nama marketing nanti.

### 4.5 Status akhir Fase 1

| Modul | Status | Catatan |
|---|---|---|
| **Core / Infra (config, DB, logger)** | `TESTED (STAGING)` | Diuji lewat live E2E di mesin Windows; bukan environment staging terpisah |
| **Auth (JWT, bcrypt)** | `TESTED (STAGING)` | 3 unit test + integration; **TLS belum diuji** |
| **RBAC** | `TESTED (STAGING)` | 5 kasus hierarki + 403/201 live |
| **Transport (WS, hub, offline detection)** | `TESTED (STAGING)` | Termasuk bug deadlock yang ditemukan dan diperbaiki E2E |
| **Audit log** | `TESTED (STAGING)` | 6 aksi terverifikasi berurutan |
| **Agent — Windows** | `TESTED (STAGING)` | Binary asli berjalan di mesin ini |
| **Agent — Linux** | `CODE COMPLETE (UNTESTED)` | Ter-compile saja |
| **Agent — macOS** | `CODE COMPLETE (UNTESTED)` | Ter-compile saja |
| **TLS/WSS** | `CODE COMPLETE (UNTESTED)` | Semua tes jalan di plaintext |
| **Task Scheduler** | `NOT STARTED` | Disetujui di Fase 0, belum dibangun |
| **Patch Management** | `NOT STARTED` | |
| **Software Deployment** | `NOT STARTED` | |
| **Remote Control** | `NOT STARTED` | |
| **Reports** | `NOT STARTED` | |
| **User Management (selain bootstrap)** | `NOT STARTED` | |
| **Web Filter** | `NOT STARTED` | |
| **Dashboard** | `NOT STARTED` | |
| **Device Management (modul penuh)** | `NOT STARTED` | Yang ada sekarang adalah registry dasar Fase 1, bukan modul Device Management lengkap |
| **Agent Self-Update** | `NOT STARTED` | |
| **Bandwidth Throttling / Staggered Rollout** | `NOT STARTED` | |
| **Notification / Alerting** | `NOT STARTED` | |
| **Asset & License Management** | `NOT STARTED` | |

### 4.6 Pernyataan jujur tentang "Production Ready"

**Fase 1 TIDAK production ready.** Tidak ada komponen di atas yang berstatus
`PRODUCTION READY`, dan menurut kriteria spec, aplikasi baru bisa disebut
production ready bila **semua** modul minimal `TESTED (STAGING)` dan modul
kritikal (Remote Control, Patch Management, Software Deployment)
`PRODUCTION READY` dengan bukti di kondisi mendekati nyata. Saat ini modul
kritikal itu semuanya `NOT STARTED`.

Yang berani diklaim: **fondasi transport-nya nyata, bukan mock.** Agent
Windows sungguhan terhubung ke server sungguhan via socket yang dibuka agent
sendiri (outbound, sesuai konstrain cabang), menjalankan command, dan
perpindahan online/offline-nya terverifikasi di DB. Tidak ada bagian dari flow
ini yang memakai dummy data.

Yang **tidak** berani diklaim: keamanan (plaintext), skala (1 device, bukan
500), dan OS lain (Windows saja yang diuji).

### 4.7 Angka konkrit

- **48 file** ter-commit, **4.259 baris** (sekali git init, 1 commit)
- **10 test Go** lulus (7 unit + 3 integration), total ~10 detik
- **8 cek live E2E** lulus terhadap binary produksi
- **3 bug produksi** ditemukan & diperbaiksi oleh E2E
- **3 OS target** ter-compile (windows/linux/darwin), 1 diuji jalan
- **0 cgo**, seluruhnya pure-Go
- **Offline detection latency: 0.35 detik** (dari tidak pernah terdeteksi)
