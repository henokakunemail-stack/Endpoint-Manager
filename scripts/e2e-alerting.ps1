# scripts/e2e-alerting.ps1
# Live E2E test for Fase 9: Alerting & Notification Engine
# Validates Rule Management, RBAC, On-Demand Evaluation, Deduplication,
# Incident Lifecycle (Open -> Acknowledged -> Resolved), and Audit Logging.

$ErrorActionPreference = 'Stop'

$port = 18451
$base = "http://localhost:$port"
$dbPath = Join-Path $env:TEMP "em-e2e-alerting.db"
$serverExe = Join-Path $env:TEMP "emserver-alerting.exe"

# Clean slate
Remove-Item $dbPath -ErrorAction SilentlyContinue
Get-Process emserver-alerting -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "=== FASE 9 E2E: ALERTING & NOTIFICATION ENGINE ===" -ForegroundColor Cyan

# 1. Build
Write-Host "1. Building server binary..."
Push-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))
$env:CGO_ENABLED = '0'
go build -o $serverExe ./server/cmd/server
if ($LASTEXITCODE -ne 0) { throw "Server compilation failed" }
Pop-Location

# 2. Start server
$env:DB_PATH = $dbPath
$env:JWT_SECRET = "e2e-alerting-secret-key-32chars-min-ok"
$env:HTTP_ADDR = ":$port"
$env:LOG_LEVEL = "info"
$env:ADMIN_PASSWORD = "admin_alert_password"

$serverProc = Start-Process $serverExe -PassThru -WindowStyle Hidden
Write-Host "Server started with PID: $($serverProc.Id) on port $port"

$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    try {
        $res = Invoke-RestMethod "$base/healthz" -TimeoutSec 2
        if ($res.status -eq "ok") { $ready = $true; break }
    } catch {
        Start-Sleep -Milliseconds 250
    }
}
if (-not $ready) {
    Stop-Process -Id $serverProc.Id -Force
    throw "Server failed to respond on /healthz"
}
Write-Host "Server is HEALTHY and listening." -ForegroundColor Green

try {
    # 3. Authenticate Admin and create technician and viewer accounts
    Write-Host "`n2. Authenticating Admin and Creating Test Roles..."
    $adminLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "admin"; password = "admin_alert_password" } | ConvertTo-Json) -ContentType "application/json"
    $adminToken = $adminLogin.access_token
    $adminHeaders = @{ Authorization = "Bearer $adminToken" }
    Write-Host "  [PASS] Admin authenticated" -ForegroundColor Green

    # Create technician & viewer via Admin User Management API
    $techBody = @{ username = "techalert"; password = "Password123!"; role = "technician"; display_name = "Tech Operator" } | ConvertTo-Json
    $null = Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body $techBody -ContentType "application/json"
    $viewerBody = @{ username = "vieweralert"; password = "Password123!"; role = "viewer"; display_name = "Viewer Monitor" } | ConvertTo-Json
    $null = Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body $viewerBody -ContentType "application/json"

    # Authenticate technician & viewer
    $techLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "techalert"; password = "Password123!" } | ConvertTo-Json) -ContentType "application/json"
    $techHeaders = @{ Authorization = "Bearer $($techLogin.access_token)" }
    $viewerLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "vieweralert"; password = "Password123!" } | ConvertTo-Json) -ContentType "application/json"
    $viewerHeaders = @{ Authorization = "Bearer $($viewerLogin.access_token)" }
    Write-Host "  [PASS] Technician and Viewer authenticated" -ForegroundColor Green

    # 4. Test RBAC on Rule Creation
    Write-Host "`n3. Testing RBAC on Alert Rule Management..."
    $rulePayload = @{
        name = "Critical Low Disk"
        rule_type = "disk_low"
        threshold_val = 15.0
        severity = "critical"
        webhook_url = "http://localhost:9999/webhook"
        is_enabled = $true
    } | ConvertTo-Json

    try {
        Invoke-RestMethod -Uri "$base/api/alerts/rules" -Method POST -Headers $viewerHeaders -Body $rulePayload -ContentType "application/json"
        throw "Viewer unexpectedly allowed to create alert rule"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            Write-Host "  [PASS] Viewer correctly REJECTED from creating rule (HTTP 403 Forbidden)" -ForegroundColor Green
        } else {
            throw "Expected HTTP 403, got: $($_.Exception.Message)"
        }
    }

    try {
        Invoke-RestMethod -Uri "$base/api/alerts/rules" -Method POST -Headers $techHeaders -Body $rulePayload -ContentType "application/json"
        throw "Technician unexpectedly allowed to create alert rule"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            Write-Host "  [PASS] Technician correctly REJECTED from creating rule (HTTP 403 Forbidden)" -ForegroundColor Green
        } else {
            throw "Expected HTTP 403, got: $($_.Exception.Message)"
        }
    }

    # Admin creates rule
    $createdRule = Invoke-RestMethod -Uri "$base/api/alerts/rules" -Method POST -Headers $adminHeaders -Body $rulePayload -ContentType "application/json"
    $ruleId = $createdRule.id
    if (-not $ruleId) { throw "Failed to create rule" }
    Write-Host "  [PASS] Admin successfully created alert rule '$($createdRule.name)' (ID: $ruleId)" -ForegroundColor Green

    # Viewer can list rules
    $rulesList = Invoke-RestMethod -Uri "$base/api/alerts/rules" -Headers $viewerHeaders
    if ($rulesList.Count -lt 1) { throw "Viewer failed to list rules" }
    Write-Host "  [PASS] Viewer successfully listed $($rulesList.Count) alert rule(s)" -ForegroundColor Green

    # 5. Seed Device Telemetry for Alert Evaluation
    Write-Host "`n4. Seeding Telemetry Data for Evaluation..."
    Push-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))
    $seedCode = @"
package main
import (
    "time"
    "github.com/jmoiron/sqlx"
    _ "modernc.org/sqlite"
)
func main() {
    d, err := sqlx.Open("sqlite", "$($dbPath.Replace('\', '/'))")
    if err != nil { panic(err) }
    defer d.Close()
    now := time.Now().UTC()

    // Low disk device (7.2% free -> triggers disk_low < 15.0)
    _, _ = d.Exec("INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at) VALUES ('d-low', 'SRV-FILE-01', 'windows', '10.0', '1.0.0', 'Bandung', 'online', ?, ?, 'h1', ?, ?)", now, now, now, now)
    _, _ = d.Exec("INSERT INTO device_inventory (id, device_id, hw, software, os_detail, hw_ram_bytes, hw_disk_free_pct, hw_cpu_model, collected_at, updated_at) VALUES ('inv-low', 'd-low', '{}', '[]', '{}', 17179869184, 7.2, 'Intel Xeon', ?, ?)", now, now)

    // Healthy device (52.0% free -> should NOT trigger)
    _, _ = d.Exec("INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at) VALUES ('d-ok', 'PC-DESK-02', 'windows', '11.0', '1.0.0', 'Jakarta', 'online', ?, ?, 'h2', ?, ?)", now, now, now, now)
    _, _ = d.Exec("INSERT INTO device_inventory (id, device_id, hw, software, os_detail, hw_ram_bytes, hw_disk_free_pct, hw_cpu_model, collected_at, updated_at) VALUES ('inv-ok', 'd-ok', '{}', '[]', '{}', 17179869184, 52.0, 'Intel i5', ?, ?)", now, now)
}
"@
    $helperFile = Join-Path $env:TEMP "seed_alert_telemetry.go"
    Set-Content -Path $helperFile -Value $seedCode -Encoding UTF8
    go run $helperFile
    Remove-Item $helperFile -ErrorAction SilentlyContinue
    Pop-Location
    Write-Host "  [PASS] Telemetry seeded: 1 low-disk device, 1 healthy device" -ForegroundColor Green

    # 6. Trigger On-Demand Evaluation
    Write-Host "`n5. Triggering On-Demand Evaluation Cycle 1..."
    $evalRes = Invoke-RestMethod -Uri "$base/api/alerts/evaluate" -Method POST -Headers $techHeaders
    if ($evalRes.incidents_new -ne 1) {
        throw "Expected 1 new incident, got: $($evalRes.incidents_new)"
    }
    Write-Host "  [PASS] Evaluation Cycle 1 executed: $($evalRes.incidents_new) new incident generated" -ForegroundColor Green

    # 7. Verify Incident Data
    Write-Host "`n6. Verifying Generated Incident..."
    $incidents = Invoke-RestMethod -Uri "$base/api/alerts/incidents?status=open" -Headers $viewerHeaders
    if ($incidents.Count -ne 1) { throw "Expected 1 open incident" }
    $incident = $incidents[0]
    if ($incident.device_id -ne "d-low" -or $incident.hostname -ne "SRV-FILE-01") {
        throw "Unexpected incident device: $($incident.hostname)"
    }
    if ($incident.trigger_count -ne 1) { throw "Expected trigger_count = 1" }
    $incidentId = $incident.id
    Write-Host "  [PASS] Incident created: '$($incident.title)' (ID: $incidentId, Triggers: $($incident.trigger_count))" -ForegroundColor Green

    # 8. Test Deduplication
    Write-Host "`n7. Testing Incident Deduplication (Evaluation Cycle 2)..."
    $evalRes2 = Invoke-RestMethod -Uri "$base/api/alerts/evaluate" -Method POST -Headers $techHeaders
    if ($evalRes2.incidents_new -ne 0 -or $evalRes2.incidents_dedup -ne 1) {
        throw "Expected 0 new and 1 dedup, got new=$($evalRes2.incidents_new), dedup=$($evalRes2.incidents_dedup)"
    }
    $incidentAfter = Invoke-RestMethod -Uri "$base/api/alerts/incidents/$incidentId" -Headers $viewerHeaders
    if ($incidentAfter.trigger_count -ne 2) {
        throw "Expected trigger_count = 2, got: $($incidentAfter.trigger_count)"
    }
    Write-Host "  [PASS] Deduplication verified: trigger_count incremented to 2 without creating duplicate incident rows" -ForegroundColor Green

    # 9. Test Incident Lifecycle: Acknowledge & Resolve
    Write-Host "`n8. Testing Incident Lifecycle (Acknowledge & Resolve)..."
    # Viewer cannot acknowledge
    try {
        Invoke-RestMethod -Uri "$base/api/alerts/incidents/$incidentId/acknowledge" -Method POST -Headers $viewerHeaders
        throw "Viewer unexpectedly allowed to acknowledge incident"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            Write-Host "  [PASS] Viewer correctly REJECTED from acknowledging incident (HTTP 403 Forbidden)" -ForegroundColor Green
        } else {
            throw "Expected HTTP 403, got: $($_.Exception.Message)"
        }
    }

    # Technician acknowledges
    $ackRes = Invoke-RestMethod -Uri "$base/api/alerts/incidents/$incidentId/acknowledge" -Method POST -Headers $techHeaders
    if ($ackRes.status -ne "acknowledged") { throw "Failed to acknowledge incident" }
    Write-Host "  [PASS] Technician successfully acknowledged incident" -ForegroundColor Green

    # Technician resolves
    $resRes = Invoke-RestMethod -Uri "$base/api/alerts/incidents/$incidentId/resolve" -Method POST -Headers $techHeaders
    if ($resRes.status -ne "resolved") { throw "Failed to resolve incident" }
    Write-Host "  [PASS] Technician successfully resolved incident" -ForegroundColor Green

    # Verify open list is now empty
    $openAfter = Invoke-RestMethod -Uri "$base/api/alerts/incidents?status=open" -Headers $viewerHeaders
    if ($openAfter.Count -ne 0) { throw "Expected 0 open incidents after resolution" }
    Write-Host "  [PASS] Open incident count is now 0" -ForegroundColor Green

    # 10. Verify Audit Log
    Write-Host "`n9. Verifying Audit Trail..."
    $auditLogs = Invoke-RestMethod -Uri "$base/api/reports/audit?limit=20" -Headers $adminHeaders
    $hasRuleCreate = $false
    $hasAlertAck = $false
    $hasAlertResolve = $false
    foreach ($log in $auditLogs) {
        if ($log.action -eq "alert_rule.create") { $hasRuleCreate = $true }
        if ($log.action -eq "alert.acknowledge") { $hasAlertAck = $true }
        if ($log.action -eq "alert.resolve") { $hasAlertResolve = $true }
    }
    if (-not $hasRuleCreate -or -not $hasAlertAck -or -not $hasAlertResolve) {
        throw "Missing audit trail records (create=$hasRuleCreate, ack=$hasAlertAck, resolve=$hasAlertResolve)"
    }
    Write-Host "  [PASS] All alerting actions (rule.create, alert.acknowledge, alert.resolve) recorded in audit logs" -ForegroundColor Green

    Write-Host "`n=======================================================" -ForegroundColor Green
    Write-Host "FASE 9 E2E VERIFICATION PASSED WITH 100% SUCCESS!" -ForegroundColor Green
    Write-Host "All criteria met: Alert Rules CRUD, RBAC Enforcement," -ForegroundColor Green
    Write-Host "Threshold Evaluation, Deduplication Engine," -ForegroundColor Green
    Write-Host "Incident Lifecycle (Open->Ack->Resolve), and Audit Trail." -ForegroundColor Green
    Write-Host "=======================================================" -ForegroundColor Green

} finally {
    Write-Host "`nCleaning up background processes..."
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-Item $dbPath -ErrorAction SilentlyContinue
}
