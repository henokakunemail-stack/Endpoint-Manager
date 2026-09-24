# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |

Only the latest stable release receives security updates. Backporting to
earlier releases is not guaranteed.

## Reporting a Vulnerability

**Please do not open a public issue for security vulnerabilities.**

Instead, report vulnerabilities privately through GitHub Security Advisories
or send a detailed report to the repository maintainers. Include:

- A clear description of the vulnerability and its impact
- Steps to reproduce (minimal proof-of-concept if possible)
- The affected component(s) and version(s)
- Any mitigations you are aware of

Reports will be acknowledged within 48 hours. A fix timeline will be
communicated once the issue has been triaged.

## Disclosure Process

1. Report received and acknowledged
2. Vulnerability verified and impact assessed
3. Fix developed and tested
4. Advisory prepared
5. Fix released (minor/patch version bump)
6. Advisory published after users have had time to upgrade

## Security Architecture Notes

This platform was designed with the following threat model assumptions:

- **Outbound-only agent connections**: Branch firewalls need zero inbound
  ports. Agents initiate TLS WebSocket connections to the central server.
- **Device authentication**: Every agent request carries `X-Device-Id` and
  `X-Device-Secret`. The server verifies the SHA-256 hash of the secret
  against the enrolled device record. A secret bound to one device ID cannot
  be replayed against another.
- **WebSocket origin validation**: The three WebSocket upgraders (agent
  connect, remote terminal, remote control) enforce an allowlist of browser
  origins via the `ALLOWED_ORIGIN_DOMAINS` environment variable. The default
  (empty) trusts only same-host and loopback origins.
- **Agent self-update integrity**: Binary releases are identified by SHA-256.
  The agent validates the hash before executing a downloaded update.
- **Role-based access control**: Console operators are `admin`, `technician`,
  or `viewer`. Administrative actions and sensitive data exports require
  `admin` role.
- **Audit logging**: Every administrative action, command execution, remote
  desktop session, and enrollment event is recorded with actor, timestamp, and
  details.

## Responsible Configuration

Deployers must ensure:

- `JWT_SECRET` is a high-entropy random string (64 hex chars recommended).
  The installer generates one automatically.
- `ALLOWED_ORIGIN_DOMAINS` is set only if the web console is served from a
  different hostname than the API. Leaving it empty is the safe default.
- TLS certificates are valid, rotated before expiry, and private keys are
  stored with restrictive permissions (root-only read).
- The server database directory (`/opt/endpoint-mgmt/data` by default) is
  backed up regularly and access is restricted to the service user.
- Agent deployments use a code-signing certificate so Windows SmartScreen
  does not block the installer.