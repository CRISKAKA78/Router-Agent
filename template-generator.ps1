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
    $url = "http://127.0.0.1:$Port"
    $projectDirectory = Join-Path $projectRoot 'src/ProbeTemplateGenerator'
    # Keep running instances and build-only checks out of the default bin/obj tree.
    $outputName = if ($BuildOnly) { 'build-only' } else { "port-$Port" }
    $artifacts = Join-Path $projectRoot "build/template-generator/$outputName"
    $appDirectory = Join-Path $artifacts 'bin/ProbeTemplateGenerator/release'
    $appDll = Join-Path $appDirectory 'ProbeTemplateGenerator.dll'
    if (!$BuildOnly) {
        $listeners = @([Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().GetActiveTcpListeners() |
            Where-Object { $_.Port -eq $Port -and $_.Address.ToString() -in @('127.0.0.1', '0.0.0.0', '::') })
        if ($listeners.Count -gt 0) {
            $owners = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop |
                Where-Object { $_.LocalAddress -in @('127.0.0.1', '0.0.0.0', '::') } |
                Select-Object -ExpandProperty OwningProcess -Unique)
            $currentExe = Join-Path $appDirectory 'ProbeTemplateGenerator.exe'
            $matchingOwners = @($owners | Where-Object {
                $owner = Get-CimInstance Win32_Process -Filter "ProcessId = $_" -ErrorAction Stop
                $owner -and ($owner.ExecutablePath -eq $currentExe -or
                    ([IO.Path]::GetFileName($owner.ExecutablePath) -eq 'dotnet.exe' -and
                     $owner.CommandLine -match ('(?:^|\s)(?:"' + [regex]::Escape($appDll) + '"|' + [regex]::Escape($appDll) + ')(?:\s|$)')))
            })
            if ($matchingOwners.Count -ne $owners.Count -or $matchingOwners.Count -eq 0) {
                throw "Port $Port is occupied by another program. Use -Port with a free port. No process was stopped."
            }
            Write-Host "Generator is already running (PID $($matchingOwners -join ', ')): $url"
            Write-Host 'Reusing the current editor. To load source changes, stop its original window with Ctrl+C, then start again.'
            if (!$NoBrowser) { Start-Process $url }
            exit 0
        }
    }
    if (!$Dotnet) {
        $localSdk = Join-Path $projectRoot 'build/dotnet10/dotnet.exe'
        $Dotnet = if (Test-Path -LiteralPath $localSdk) { $localSdk } else { (Get-Command dotnet -ErrorAction Stop).Source }
    }
    $sdkList = & $Dotnet --list-sdks
    if ($LASTEXITCODE -ne 0 -or !($sdkList -match '^10\.')) { throw 'Install the .NET 10 SDK, or pass -Dotnet with its dotnet.exe path.' }
    $projectFile = Join-Path $projectRoot 'src/ProbeTemplateGenerator/ProbeTemplateGenerator.csproj'
    & $Dotnet build $projectFile -c Release --artifacts-path $artifacts --nologo
    if ($LASTEXITCODE -ne 0) { throw 'Generator build failed.' }
    if ($BuildOnly) { exit 0 }
    Write-Host "Probe Template Generator: $url"
    Write-Host 'Keep this window open while using the editor. Press Ctrl+C to stop.'
    # Readiness-gated browser opening is handled by the app after the listener starts.
    $env:PROBE_GENERATOR_OPEN_BROWSER = if ($NoBrowser) { '0' } else { '1' }
    & $Dotnet $appDll --contentRoot $projectDirectory --urls $url
    if ($LASTEXITCODE -ne 0) { throw 'Generator stopped with an error.' }
} catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    Remove-Item Env:PROBE_GENERATOR_OPEN_BROWSER -ErrorAction SilentlyContinue
}
