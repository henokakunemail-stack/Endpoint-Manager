# scripts/e2e-asset-license.ps1
# Live E2E test for Fase 14: Asset & License Management
# Validates Hardware Asset Lifecycle, Financial Valuation & Warranty Tracking,
# Software License Contract Management, Automated Seat Reconciliation,
# Over-Allocation Detection, RBAC Enforcement, and Forensic Audit Logging.

$ErrorActionPreference = 'Stop'

$port = 18456
$base = "http://localhost:$port"
$dbPath = Join-Path $env:TEMP "em-e2e-asset.db"
$serverExe = Join-Path $env:TEMP "emserver-asset.exe"

# Clean slate
Remove-Item $dbPath -ErrorAction SilentlyContinue
Get-Process emserver-asset -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "=== FASE 14 E2E: ASSET & LICENSE MANAGEMENT ===" -ForegroundColor Cyan

# 1. Build server
Write-Host "1. Building server binary (CGO_ENABLED=0)..."
Push-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))
$env:CGO_ENABLED = '0'
go build -o $serverExe ./server/cmd/server
if ($LASTEXITCODE -ne 0) { throw "Server compilation failed" }
Pop-Location

# 2. Start server
$env:DB_PATH = $dbPath
$env:JWT_SECRET = "e2e-asset-license-secret-key-32chars-min-ok"
$env:HTTP_ADDR = ":$port"
$env:LOG_LEVEL = "info"
$env:ADMIN_PASSWORD = "admin_asset_password"

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
    # 3. Authenticate Admin and create technician and viewer
    Write-Host "`n2. Authenticating Admin and Creating Test Roles..."
    $adminLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "admin"; password = "admin_asset_password" } | ConvertTo-Json) -ContentType "application/json"
    $adminToken = $adminLogin.access_token
    $adminHeaders = @{ Authorization = "Bearer $adminToken" }
    Write-Host "  [PASS] Admin authenticated" -ForegroundColor Green

    $null = Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body (@{ username = "techasset"; password = "Password123!"; role = "technician"; display_name = "Tech Asset" } | ConvertTo-Json) -ContentType "application/json"
    $null = Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body (@{ username = "viewerasset"; password = "Password123!"; role = "viewer"; display_name = "Viewer Asset" } | ConvertTo-Json) -ContentType "application/json"

    $techLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "techasset"; password = "Password123!" } | ConvertTo-Json) -ContentType "application/json"
    $techHeaders = @{ Authorization = "Bearer $($techLogin.access_token)" }
    $viewerLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "viewerasset"; password = "Password123!" } | ConvertTo-Json) -ContentType "application/json"
    $viewerHeaders = @{ Authorization = "Bearer $($viewerLogin.access_token)" }
    Write-Host "  [PASS] Technician and Viewer accounts created and authenticated" -ForegroundColor Green

    # 4. Enroll 2 Test Devices
    Write-Host "`n3. Enrolling Test Devices..."
    $dev1TokenResp = Invoke-RestMethod -Uri "$base/api/devices/enroll-token" -Method POST -Headers $adminHeaders -Body (@{
        hostname = "LAPTOP-SALES-01"
        os_name = "windows"
        site = "Jakarta-HQ"
    } | ConvertTo-Json) -ContentType "application/json"
    $dev1Enroll = Invoke-RestMethod -Uri "$base/api/agent/enroll" -Method POST -Body (@{ enrollment_token = $dev1TokenResp.enrollment_token } | ConvertTo-Json) -ContentType "application/json"
    $dev1Id = $dev1Enroll.device_id

    $dev2TokenResp = Invoke-RestMethod -Uri "$base/api/devices/enroll-token" -Method POST -Headers $adminHeaders -Body (@{
        hostname = "DESKTOP-OPS-01"
        os_name = "windows"
        site = "Surabaya-Branch"
    } | ConvertTo-Json) -ContentType "application/json"
    $dev2Enroll = Invoke-RestMethod -Uri "$base/api/agent/enroll" -Method POST -Body (@{ enrollment_token = $dev2TokenResp.enrollment_token } | ConvertTo-Json) -ContentType "application/json"
    $dev2Id = $dev2Enroll.device_id
    Write-Host "  [PASS] Enrolled Device 1 ($dev1Id) and Device 2 ($dev2Id)" -ForegroundColor Green

    # 5. Test RBAC: Viewer must be blocked from creating assets
    Write-Host "`n4. Testing RBAC on Asset and License Creation..."
    $rbacBlocked = $false
    try {
        Invoke-RestMethod -Uri "$base/api/assets" -Method POST -Headers $viewerHeaders -Body (@{
            asset_tag = "AST-FORBIDDEN"
            model_name = "Unauthorized Model"
        } | ConvertTo-Json) -ContentType "application/json"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            $rbacBlocked = $true
        }
    }
    if (-not $rbacBlocked) {
        throw "Security violation: Viewer was able to create asset"
    }
    Write-Host "  [PASS] Viewer correctly blocked from asset creation (HTTP 403)" -ForegroundColor Green

    # 6. Technician Creates Hardware Assets
    Write-Host "`n5. Technician Creating Hardware Assets..."
    $nowUtc = (Get-Date).ToUniversalTime()
    $warrantyExpiringSoon = $nowUtc.AddDays(15).ToString("yyyy-MM-ddTHH:mm:ssZ")
    $warrantySafe = $nowUtc.AddYears(2).ToString("yyyy-MM-ddTHH:mm:ssZ")

    $asset1Resp = Invoke-RestMethod -Uri "$base/api/assets" -Method POST -Headers $techHeaders -Body (@{
        asset_tag = "AST-2026-001"
        device_id = $dev1Id
        model_name = "Dell Latitude 3420"
        serial_number = "SN-DELL-001"
        vendor = "Dell Technologies"
        site = "Jakarta-HQ"
        department = "Sales"
        assigned_user = "Budi Hartono"
        purchase_cost = 15000000.0
        warranty_expires_at = $warrantyExpiringSoon
        status = "in_use"
        notes = "Sales executive primary laptop"
    } | ConvertTo-Json) -ContentType "application/json"
    $asset1Id = $asset1Resp.id

    $asset2Resp = Invoke-RestMethod -Uri "$base/api/assets" -Method POST -Headers $techHeaders -Body (@{
        asset_tag = "AST-2026-002"
        device_id = $dev2Id
        model_name = "Lenovo ThinkCentre M70q"
        serial_number = "SN-LENOVO-002"
        vendor = "Lenovo Enterprise"
        site = "Surabaya-Branch"
        department = "Operations"
        assigned_user = "Dewi Lestari"
        purchase_cost = 12000000.0
        warranty_expires_at = $warrantySafe
        status = "in_use"
        notes = "Branch operations workstation"
    } | ConvertTo-Json) -ContentType "application/json"
    $asset2Id = $asset2Resp.id
    Write-Host "  [PASS] 2 Hardware Assets created: AST-2026-001 and AST-2026-002" -ForegroundColor Green

    # 7. Query Asset Financial Summary
    Write-Host "`n6. Verifying Asset Financial & Warranty Summary..."
    $summary = Invoke-RestMethod -Uri "$base/api/assets/summary" -Method GET -Headers $viewerHeaders
    if ($summary.total_assets -ne 2) {
        throw "Expected 2 total assets, got: $($summary.total_assets)"
    }
    if ($summary.active_assets -ne 2) {
        throw "Expected 2 active assets, got: $($summary.active_assets)"
    }
    if ($summary.total_valuation -ne 27000000.0) {
        throw "Expected valuation 27,000,000, got: $($summary.total_valuation)"
    }
    if ($summary.warranty_expiring_count -ne 1) {
        throw "Expected 1 warranty expiring soon, got: $($summary.warranty_expiring_count)"
    }
    Write-Host "  [PASS] Summary verified: Total Assets=2, Valuation=Rp 27,000,000, Warranty Expiring Soon=1" -ForegroundColor Green

    # 8. Technician Updates Asset Status
    Write-Host "`n7. Technician Updating Asset Lifecycle Status..."
    $updateResp = Invoke-RestMethod -Uri "$base/api/assets/$asset1Id" -Method PUT -Headers $techHeaders -Body (@{
        asset_tag = "AST-2026-001"
        device_id = $dev1Id
        model_name = "Dell Latitude 3420"
        serial_number = "SN-DELL-001"
        vendor = "Dell Technologies"
        site = "Jakarta-HQ"
        department = "Sales"
        assigned_user = "Budi Hartono"
        purchase_cost = 15000000.0
        warranty_expires_at = $warrantyExpiringSoon
        status = "in_repair"
        notes = "Keyboard replacement at authorized service center"
    } | ConvertTo-Json) -ContentType "application/json"
    if ($updateResp.status -ne "in_repair") {
        throw "Asset status was not updated to 'in_repair'"
    }
    Write-Host "  [PASS] Asset AST-2026-001 updated to status 'in_repair'" -ForegroundColor Green

    # 9. Admin Creates Software Licenses
    Write-Host "`n8. Admin Creating Software Licenses..."
    # License 1: 5 seats purchased
    $lic1Resp = Invoke-RestMethod -Uri "$base/api/licenses" -Method POST -Headers $adminHeaders -Body (@{
        software_name = "Endpoint Security Suite"
        publisher = "CyberDefense Inc"
        license_type = "per_device"
        total_seats = 5
        cost = 7500000.0
    } | ConvertTo-Json) -ContentType "application/json"
    $lic1Id = $lic1Resp.id

    # License 2: 1 seat purchased (will be over-allocated by fleet)
    $lic2Resp = Invoke-RestMethod -Uri "$base/api/licenses" -Method POST -Headers $adminHeaders -Body (@{
        software_name = "Specialized Engineering CAD"
        publisher = "DesignSoft Corp"
        license_type = "per_device"
        total_seats = 1
        cost = 35000000.0
    } | ConvertTo-Json) -ContentType "application/json"
    $lic2Id = $lic2Resp.id
    Write-Host "  [PASS] 2 Software Licenses created (Security Suite: 5 seats, CAD: 1 seat)" -ForegroundColor Green

    # 10. Allocate License
    Write-Host "`n9. Technician Allocating License to Device..."
    $allocResp = Invoke-RestMethod -Uri "$base/api/licenses/$lic1Id/allocate" -Method POST -Headers $techHeaders -Body (@{
        device_id = $dev1Id
    } | ConvertTo-Json) -ContentType "application/json"
    if ($allocResp.status -ne "allocated") {
        throw "Failed to allocate license"
    }
    Write-Host "  [PASS] License allocated to Device 1" -ForegroundColor Green

    # 11. Seed Device Inventory to Trigger License Reconciliation
    Write-Host "`n10. Simulating Agent Inventory Reporting Installed Software..."
    # In SQLite, insert inventory snapshot for both devices
    # Both devices have Endpoint Security Suite and Specialized Engineering CAD
    $agentWSScheme = "ws://localhost:$port/api/agent/connect"
    $wsAgent1 = New-Object System.Net.WebSockets.ClientWebSocket
    $wsAgent1.Options.SetRequestHeader("X-Device-Id", $dev1Id)
    $wsAgent1.Options.SetRequestHeader("X-Device-Secret", $dev1Enroll.device_secret)
    $ctSource = New-Object System.Threading.CancellationTokenSource
    $wsAgent1.ConnectAsync([uri]$agentWSScheme, $ctSource.Token).Wait(5000) | Out-Null

    # Send inventory frame with installed software
    $invPayload1 = @{
        type = "inventory"
        payload = @{
            hw = @{ cpu = @{ model = "i5-1135G7" }; ram_bytes = 17179869184 }
            os = @{ name = "windows"; version = "11.0" }
            software = @(
                @{ name = "Endpoint Security Suite 2026"; version = "2.0"; publisher = "CyberDefense Inc" },
                @{ name = "Specialized Engineering CAD v12"; version = "12.0"; publisher = "DesignSoft Corp" }
            )
        }
    } | ConvertTo-Json -Depth 5
    $invBytes1 = [System.Text.Encoding]::UTF8.GetBytes($invPayload1)
    $invSeg1 = New-Object System.ArraySegment[byte] ($invBytes1, 0, $invBytes1.Length)
    $wsAgent1.SendAsync($invSeg1, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $ctSource.Token).Wait(3000) | Out-Null

    # Connect device 2 and report same software
    $wsAgent2 = New-Object System.Net.WebSockets.ClientWebSocket
    $wsAgent2.Options.SetRequestHeader("X-Device-Id", $dev2Id)
    $wsAgent2.Options.SetRequestHeader("X-Device-Secret", $dev2Enroll.device_secret)
    $wsAgent2.ConnectAsync([uri]$agentWSScheme, $ctSource.Token).Wait(5000) | Out-Null

    $invPayload2 = @{
        type = "inventory"
        payload = @{
            hw = @{ cpu = @{ model = "i7-12700" }; ram_bytes = 34359738368 }
            os = @{ name = "windows"; version = "11.0" }
            software = @(
                @{ name = "Endpoint Security Suite 2026"; version = "2.0"; publisher = "CyberDefense Inc" },
                @{ name = "Specialized Engineering CAD v12"; version = "12.0"; publisher = "DesignSoft Corp" }
            )
        }
    } | ConvertTo-Json -Depth 5
    $invBytes2 = [System.Text.Encoding]::UTF8.GetBytes($invPayload2)
    $invSeg2 = New-Object System.ArraySegment[byte] ($invBytes2, 0, $invBytes2.Length)
    $wsAgent2.SendAsync($invSeg2, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $ctSource.Token).Wait(3000) | Out-Null
    Write-Host "  [PASS] Live inventory frames processed for both devices" -ForegroundColor Green
    Start-Sleep -Milliseconds 800

    # 12. Query License Compliance Audit Report
    Write-Host "`n11. Auditing Fleet Software License Compliance..."
    $complianceResp = Invoke-RestMethod -Uri "$base/api/licenses/compliance" -Method GET -Headers $viewerHeaders
    $compList = $complianceResp.compliance

    $secSuite = $compList | Where-Object { $_.software_name -eq "Endpoint Security Suite" }
    $cadSuite = $compList | Where-Object { $_.software_name -eq "Specialized Engineering CAD" }

    if (-not $secSuite -or -not $cadSuite) {
        throw "Missing compliance results for licenses"
    }

    if ($secSuite.status -ne "compliant") {
        throw "Expected Security Suite to be 'compliant', got: $($secSuite.status)"
    }
    if ($cadSuite.status -ne "over_allocated") {
        throw "Expected CAD Suite to be 'over_allocated' (1 seat vs 2 installations), got: $($cadSuite.status)"
    }

    Write-Host "  [PASS] Compliance audit verified: Security Suite (Seats=5, Installed=$($secSuite.installed_detected), Status=$($secSuite.status))" -ForegroundColor Green
    Write-Host "  [PASS] Compliance audit verified: Engineering CAD (Seats=1, Installed=$($cadSuite.installed_detected), Status=$($cadSuite.status)) [ALERT FLAGGED]" -ForegroundColor Yellow

    # 13. Verify Audit Trail
    Write-Host "`n12. Verifying Forensic Audit Logging..."
    $auditLogs = Invoke-RestMethod -Uri "$base/api/audit-logs" -Method GET -Headers $adminHeaders
    $assetCreateAudit = $auditLogs.logs | Where-Object { $_.action -eq "asset.create" }
    $assetUpdateAudit = $auditLogs.logs | Where-Object { $_.action -eq "asset.update" }
    $licenseCreateAudit = $auditLogs.logs | Where-Object { $_.action -eq "license.create" }
    $licenseAllocAudit = $auditLogs.logs | Where-Object { $_.action -eq "license.allocate" }

    if (-not $assetCreateAudit) { throw "Expected audit log for asset.create" }
    if (-not $assetUpdateAudit) { throw "Expected audit log for asset.update" }
    if (-not $licenseCreateAudit) { throw "Expected audit log for license.create" }
    if (-not $licenseAllocAudit) { throw "Expected audit log for license.allocate" }

    Write-Host "  [PASS] Audit logs verified for asset creation/update and license creation/allocation" -ForegroundColor Green

    # Clean up websockets
    $wsAgent1.Dispose()
    $wsAgent2.Dispose()

    Write-Host "`n========================================================" -ForegroundColor Green
    Write-Host "   ALL 12 FASE 14 CRITERIA PASSED LIVE E2E VERIFICATION  " -ForegroundColor Green
    Write-Host "========================================================" -ForegroundColor Green
} finally {
    Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    Remove-Item $dbPath -ErrorAction SilentlyContinue
    Remove-Item $serverExe -ErrorAction SilentlyContinue
}
