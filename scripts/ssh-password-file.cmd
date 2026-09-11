@echo off
powershell.exe -NoLogo -NoProfile -NonInteractive -Command "try { $p=[IO.File]::ReadAllText($env:RMP_SSH_PASSWORD_FILE).TrimEnd([char[]]([char]13,[char]10)); if ([string]::IsNullOrEmpty($p) -or $p.Contains([char]13) -or $p.Contains([char]10)) { exit 1 }; [Console]::Write($p) } catch { exit 1 }"
