@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion

:: ============================================
:: NetWeaverGo Build Script (Windows)
:: ============================================

echo.
echo ============================================
echo      NetWeaverGo Build Script
echo ============================================
echo.

:: Get project root directory
set "PROJECT_ROOT=%~dp0"
cd /d "%PROJECT_ROOT%"

:: Create dist directory if not exists
if not exist "dist" mkdir dist

:: ============================================
:: Step 1: Build Frontend
:: ============================================
echo [1/2] Building frontend...
echo.

cd /d "%PROJECT_ROOT%frontend"

:: Check if Node.js is installed
where node >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Node.js not found, please install Node.js first
    exit /b 1
)

:: Check if node_modules exists, install dependencies if not
if not exist "node_modules" (
    echo [INFO] First build, installing frontend dependencies...
    call npm install
    if %ERRORLEVEL% neq 0 (
        echo [ERROR] Frontend dependencies installation failed
        exit /b 1
    )
)

:: Build frontend
echo [INFO] Building frontend assets...
call npm run build
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Frontend build failed
    exit /b 1
)

echo [SUCCESS] Frontend build completed
echo.

:: ============================================
:: Step 2: Build Backend
:: ============================================
echo [2/2] Building backend...
echo.

cd /d "%PROJECT_ROOT%"

:: Check if Go is installed
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Go not found, please install Go first
    exit /b 1
)

:: Set Go build parameters
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1

:: Build backend, output to dist directory
echo [INFO] Building backend program...
go build -ldflags="-s -w" -o "dist\netWeaverGo.exe" ./cmd/netweaver
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Backend build failed
    exit /b 1
)

echo [SUCCESS] Backend build completed
echo.

:: ============================================
:: Done
:: ============================================
echo ============================================
echo            Build Successful!
echo ============================================
echo   Output: dist\netWeaverGo.exe
echo ============================================
echo.

:: Show file size
for %%A in ("dist\netWeaverGo.exe") do (
    set /a "SIZE_MB=%%~zA/1024/1024"
    echo File size: !SIZE_MB! MB
)

exit /b 0
