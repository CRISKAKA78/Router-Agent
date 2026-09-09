@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0template-generator.ps1" %*
set "result=%errorlevel%"
if not "%result%"=="0" pause
exit /b %result%
