# Fase 1 — Core & Infrastructure

**Modul:** Core (auth, RBAC, enrollment, heartbeat, transport WSS)
**Tanggal:** 2026-09-22
**Status:** IN PROGRESS

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

## 3. E2E Testing & 4. Audit

Akan diisi setelah eksekusi Fase 1 selesai (format sesuai spec bagian 6).
