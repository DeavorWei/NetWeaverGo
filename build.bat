@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion

:: ============================================
:: NetWeaverGo 编译脚本 (Windows)
:: 编译前端 + 后端，输出到 dist/netWeaverGo.exe
:: ============================================

echo.
echo ╔════════════════════════════════════════════╗
echo ║     NetWeaverGo Build Script              ║
echo ╚════════════════════════════════════════════╝
echo.

:: 获取项目根目录
set "PROJECT_ROOT=%~dp0"
cd /d "%PROJECT_ROOT%"

:: 确保 dist 目录存在
if not exist "dist" mkdir dist

:: ============================================
:: 步骤 1: 编译前端
:: ============================================
echo [1/2] 编译前端...
echo.

cd /d "%PROJECT_ROOT%frontend"

:: 检查 node 是否安装
where node >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [错误] 未找到 Node.js，请先安装 Node.js
    exit /b 1
)

:: 检查 node_modules 是否存在，不存在则安装依赖
if not exist "node_modules" (
    echo [信息] 首次编译，正在安装前端依赖...
    call npm install
    if %ERRORLEVEL% neq 0 (
        echo [错误] 前端依赖安装失败
        exit /b 1
    )
)

:: 编译前端
echo [信息] 正在编译前端资源...
call npm run build
if %ERRORLEVEL% neq 0 (
    echo [错误] 前端编译失败
    exit /b 1
)

echo [成功] 前端编译完成
echo.

:: ============================================
:: 步骤 2: 编译后端
:: ============================================
echo [2/2] 编译后端...
echo.

cd /d "%PROJECT_ROOT%"

:: 检查 go 是否安装
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [错误] 未找到 Go，请先安装 Go
    exit /b 1
)

:: 设置 Go 编译参数
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1

:: 编译后端，输出到 dist 目录
echo [信息] 正在编译后端程序...
go build -ldflags="-s -w" -o "dist\netWeaverGo.exe" ./cmd/netweaver
if %ERRORLEVEL% neq 0 (
    echo [错误] 后端编译失败
    exit /b 1
)

echo [成功] 后端编译完成
echo.

:: ============================================
:: 完成
:: ============================================
echo ╔════════════════════════════════════════════╗
echo ║            编译成功！                      ║
echo ╠════════════════════════════════════════════╣
echo ║  输出文件: dist\netWeaverGo.exe           ║
echo ╚════════════════════════════════════════════╝
echo.

:: 显示文件大小
for %%A in ("dist\netWeaverGo.exe") do (
    set /a "SIZE_MB=%%~zA/1024/1024"
    echo 文件大小: !SIZE_MB! MB
)

exit /b 0
