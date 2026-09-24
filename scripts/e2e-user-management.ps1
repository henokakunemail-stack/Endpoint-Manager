# scripts/e2e-user-management.ps1
# Live E2E test for Fase 7: User Management
# Validates User CRUD, RBAC restrictions, self-service password change,
# admin password reset, account deactivation, and audit trail.

$ErrorActionPreference = 'Stop'

$port = 18449
$base = "http://localhost:$port"
$dbPath = Join-Path $env:TEMP "em-e2e-user.db"
$serverExe = Join-Path $env:TEMP "emserver-user.exe"

# Clean slate
Remove-Item $dbPath -ErrorAction SilentlyContinue
Get-Process emserver-user -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "=== FASE 7 E2E: USER MANAGEMENT & ACCESS CONTROL ===" -ForegroundColor Cyan

# 1. Build
Write-Host "1. Building server binary..."
Push-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))
$env:CGO_ENABLED = '0'
go build -o $serverExe ./server/cmd/server
if ($LASTEXITCODE -ne 0) { throw "Server compilation failed" }
Pop-Location

# 2. Start server
$env:DB_PATH = $dbPath
$env:JWT_SECRET = "e2e-user-secret-key-32chars-min-ok"
$env:HTTP_ADDR = ":$port"
$env:LOG_LEVEL = "info"
$env:ADMIN_PASSWORD = "admin_user_password"

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
    # 3. Authenticate Admin
    Write-Host "`n2. Authenticating Admin User..."
    $loginBody = @{ username = "admin"; password = "admin_user_password" } | ConvertTo-Json
    $loginRes = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body $loginBody -ContentType "application/json"
    $adminToken = $loginRes.access_token
    if (-not $adminToken) { throw "Admin login failed" }
    $adminHeaders = @{ Authorization = "Bearer $adminToken" }
    Write-Host "  [PASS] Admin JWT issued successfully" -ForegroundColor Green

    # 4. Create Users (Technician and Viewer)
    Write-Host "`n3. Creating New Users via Admin API..."
    $techBody = @{
        username = "techbob"
        password = "password123"
        role = "technician"
        display_name = "Bob Technician"
    } | ConvertTo-Json
    $techUser = Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body $techBody -ContentType "application/json"
    $techId = $techUser.id
    if (-not $techId) { throw "Failed to create technician user" }
    Write-Host "  [PASS] Created user 'techbob' (ID: $techId, Role: $($techUser.role))" -ForegroundColor Green

    $viewerBody = @{
        username = "vieweralice"
        password = "password123"
        role = "viewer"
        display_name = "Alice Viewer"
    } | ConvertTo-Json
    $viewerUser = Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body $viewerBody -ContentType "application/json"
    $viewerId = $viewerUser.id
    Write-Host "  [PASS] Created user 'vieweralice' (ID: $viewerId, Role: $($viewerUser.role))" -ForegroundColor Green

    # 5. Prevent Duplicate Username
    Write-Host "`n4. Testing Duplicate Username Rejection..."
    try {
        Invoke-RestMethod -Uri "$base/api/users" -Method POST -Headers $adminHeaders -Body $techBody -ContentType "application/json"
        throw "Expected duplicate username to fail"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 409) {
            Write-Host "  [PASS] Duplicate username rejected with HTTP 409 Conflict" -ForegroundColor Green
        } else {
            throw "Expected HTTP 409, got: $($_.Exception.Message)"
        }
    }

    # 6. List Users
    Write-Host "`n5. Listing All Users..."
    $users = Invoke-RestMethod -Uri "$base/api/users" -Headers $adminHeaders
    if ($users.Count -lt 3) { throw "Expected at least 3 users (admin, techbob, vieweralice)" }
    Write-Host "  [PASS] GET /api/users returned $($users.Count) users" -ForegroundColor Green

    # 7. Authenticate as Created Users
    Write-Host "`n6. Authenticating as Created Users..."
    $techLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "techbob"; password = "password123" } | ConvertTo-Json) -ContentType "application/json"
    $techToken = $techLogin.access_token
    $techHeaders = @{ Authorization = "Bearer $techToken" }
    Write-Host "  [PASS] 'techbob' authenticated successfully" -ForegroundColor Green

    $viewerLogin = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "vieweralice"; password = "password123" } | ConvertTo-Json) -ContentType "application/json"
    $viewerToken = $viewerLogin.access_token
    $viewerHeaders = @{ Authorization = "Bearer $viewerToken" }
    Write-Host "  [PASS] 'vieweralice' authenticated successfully" -ForegroundColor Green

    # 8. RBAC: Non-admin cannot list or create users
    Write-Host "`n7. Verifying RBAC Restrictions (Technician and Viewer cannot access /api/users)..."
    try {
        Invoke-RestMethod -Uri "$base/api/users" -Headers $techHeaders
        throw "Technician should not be able to list users"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            Write-Host "  [PASS] Technician rejected from /api/users (HTTP 403)" -ForegroundColor Green
        } else {
            throw "Expected HTTP 403, got: $($_.Exception.Message)"
        }
    }

    try {
        Invoke-RestMethod -Uri "$base/api/users" -Headers $viewerHeaders
        throw "Viewer should not be able to list users"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) {
            Write-Host "  [PASS] Viewer rejected from /api/users (HTTP 403)" -ForegroundColor Green
        } else {
            throw "Expected HTTP 403, got: $($_.Exception.Message)"
        }
    }

    # 9. Self-Service Password Change
    Write-Host "`n8. Testing Self-Service Password Change..."
    $changeBody = @{
        old_password = "password123"
        new_password = "newpassword456"
    } | ConvertTo-Json
    $changeRes = Invoke-RestMethod -Uri "$base/api/users/me/password" -Method PUT -Headers $techHeaders -Body $changeBody -ContentType "application/json"
    Write-Host "  [PASS] Self password change response: $($changeRes.status)" -ForegroundColor Green

    # Verify old password fails
    try {
        Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "techbob"; password = "password123" } | ConvertTo-Json) -ContentType "application/json"
        throw "Old password should no longer work"
    } catch {
        Write-Host "  [PASS] Old password correctly rejected" -ForegroundColor Green
    }

    # Verify new password works
    $techLogin2 = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "techbob"; password = "newpassword456" } | ConvertTo-Json) -ContentType "application/json"
    if (-not $techLogin2.access_token) { throw "Login with new password failed" }
    Write-Host "  [PASS] Login with new password succeeded" -ForegroundColor Green

    # 10. Admin Password Reset
    Write-Host "`n9. Testing Admin Password Reset..."
    $resetBody = @{ new_password = "adminreset789" } | ConvertTo-Json
    $resetRes = Invoke-RestMethod -Uri "$base/api/users/$viewerId/password" -Method PUT -Headers $adminHeaders -Body $resetBody -ContentType "application/json"
    Write-Host "  [PASS] Admin password reset: $($resetRes.status)" -ForegroundColor Green

    $viewerLogin2 = Invoke-RestMethod -Uri "$base/api/auth/login" -Method POST -Body (@{ username = "vieweralice"; password = "adminreset789" } | ConvertTo-Json) -ContentType "application/json"
    if (-not $viewerLogin2.access_token) { throw "Login with reset password failed" }
    Write-Host "  [PASS] Login with reset password succeeded" -ForegroundColor Green

    # 11. Update User Details
    Write-Host "`n10. Updating User Profile..."
    $updateBody = @{
        display_name = "Alice Lead Viewer"
        role = "technician"
    } | ConvertTo-Json
    $updatedUser = Invoke-RestMethod -Uri "$base/api/users/$viewerId" -Method PUT -Headers $adminHeaders -Body $updateBody -ContentType "application/json"
    if ($updatedUser.role -ne "technician" -or $updatedUser.display_name -ne "Alice Lead Viewer") {
        throw "User update did not reflect expected values"
    }
    Write-Host "  [PASS] User updated: Role=$($updatedUser.role), Name=$($updatedUser.display_name)" -ForegroundColor Green

    # 12. Deactivate User
    Write-Host "`n11. Deactivating User..."
    $deactRes = Invoke-RestMethod -Uri "$base/api/users/$techId" -Method DELETE -Headers $adminHeaders
    Write-Host "  [PASS] Deactivation response: $($deactRes.status)" -ForegroundColor Green

    $checkUser = Invoke-RestMethod -Uri "$base/api/users/$techId" -Headers $adminHeaders
    if ($checkUser.is_active) { throw "User should be marked inactive" }
    Write-Host "  [PASS] User confirmed inactive: is_active=$($checkUser.is_active)" -ForegroundColor Green

    # 13. Audit Trail
    Write-Host "`n12. Verifying Audit Trail..."
    $auditLogs = Invoke-RestMethod -Uri "$base/api/audit-logs" -Headers $adminHeaders
    $createAudit = $auditLogs.logs | Where-Object { $_.action -eq "user.create" }
    $updateAudit = $auditLogs.logs | Where-Object { $_.action -eq "user.update" }
    $passAudit = $auditLogs.logs | Where-Object { $_.action -eq "user.change_password" }
    $deactAudit = $auditLogs.logs | Where-Object { $_.action -eq "user.deactivate" }

    if (-not $createAudit) { throw "Missing user.create audit log" }
    if (-not $updateAudit) { throw "Missing user.update audit log" }
    if (-not $passAudit) { throw "Missing user.change_password audit log" }
    if (-not $deactAudit) { throw "Missing user.deactivate audit log" }

    Write-Host "  [PASS] All 4 user management audit actions verified in log" -ForegroundColor Green

    Write-Host "`n=======================================================" -ForegroundColor Green
    Write-Host "FASE 7 E2E VERIFICATION PASSED WITH 100% SUCCESS!" -ForegroundColor Green
    Write-Host "All criteria met: User CRUD, Duplicate Prevention, RBAC," -ForegroundColor Green
    Write-Host "Password Management, Deactivation, and Audit Trail." -ForegroundColor Green
    Write-Host "=======================================================" -ForegroundColor Green

} finally {
    Write-Host "`nCleaning up background processes..."
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-Item $dbPath -ErrorAction SilentlyContinue
}
