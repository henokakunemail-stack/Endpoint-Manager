# scripts/e2e-reports.ps1
# Live E2E test for Fase 8: Reports & Export Engine
# Validates JSON and CSV exports for Device Inventory, Patch Compliance,
# Software Deployment History, and Audit Trail, plus RBAC enforcement.

$ErrorActionPreference = 'Stop'

$port = 18450
$base = "http://localhost:$port"
$dbPath = Join-Path $env:TEMP "em-e2e-reports.db"
$serverExe = Join-Path $env:TEMP "emserver-reports.exe"

# Clean slate
Remove-Item $dbPath -ErrorAction SilentlyContinue
Get-Process emserver-reports -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "=== FASE 8 E2E: REPORTS & EXPORT ENGINE ===" -ForegroundColor Cyan

# 1. Build
Write-Host "1. Building server binary..."
Push-Location "D:\Henok\Projects\Desktop Manage"
$env:CGO_ENABLED = '0'
go build -o $serverExe ./server/cmd/server
if ($LASTEXITCODE -ne 0) { throw "Server compilation failed" }
Pop-Location

# 2. Start server
$env:DB_PATH = $dbPath
$env:JWT_SECRET = "e2e-reports-secret-key-32chars-min-ok"
$env:HTTP_ADDR = ":$port"
$env:LOG_LEVEL = "info"
$env:ADMIN_PASSWORD = "admin_reports_password"

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
    # 3. Authenticate
    Write-Host "`n2. Authenticating Admin and Setting Up Users..."
    $loginBody = @{ username = "admin"; password = "admin_reports_password" } | ConvertTo-Json
    $loginRes = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body $loginBody -ContentType "application/json"
    $adminToken = $loginRes.access_token
    $adminHeaders = @{ Authorization = "Bearer $adminToken" }
    Write-Host "  [PASS] Admin JWT issued successfully" -ForegroundColor Green

    # Create viewer
    Push-Location "D:\Henok\Projects\Desktop Manage"
    $createUserCode = @"
package main
import (
    "time"
    "github.com/endpoint-mgmt/server/core/auth"
    "github.com/endpoint-mgmt/server/core/rbac"
    "github.com/jmoiron/sqlx"
    _ "modernc.org/sqlite"
)
func main() {
    d, err := sqlx.Open("sqlite", "$($dbPath.Replace('\', '/'))")
    if err != nil { panic(err) }
    defer d.Close()
    h1, _ := auth.HashPassword("viewer12345")
    now := time.Now().UTC()
    _, _ = d.Exec("INSERT INTO users (id, username, password_hash, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
        "viewer-rep-uuid", "viewerrep", h1, rbac.RoleViewer, now, now)

    // Seed mock data for reports
    _, _ = d.Exec("INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at) VALUES ('d1', 'PC-JAKARTA-01', 'windows', '11.0', '1.0.0', 'Jakarta', 'online', ?, ?, 'h1', ?, ?)", now, now, now, now)
    _, _ = d.Exec("INSERT INTO device_inventory (id, device_id, hw, software, os_detail, hw_ram_bytes, hw_disk_free_pct, hw_cpu_model, collected_at, updated_at) VALUES ('inv-1', 'd1', '{}', '[]', '{}', 17179869184, 62.4, 'Intel Core i5-1135G7', ?, ?)", now, now)
    _, _ = d.Exec("INSERT INTO device_patches (id, device_id, patch_id, title, severity, category, installed_state, reboot_required, discovered_at, updated_at) VALUES ('p1', 'd1', 'KB5034441', 'Security Update BitLocker', 'critical', 'security', 'missing', 1, ?, ?)", now, now)
    _, _ = d.Exec("INSERT INTO software_packages (id, name, version, os_target, package_type, file_name, file_size, sha256, storage_path, created_at, updated_at) VALUES ('pkg-1', '7-Zip', '23.01', 'windows', 'msi', '7z.msi', 1500000, 'hash256', '/tmp/7z.msi', ?, ?)", now, now)
    _, _ = d.Exec("INSERT INTO software_deployments (id, package_id, name, target_type, target_id, created_by, status, created_at) VALUES ('dep-1', 'pkg-1', 'Deploy 7-Zip Fleet', 'all', '', 'admin', 'completed', ?)", now)
    _, _ = d.Exec("INSERT INTO deployment_tasks (id, deployment_id, package_id, device_id, status, exit_code, created_at, updated_at, completed_at) VALUES ('task-1', 'dep-1', 'pkg-1', 'd1', 'success', 0, ?, ?, ?)", now, now, now)
}
"@
    $helperFile = Join-Path $env:TEMP "create_reports_mock.go"
    Set-Content -Path $helperFile -Value $createUserCode -Encoding UTF8
    go run $helperFile
    Remove-Item $helperFile -ErrorAction SilentlyContinue
    Pop-Location

    $viewerLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "viewerrep"; password = "viewer12345" } | ConvertTo-Json) -ContentType "application/json"
    $viewerToken = $viewerLogin.access_token
    $viewerHeaders = @{ Authorization = "Bearer $viewerToken" }
    Write-Host "  [PASS] Viewer authenticated" -ForegroundColor Green

    # 4. Test 1: Device Inventory JSON & CSV
    Write-Host "`n3. Testing Device Inventory Export (JSON & CSV)..."
    $invJson = Invoke-RestMethod -Uri "$base/api/reports/inventory" -Headers $viewerHeaders
    if ($invJson.Count -ne 1 -or $invJson[0].hostname -ne "PC-JAKARTA-01") {
        throw "Unexpected inventory JSON report"
    }
    Write-Host "  [PASS] GET /api/reports/inventory (JSON) returned 1 device" -ForegroundColor Green

    $invCsv = Invoke-WebRequest -Uri "$base/api/reports/inventory?format=csv" -Headers $viewerHeaders -UseBasicParsing
    if ($invCsv.StatusCode -ne 200 -or -not ($invCsv.Content -match "PC-JAKARTA-01") -or -not ($invCsv.Content -match "Intel Core i5-1135G7")) {
        throw "Unexpected inventory CSV export"
    }
    Write-Host "  [PASS] GET /api/reports/inventory?format=csv returned valid CSV with hardware headers" -ForegroundColor Green

    # 5. Test 2: Patch Compliance JSON & CSV
    Write-Host "`n4. Testing Patch Compliance Export (JSON & CSV)..."
    $patchJson = Invoke-RestMethod -Uri "$base/api/reports/patches" -Headers $viewerHeaders
    if ($patchJson.Count -ne 1 -or $patchJson[0].patch_id -ne "KB5034441") {
        throw "Unexpected patch compliance JSON report"
    }
    Write-Host "  [PASS] GET /api/reports/patches (JSON) returned 1 patch entry" -ForegroundColor Green

    $patchCsv = Invoke-WebRequest -Uri "$base/api/reports/patches?format=csv" -Headers $viewerHeaders -UseBasicParsing
    if ($patchCsv.StatusCode -ne 200 -or -not ($patchCsv.Content -match "KB5034441") -or -not ($patchCsv.Content -match "critical")) {
        throw "Unexpected patch compliance CSV export"
    }
    Write-Host "  [PASS] GET /api/reports/patches?format=csv returned valid CSV with patch headers" -ForegroundColor Green

    # 6. Test 3: Deployment History JSON & CSV
    Write-Host "`n5. Testing Software Deployment History Export (JSON & CSV)..."
    $depJson = Invoke-RestMethod -Uri "$base/api/reports/deployments" -Headers $viewerHeaders
    if ($depJson.Count -ne 1 -or $depJson[0].package_name -ne "7-Zip") {
        throw "Unexpected deployment history JSON report"
    }
    Write-Host "  [PASS] GET /api/reports/deployments (JSON) returned 1 task record" -ForegroundColor Green

    $depCsv = Invoke-WebRequest -Uri "$base/api/reports/deployments?format=csv" -Headers $viewerHeaders -UseBasicParsing
    if ($depCsv.StatusCode -ne 200 -or -not ($depCsv.Content -match "Deploy 7-Zip Fleet") -or -not ($depCsv.Content -match "success")) {
        throw "Unexpected deployment CSV export"
    }
    Write-Host "  [PASS] GET /api/reports/deployments?format=csv returned valid CSV with deployment headers" -ForegroundColor Green

    # 7. Test 4: Audit Trail Export & RBAC Protection
    Write-Host "`n6. Testing Forensic Audit Trail Export & RBAC..."
    try {
        Invoke-RestMethod -Uri "$base/api/reports/audit" -Headers $viewerHeaders
        throw "Viewer unexpectedly allowed to access audit report"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            Write-Host "  [PASS] Viewer correctly REJECTED from /api/reports/audit (HTTP 403 Forbidden)" -ForegroundColor Green
        } else {
            throw "Expected HTTP 403, got: $($_.Exception.Message)"
        }
    }

    $auditJson = Invoke-RestMethod -Uri "$base/api/reports/audit" -Headers $adminHeaders
    if ($auditJson.Count -lt 1) { throw "Expected at least 1 audit entry" }
    Write-Host "  [PASS] Admin GET /api/reports/audit (JSON) returned $($auditJson.Count) audit entries" -ForegroundColor Green

    $auditCsv = Invoke-WebRequest -Uri "$base/api/reports/audit?format=csv" -Headers $adminHeaders -UseBasicParsing
    if ($auditCsv.StatusCode -ne 200 -or -not ($auditCsv.Content -match "Actor Type") -or -not ($auditCsv.Content -match "Timestamp")) {
        throw "Unexpected audit CSV export"
    }
    Write-Host "  [PASS] Admin GET /api/reports/audit?format=csv returned valid CSV with forensic headers" -ForegroundColor Green

    Write-Host "`n=======================================================" -ForegroundColor Green
    Write-Host "FASE 8 E2E VERIFICATION PASSED WITH 100% SUCCESS!" -ForegroundColor Green
    Write-Host "All criteria met: Inventory Export, Patch Compliance Export," -ForegroundColor Green
    Write-Host "Deployment History Export, Audit Trail Export, CSV/JSON, RBAC." -ForegroundColor Green
    Write-Host "=======================================================" -ForegroundColor Green

} finally {
    Write-Host "`nCleaning up background processes..."
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-Item $dbPath -ErrorAction SilentlyContinue
}
