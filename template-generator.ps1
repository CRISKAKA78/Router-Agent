[CmdletBinding()]
param(
    [switch]$BuildOnly,
    [switch]$NoBrowser,
    [int]$Port = 5188,
    [string]$Dotnet = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$projectRoot = [IO.Path]::GetFullPath($PSScriptRoot)
try {
    if ($Port -lt 1 -or $Port -gt 65535) { throw 'Port must be between 1 and 65535.' }
    if (!$Dotnet) {
        $localSdk = Join-Path $projectRoot 'build/dotnet10/dotnet.exe'
        $Dotnet = if (Test-Path -LiteralPath $localSdk) { $localSdk } else { (Get-Command dotnet -ErrorAction Stop).Source }
    }
    $sdkList = & $Dotnet --list-sdks
    if ($LASTEXITCODE -ne 0 -or !($sdkList -match '^10\.')) { throw 'Install the .NET 10 SDK, or pass -Dotnet with its dotnet.exe path.' }
    $projectFile = Join-Path $projectRoot 'src/ProbeTemplateGenerator/ProbeTemplateGenerator.csproj'
    & $Dotnet build $projectFile -c Release --nologo
    if ($LASTEXITCODE -ne 0) { throw 'Generator build failed.' }
    if ($BuildOnly) { exit 0 }
    $url = "http://127.0.0.1:$Port"
    Write-Host "Probe Template Generator: $url"
    Write-Host 'Keep this window open while using the editor. Press Ctrl+C to stop.'
    # Readiness-gated browser opening is handled by the app after the listener starts.
    if (!$NoBrowser) { $env:PROBE_GENERATOR_OPEN_BROWSER = '1' }
    & $Dotnet run --project $projectFile -c Release --no-build --no-launch-profile -- --urls $url
    if ($LASTEXITCODE -ne 0) { throw 'Generator stopped with an error.' }
} catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    Remove-Item Env:PROBE_GENERATOR_OPEN_BROWSER -ErrorAction SilentlyContinue
}
