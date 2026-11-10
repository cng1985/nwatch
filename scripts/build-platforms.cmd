@echo off
rem Windows CMD 构建入口：调用 build-platforms.ps1 并返回其退出码。
rem 用法：scripts\build-platforms.cmd [目标...]（默认 windows/amd64 linux/amd64 linux/arm64）

setlocal
where pwsh >nul 2>nul
if %errorlevel%==0 (
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-platforms.ps1" %*
    exit /b %errorlevel%
)
where powershell >nul 2>nul
if %errorlevel%==0 (
    powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-platforms.ps1" %*
    exit /b %errorlevel%
)
echo 未找到 PowerShell（pwsh 或 powershell），无法构建。 1>&2
exit /b 1
