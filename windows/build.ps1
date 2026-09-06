param([string]$Dotnet = 'dotnet', [switch]$Verify, [string]$OutputDirectory = '')
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
if (!$OutputDirectory) { $OutputDirectory = Join-Path $repoRoot 'build/windows-ui/win-x64' }
Push-Location $repoRoot
try {
    & $Dotnet build windows/RouterWorkbench -c Release
    if ($LASTEXITCODE -ne 0) { throw 'Windows UI build failed' }
    if ($Verify) {
        New-Item -ItemType Directory -Force build/server | Out-Null
        go build -o build/server/router-server-phase6.exe ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw 'Server build failed' }
        & $Dotnet run --project windows/RouterWorkbench.Tests -c Release -- build/server/router-server-phase6.exe build/phase6-tests
        if ($LASTEXITCODE -ne 0) { throw 'Windows UI verification failed' }
    }
    & $Dotnet publish windows/RouterWorkbench -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -p:DebugType=None -o $OutputDirectory
    if ($LASTEXITCODE -ne 0) { throw 'Windows UI publish failed' }
    Copy-Item -LiteralPath windows/README.md -Destination (Join-Path $OutputDirectory 'README.md')
    Write-Output "Windows UI: $OutputDirectory"
}
finally { Pop-Location }
