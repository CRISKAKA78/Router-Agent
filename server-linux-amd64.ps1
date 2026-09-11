[CmdletBinding()]
param([switch]$BuildOnly)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$savedEnvironment = @{}
try {
    $outputDirectory = Join-Path $PSScriptRoot 'build/server-linux-amd64'
    New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
    foreach ($variableName in @('GOOS', 'GOARCH', 'CGO_ENABLED')) {
        $savedEnvironment[$variableName] = [Environment]::GetEnvironmentVariable($variableName, 'Process')
    }
    Push-Location $PSScriptRoot
    try {
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        & go build -trimpath -o (Join-Path $outputDirectory 'router-server') ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw 'Linux AMD64 Server build failed.' }
    } finally { Pop-Location }
    $startScript = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'scripts/start-server-linux.sh')).Replace("`r`n", "`n")
    [IO.File]::WriteAllText((Join-Path $outputDirectory 'start.sh'), $startScript, (New-Object Text.UTF8Encoding($false)))
    Write-Host "Built: $outputDirectory"
    if (!$BuildOnly) {
        & (Join-Path $PSScriptRoot 'scripts/upload-server.ps1') -ArtifactDirectory $outputDirectory
    }
} catch {
    Write-Error $_
    exit 1
} finally {
    foreach ($variableName in $savedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($variableName, $savedEnvironment[$variableName], 'Process')
    }
}
