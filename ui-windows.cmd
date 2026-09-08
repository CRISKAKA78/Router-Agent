@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0windows\build-desktop.ps1" %*
set "result=%errorlevel%"
pause
exit /b %result%
