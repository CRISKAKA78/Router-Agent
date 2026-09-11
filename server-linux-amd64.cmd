@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0server-linux-amd64.ps1" %*
if errorlevel 1 pause
