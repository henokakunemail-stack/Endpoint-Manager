# Endpoint Management Platform

Platform manajemen endpoint terpusat untuk 500+ perangkat (Windows, Linux, macOS)
yang tersebar di kantor pusat dan cabang di luar jaringan pusat.

## Status proyek

**Fase 2 selesai (device management) — menunggu approve untuk Fase 3.**

Status per modul: `TESTED (STAGING)` untuk core/auth/RBAC/transport/audit/
agent-Windows/device-management; sisanya `NOT STARTED`. Detail + bukti test ada
di scorecard.

Lihat:
- [`docs/architecture/phase-0-brainstorming.md`](docs/architecture/phase-0-brainstorming.md)
  — pilihan tech stack, trade-off, keterbatasan teknis, rencana eksekusi.
- [`docs/readiness-reports/README.md`](docs/readiness-reports/README.md)
  — Production Readiness Scorecard (di-update setiap sesi).

## Stack (direkomendasikan di Fase 0)

| Lapisan | Teknologi |
|---|---|
| Server | Go (net/http + chi) |
| Agent | Go, build tag per-OS (Windows/Linux/macOS) |
| DB | SQLite via `modernc.org/sqlite` (pure-Go, no cgo) |
| Queue | Embedded di server (DB-backed job queue) |
| Transport agent-server | WebSocket over TLS (agent-initiated outbound) |
| Remote control | Pion WebRTC + MJPEG pure-Go |
| Web console | React + Vite + TypeScript |

## Prinsip kerja

Setiap modul melalui 5 fase eksplisit:
1. **Brainstorming** — opsi, trade-off, risiko, rekomendasi.
2. **Writing Plan** — struktur, file, urutan, data model, API contract.
3. **Execution** — implementasi nyata.
4. **E2E Testing** — pengujian dengan hasil aktual.
5. **Audit & Readiness Report** — status jujur per modul.

Aturan anti-halusinasi: tidak ada status `PRODUCTION READY` tanpa bukti test
end-to-end yang ditampilkan.

## Struktur folder (target)

```
/server/modules/*    /agent/{shared,windows,linux,macos}    /web-console
/docs/{architecture,readiness-reports}    /tests/{unit,integration,e2e}
```

## Catatan environment pengembangan

- Go 1.26.8, Node v24.20.0 tersedia.
- **Tidak ada compiler C** → semua dependency dipilih pure-Go.
- **WSL/Docker daemon mati** → agent Linux/macOS belum dapat diuji di mesin ini.
- **git terpasang** (2.55.0) tetapi **tidak di PATH** untuk sesi PowerShell;
  pakai path absolut.
