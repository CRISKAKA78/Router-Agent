[CmdletBinding()]
param([switch]$BuildOnly)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

try {
    $projectRoot = [IO.Path]::GetFullPath($PSScriptRoot)
    $buildRoot = Join-Path $projectRoot 'build'
    $outputDirectory = [IO.Path]::GetFullPath((Join-Path $buildRoot 'server-windows'))
    $repositoryDirectory = Join-Path $projectRoot 'data\server\repository'
    $serverExecutable = Join-Path $outputDirectory 'router-server.exe'
    $goCommand = (Get-Command go -CommandType Application -ErrorAction Stop).Source
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot 'go.mod') -PathType Leaf)) {
        throw 'Run this script from its original repository location.'
    }

    # Only this dedicated output tree may be removed. Never follow junctions.
    if ($outputDirectory -ne ($projectRoot.TrimEnd('\') + '\build\server-windows')) {
        throw 'Unexpected build output path; refusing cleanup.'
    }
    foreach ($pathToCheck in @($projectRoot, $buildRoot, $outputDirectory)) {
        if (Test-Path -LiteralPath $pathToCheck) {
            if ((Get-Item -LiteralPath $pathToCheck -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) {
                throw "Refusing cleanup through a reparse point: $pathToCheck"
            }
        }
    }
    if (Test-Path -LiteralPath $outputDirectory) {
        $linkedItem = Get-ChildItem -LiteralPath $outputDirectory -Recurse -Force |
            Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint } |
            Select-Object -First 1
        if ($null -ne $linkedItem) { throw "Remove the build-directory link manually first: $($linkedItem.FullName)" }
        Write-Host "Removing previous build: $outputDirectory"
        Remove-Item -LiteralPath $outputDirectory -Recurse -Force
    }
    New-Item -ItemType Directory -Path $outputDirectory | Out-Null

    $savedEnvironment = @{}
    foreach ($variableName in @('GOOS', 'GOARCH', 'CGO_ENABLED', 'GOCACHE')) {
        $savedEnvironment[$variableName] = [Environment]::GetEnvironmentVariable($variableName, 'Process')
    }
    Push-Location $projectRoot
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        $env:GOCACHE = Join-Path $outputDirectory 'go-cache'
        Write-Host 'Building Windows x64 Server from source (including dependencies)...'
        & $goCommand build -a -trimpath -o $serverExecutable ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE. Server was not started." }
    }
    finally {
        Pop-Location
        foreach ($variableName in $savedEnvironment.Keys) {
            [Environment]::SetEnvironmentVariable($variableName, $savedEnvironment[$variableName], 'Process')
        }
    }
    if ($BuildOnly) {
        Write-Host "Built: $serverExecutable"
        exit 0
    }

    $serverArguments = @(
        '-listen', ':9000',
        '-http-listen', ':8888',
        '-repository-dir', $repositoryDirectory,
        '-tunnel-bind', '::',
        '-tunnel-host', '47.119.168.150',
        '-tunnel-data-listen', ':9001',
        '-tunnel-data-host', '47.119.168.150',
        '-tunnel-port-first', '20000',
        '-tunnel-port-last', '20199'
    )
    Write-Host "Persistent data: $repositoryDirectory"
    Write-Host 'Client API: http://47.119.168.150:8888'
    Write-Host 'Probe: 47.119.168.150:9000; press Ctrl+C to stop before rebuilding.'
    & $serverExecutable @serverArguments
    exit $LASTEXITCODE
}
catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host 'If the old Server is running, stop it with Ctrl+C before rebuilding.'
    exit 1
}
