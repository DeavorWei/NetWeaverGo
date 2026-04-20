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

:: ============================================
:: Kill running process to allow file overwrite
:: ============================================
echo [INFO] Checking for running netWeaverGo.exe process...
tasklist /FI "IMAGENAME eq netWeaverGo.exe" 2>nul | find /I "netWeaverGo.exe" >nul
if %ERRORLEVEL% equ 0 (
    echo [INFO] Found running netWeaverGo.exe, attempting to terminate...
    taskkill /F /IM "netWeaverGo.exe" >nul 2>&1
    if %ERRORLEVEL% equ 0 (
        echo [SUCCESS] Process terminated successfully
    ) else (
        echo [WARN] Failed to terminate process, continuing build...
    )
) else (
    echo [INFO] No running netWeaverGo.exe process found
)
echo.

:: Create dist directory if not exists
if not exist "dist" mkdir dist

:: ============================================
:: Step 0: Generate Windows Resource (Icon)
:: ============================================
echo [0/3] Generating Windows resource file for application icon...
echo.

cd /d "%PROJECT_ROOT%"

:: Check if rsrc.exe exists
if exist "rsrc.exe" (
    echo [INFO] Found rsrc.exe, generating resource file...
    rsrc.exe -ico "frontend\public\logo.ico" -o "cmd\netweaver\rsrc.syso"
    if %ERRORLEVEL% neq 0 (
        echo [WARN] Failed to generate resource file, continuing without custom icon...
    ) else (
        echo [SUCCESS] Resource file generated: cmd\netweaver\rsrc.syso
    )
) else (
    echo [WARN] rsrc.exe not found, skipping icon embedding...
)
echo.

:: ============================================
:: Step 1: Generate Wails Bindings
:: ============================================
echo [1/3] Generating Wails bindings...
echo.

cd /d "%PROJECT_ROOT%"

:: Check if wails3 command exists
where wails3 >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [WARN] wails3 command not found, skipping bindings generation...
    echo [WARN] Please install Wails3 or ensure wails3 is in PATH
) else (
    echo [INFO] Running wails3 generate bindings...
     wails3 generate bindings -b ./cmd/netweaver -d ./frontend/src/bindings -ts
    if %ERRORLEVEL% neq 0 (
        echo [ERROR] Failed to generate Wails bindings
        pause
        exit /b 1
    )
    echo [SUCCESS] Wails bindings generated to frontend/src/bindings
)
echo.

:: ============================================
:: Step 2: Build Frontend
:: ============================================
echo [2/3] Building frontend...
echo.

cd /d "%PROJECT_ROOT%frontend"

:: Check if Node.js is installed
where node >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Node.js not found, please install Node.js first
    pause
    exit /b 1
)

:: Check if node_modules exists, install dependencies if not
if not exist "node_modules" (
    echo [INFO] First build, installing frontend dependencies...
    call npm install
    if %ERRORLEVEL% neq 0 (
        echo [ERROR] Frontend dependencies installation failed
        pause
        exit /b 1
    )
)

:: Build frontend
echo [INFO] Building frontend assets...
call npm run build
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Frontend build failed
    pause
    exit /b 1
)

echo [SUCCESS] Frontend build completed
echo.

:: ============================================
:: Step 3: Build Backend
:: ============================================
echo [3/3] Building backend...
echo.

cd /d "%PROJECT_ROOT%"

:: Check if Go is installed
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Go not found, please install Go first
    pause
    exit /b 1
)

:: Set Go build parameters
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1

:: Clean build cache to ensure icon is re-embedded
echo [INFO] Cleaning Go build cache for icon refresh...
:: go clean -cache

:: Build backend, output to dist directory
echo [INFO] Building backend program...
go build  -o "dist\netWeaverGo.exe" ./cmd/netweaver
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Backend build failed
    pause
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

:: ============================================
:: Launch Application
:: ============================================
echo.
echo [INFO] Launching application...
cd /d "%PROJECT_ROOT%dist"
@REM start "" "netWeaverGo.exe"
echo [SUCCESS] Application started
echo.

exit /b 0
