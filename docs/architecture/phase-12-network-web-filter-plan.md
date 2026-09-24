# Architectural Blueprint — Fase 12: Network & Web Filter / Security Rules

**Status:** APPROVED FOR EXECUTION  
**Target Release:** Fase 12  
**Constraints:** 100% Pure Go / Zero CGO, Outbound-Only Agent Connection, Cross-Platform Multi-OS (Windows, Linux, macOS)

---

## 1. Problem Statement & Enterprise Context

Enterprise endpoints distributed across corporate branch offices require centralized web filtering and network security perimeter enforcement. Traditional enterprise solutions often rely on kernel-mode NDIS/WFP drivers or CGO-compiled packet capture engines (e.g. WinPcap, libpcap) that introduce driver instability, BSOD risks, and cross-compilation barriers.

In this platform, **Fase 12** introduces a pure-Go, zero-driver enterprise network & web filtering engine combining:
1. **Atomic DNS Sinkholing & Hosts Security Enforcement**:
   - Manages domain blocking using managed markers (`### BEGIN ENDPOINT-MGMT MANAGED BLOCKLIST ###` ... `### END ENDPOINT-MGMT MANAGED BLOCKLIST ###`) in system resolver configs (`%SystemRoot%\System32\drivers\etc\hosts` on Windows, `/etc/hosts` on Linux/macOS).
   - Sinkholes blocked domains to `0.0.0.0` to instantaneously drop connection attempts at the OS resolver layer without performance overhead or CPU penalties.
2. **System Firewall Rule Synchronization**:
   - Enforces IP/port egress/ingress boundary rules using pure native CLI hooks (`netsh advfirewall` on Windows, `iptables`/`nftables` on Linux, `pfctl` on macOS).
3. **Policy & Category Management**:
   - Centralized rule definitions categorized by risk: `Malware / C2`, `Phishing`, `Gambling / Adult`, `Social Media / Bandwidth`, `Custom Egress Restrictions`.
   - Scope assignment to `all`, `group`, or specific `device`.
4. **Agent Continuous Compliance & Self-Healing**:
   - Agent validates that managed markers and sinkholes remain intact. If an unauthorized local process or user tampers with the hosts file, the agent self-heals and restores the policy during the next cycle.

---

## 2. Database Schema (`0011_network_filter.sql`)

```sql
-- Network & Web Filter Categories and Rule Sets
CREATE TABLE IF NOT EXISTS filter_policies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL, -- 'all', 'group', 'device'
    target_id TEXT NOT NULL DEFAULT '',
    is_enabled INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Rules within a policy (domains or network IP/ports)
CREATE TABLE IF NOT EXISTS filter_rules (
    id TEXT PRIMARY KEY,
    policy_id TEXT NOT NULL REFERENCES filter_policies(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL, -- 'domain', 'ip_port'
    pattern TEXT NOT NULL,   -- e.g. 'tiktok.com', '198.51.100.0/24:443'
    action TEXT NOT NULL DEFAULT 'block', -- 'block', 'allow'
    category TEXT NOT NULL DEFAULT 'custom',
    created_at DATETIME NOT NULL
);

-- Per-device filter compliance status
CREATE TABLE IF NOT EXISTS device_filter_states (
    device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    policy_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'synced', -- 'synced', 'pending', 'tampered', 'failed'
    rules_applied INTEGER NOT NULL DEFAULT 0,
    last_applied_at DATETIME NOT NULL,
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_filter_policies_target ON filter_policies(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_filter_rules_policy ON filter_rules(policy_id);
```

---

## 3. Server Module Structure (`server/modules/networkfilter`)

1. `repository.go`:
   - `CreatePolicy(ctx, policy)`, `UpdatePolicy(ctx, policy)`, `DeletePolicy(ctx, id)`
   - `AddRule(ctx, rule)`, `DeleteRule(ctx, id)`, `ListRulesByPolicy(ctx, policyID)`
   - `CompileEffectiveRules(ctx, deviceID)`: Evaluates hierarchy (Device > Group > All) and aggregates active blocked domains and network constraints.
   - `RecordDeviceFilterState(ctx, state)`
2. `handler.go`:
   - REST API for Policy & Rule CRUD:
     - `POST /api/filter/policies` (Admin only)
     - `GET /api/filter/policies` (Authenticated users)
     - `PUT /api/filter/policies/{id}` (Admin only)
     - `DELETE /api/filter/policies/{id}` (Admin only)
     - `POST /api/filter/policies/{id}/rules` (Admin only)
     - `DELETE /api/filter/rules/{id}` (Admin only)
     - `POST /api/devices/{id}/filter/sync` (Technician+): Dispatches `filter.apply` to agent
     - `GET /api/devices/{id}/filter/state` (Authenticated users)
     - `POST /api/agent/devices/{id}/filter/report`: Agent reports applied state
3. `agent/shared/networkfilter`:
   - `Engine`: Manages local hosts file read/update/write with atomic file swap and markers.
   - `ApplyRules(domains []string)`: Writes hosts sinkholes to `0.0.0.0` inside marked block.
   - `VerifyIntegrity()`: Checks for unauthorized tampering.

---

## 4. Verification & Testing Strategy

1. **Unit & Integration Tests (`tests/integration/network_filter_test.go`)**:
   - Policy and rule creation, hierarchy compilation, duplicate prevention, and device state tracking.
2. **Automated Live E2E Suite (`scripts/e2e-network-filter.ps1`)**:
   - Server spin-up, RBAC enforcement (Viewer blocked with 403), policy compilation, WebSocket push (`filter.apply`), agent compliance callback, and audit verification.
3. **Cross-Compilation**:
   - Zero-CGO build verification on all 5 platforms.
