# Contributing to Endpoint Management Platform

Thank you for your interest in improving this project!

## Quick Links

- **Issues**: [GitHub Issues](https://github.com/henokakunemail-stack/Endpoint-Manager/issues)
- **Discussions**: [GitHub Discussions](https://github.com/henokakunemail-stack/Endpoint-Manager/discussions)
- **Documentation**: `docs/` directory

## How to Contribute

### 1. Reporting Bugs

- Search existing issues first to avoid duplicates
- Use the bug report template
- Include: Go version, OS, reproduction steps, expected vs actual behavior
- If possible, add a failing test case

### 2. Suggesting Features

- Open a discussion first for anything beyond a trivial change
- Explain the use case and why it belongs in core vs a plugin/extension
- Consider the maintenance burden of the proposed feature

### 3. Code Contributions

#### Prerequisites

- Go 1.24+ (the `go.mod` declares `go 1.26.8`)
- Node.js 20+ for the web console (`web-console/`)
- SQLite development headers are **not** required — this project uses
  `modernc.org/sqlite` (pure Go)

#### Build & Test Locally

```bash
# Frontend (run once, or when web-console source changes)
cd web-console
npm install
npm run build

# Backend + agent (all targets)
CGO_ENABLED=0 go build ./...

# Run all tests
CGO_ENABLED=0 go test -count=1 ./...

# Cross-compile agents
./scripts/build-all-agents.sh   # if present, or manual GOOS/GOARCH
```

#### Coding Standards

- **Go**: `gofmt` / `gofumpt`, `golangci-lint` (if configured), meaningful
  variable names, error wrapping with `%w`
- **TypeScript/React**: `eslint`, `prettier`, functional components + hooks
- **Commits**: Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`,
  `refactor:`, `test:`)
- **Tests**: Table-driven where applicable; `*_test.go` in the same package;
  integration tests in `tests/integration/`

#### Pull Request Checklist

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test -count=1 ./...` passes
- [ ] Frontend: `npm run lint && npm run build` (if web-console changed)
- [ ] New code has tests (unit or integration)
- [ ] Documentation updated (README, docs/, or code comments)
- [ ] No hardcoded secrets, domains, or machine-specific paths
- [ ] Commits are clean and follow Conventional Commits

### 4. Security Issues

See [SECURITY.md](SECURITY.md) — **do not** file public issues for
vulnerabilities.

## Project Structure Overview

```
.
├── agent/                    # Outbound agent (Pure Go, 5 targets)
│   ├── cmd/agent/            # Entrypoint, inventory, OS info
│   └── shared/               # Subsystems: enrollment, inventory, networkfilter,
│                             # patch, remotecontrol, remoteexec, software,
│                             # transport, update
├── deploy/                   # Ubuntu/Debian server install bundle
│   ├── install-ubuntu.sh     # Parameterized installer
│   ├── nginx-endpoint.conf.template
│   └── endpoint-mgmt.service
├── docs/                     # Architecture specs, installation guides,
│   └── readiness-reports/    # Phase 0-14 audit scorecards
├── packaging/                # Agent installers (NSIS, pkg, deb) — TBD
├── scripts/                  # E2E PowerShell test suites
├── server/                   # Central management server (Pure Go)
│   ├── cmd/server/           # Entrypoint, embedded web console
│   ├── core/                 # Auth, DB, RBAC, transport, config, audit
│   └── modules/              # Business logic (Phases 3-14)
├── tests/                    # Integration + unit test suites
├── web-console/              # React 19 + TypeScript + Vite SPA
└── LICENSE, SECURITY.md, CONTRIBUTING.md
```

## Development Tips

- **Single-binary delivery**: The server embeds the React build via `embed.FS`.
  After any frontend change, run `npm run build` in `web-console/` so the
  embed is fresh on next `go build`.
- **Database migrations**: `server/core/db/migrations/0001_*.sql` — sequential,
  run once on first startup. Never edit an applied migration; add a new one.
- **Agent service layer**: `agent/shared/service/` provides `Manager`
  interface (systemd, launchd, Windows SCM). The Windows SCM handler uses
  `golang.org/x/sys/windows/svc` and runs the agent under the Service Control
  Manager — the agent binary must implement the handler contract.
- **Configuration over hardcode**: All hostnames, certificate paths, and
  credentials are environment variables. See `server/core/config/config.go`.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).