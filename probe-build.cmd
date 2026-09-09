@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0probe-build.ps1" %*
set "result=%errorlevel%"
pause
exit /b %result%
