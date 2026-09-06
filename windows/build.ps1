param([string]$Dotnet = 'dotnet', [switch]$Verify, [string]$OutputDirectory = '')
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
if (!$OutputDirectory) { $OutputDirectory = Join-Path $repoRoot 'build/windows-winui/win-x64' }
Push-Location $repoRoot
try {
    & $Dotnet build windows/RouterWorkbench -c Release -p:VerifyUI=false
    if ($LASTEXITCODE -ne 0) { throw 'Windows UI build failed' }
    if ($Verify) {
        New-Item -ItemType Directory -Force build/server | Out-Null
        go build -o build/server/router-server-phase6.exe ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw 'Server build failed' }
        $verificationOutput = Join-Path $repoRoot ('build/windows-winui/verification-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
        New-Item -ItemType Directory -Force $verificationOutput | Out-Null
        & $Dotnet run --project windows/RouterWorkbench.Tests -c Release -- build/server/router-server-phase6.exe (Join-Path $verificationOutput 'core')
        if ($LASTEXITCODE -ne 0) { throw 'Windows UI verification failed' }
        & $Dotnet build windows/RouterWorkbench -c Release -p:VerifyUI=true -o build/windows-winui/verification-app
        if ($LASTEXITCODE -ne 0) { throw 'WinUI verification build failed' }
        $verificationExe = Join-Path $repoRoot 'build/windows-winui/verification-app/RouterWorkbench.exe'
        $serverExe = Join-Path $repoRoot 'build/server/router-server-phase6.exe'
        $verificationProcess = Start-Process -FilePath $verificationExe -ArgumentList ('"{0}" "{1}"' -f $serverExe, $verificationOutput) -WindowStyle Hidden -PassThru
        try {
            if (!$verificationProcess.WaitForExit(180000)) { $verificationProcess.Kill(); throw 'WinUI verification timed out' }
            if ($verificationProcess.ExitCode -ne 0 -or !(Test-Path -LiteralPath (Join-Path $verificationOutput 'ui-result.txt'))) { throw "WinUI verification failed: $verificationOutput" }
        }
        finally { $verificationProcess.Dispose() }
        Get-Content -LiteralPath (Join-Path $verificationOutput 'ui-result.txt')
    }
    & $Dotnet publish windows/RouterWorkbench -c Release -r win-x64 --self-contained true -p:VerifyUI=false -p:PublishSingleFile=false -p:DebugType=None -o $OutputDirectory
    if ($LASTEXITCODE -ne 0) { throw 'Windows UI publish failed' }
    Copy-Item -LiteralPath windows/README.md -Destination (Join-Path $OutputDirectory 'README.md')
    if ($Verify) { & (Join-Path $PSScriptRoot 'verify-published.ps1') -Executable (Join-Path $OutputDirectory 'RouterWorkbench.exe') }
    Write-Output "Windows UI: $OutputDirectory"
}
finally { Pop-Location }
