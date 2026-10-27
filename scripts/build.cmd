@echo off
setlocal EnableExtensions
rem 在 Windows cmd 中构建 NMonitor。前端先编译并嵌入二进制。
rem 用法:
rem   scripts\build.cmd            当前架构的 Windows 程序
rem   scripts\build.cmd linux      交叉编译 Linux 程序
rem   scripts\build.cmd all        Windows 和 Linux 都构建
chcp 65001 >nul

where go >nul 2>&1 || (
  echo [build] 缺少命令: go
  exit /b 1
)
where npm >nul 2>&1 || (
  echo [build] 缺少命令: npm
  exit /b 1
)

set "TARGET=%~1"
if "%TARGET%"=="" set "TARGET=windows"

set "ARCH=amd64"
if /I "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "ARCH=arm64"
if /I "%PROCESSOR_ARCHITECTURE%"=="x86" set "ARCH=386"

pushd "%~dp0.." || exit /b 1

call :build_web
if errorlevel 1 goto :fail

if /I "%TARGET%"=="windows" goto :windows
if /I "%TARGET%"=="cmd" goto :windows
if /I "%TARGET%"=="linux" goto :linux
if /I "%TARGET%"=="all" goto :all
echo 用法: %~nx0 [windows^|linux^|all]
goto :fail

:windows
call :build_one windows %ARCH% "dist\windows-%ARCH%\nmonitor.exe"
if errorlevel 1 goto :fail
goto :done

:linux
call :build_one linux %ARCH% "dist\linux-%ARCH%\nmonitor"
if errorlevel 1 goto :fail
goto :done

:all
call :build_one windows %ARCH% "dist\windows-%ARCH%\nmonitor.exe"
if errorlevel 1 goto :fail
call :build_one linux %ARCH% "dist\linux-%ARCH%\nmonitor"
if errorlevel 1 goto :fail
goto :done

:build_web
echo [build] 构建前端
pushd web || exit /b 1
if exist package-lock.json (
  call npm ci
) else (
  call npm install
)
if errorlevel 1 (
  popd
  exit /b 1
)
call npm run build
if errorlevel 1 (
  popd
  exit /b 1
)
popd
exit /b 0

:build_one
echo [build] 构建 %~1/%~2 -^> %~3
if not exist "%~dp3" mkdir "%~dp3"
set "CGO_ENABLED=0"
set "GOOS=%~1"
set "GOARCH=%~2"
go build -trimpath -ldflags "-s -w" -o "%~3" .\cmd\server
if errorlevel 1 exit /b 1
copy /Y config.yaml "%~dp3config.yaml" >nul
if errorlevel 1 exit /b 1
echo [build] 完成 %~3
exit /b 0

:done
echo [build] 产物目录: %CD%\dist
popd
exit /b 0

:fail
echo [build] 构建失败
popd
exit /b 1
