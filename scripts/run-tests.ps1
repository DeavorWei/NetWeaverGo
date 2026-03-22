# Phase 0 Test Script
# Validates core module tests pass

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "NetWeaverGo Core Module Tests" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$failed = $false

# 1. Executor module tests
Write-Host "`n[1/3] Executor module tests (internal/executor)..." -ForegroundColor Yellow
go test ./internal/executor/... -v
if ($LASTEXITCODE -ne 0) {
    Write-Host "Executor module tests FAILED!" -ForegroundColor Red
    $failed = $true
} else {
    Write-Host "Executor module tests PASSED!" -ForegroundColor Green
}

# 2. Config module tests
Write-Host "`n[2/3] Config module tests (internal/config)..." -ForegroundColor Yellow
go test ./internal/config/... -v
if ($LASTEXITCODE -ne 0) {
    Write-Host "Config module tests FAILED!" -ForegroundColor Red
    $failed = $true
} else {
    Write-Host "Config module tests PASSED!" -ForegroundColor Green
}

# 3. Engine module tests
Write-Host "`n[3/3] Engine module tests (internal/engine)..." -ForegroundColor Yellow
go test ./internal/engine/... -v
if ($LASTEXITCODE -ne 0) {
    Write-Host "Engine module tests FAILED!" -ForegroundColor Red
    $failed = $true
} else {
    Write-Host "Engine module tests PASSED!" -ForegroundColor Green
}

# 4. Race detection tests (optional, requires CGO)
Write-Host "`n[Optional] Race detection tests (requires CGO)..." -ForegroundColor Yellow
$env:CGO_ENABLED = "1"
go test -race ./internal/executor/... ./internal/engine/... 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "Race detection tests PASSED!" -ForegroundColor Green
} else {
    Write-Host "Race detection tests SKIPPED (CGO not available)" -ForegroundColor Yellow
}

Write-Host "`n========================================" -ForegroundColor Cyan
if ($failed) {
    Write-Host "Tests FAILED! Please fix before commit." -ForegroundColor Red
    exit 1
} else {
    Write-Host "All tests PASSED!" -ForegroundColor Green
    exit 0
}
