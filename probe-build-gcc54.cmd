@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0probe-build-gcc54.ps1" %*
set "result=%errorlevel%"
pause
exit /b %result%
