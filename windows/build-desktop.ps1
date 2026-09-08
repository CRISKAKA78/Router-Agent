[CmdletBinding()]
param(
    [switch]$BuildOnly,
    [switch]$Verify,
    [string]$Dotnet = '',
    [ValidatePattern('^windows-desktop(?:-[a-z0-9]+)?$')][string]$OutputName = 'windows-desktop'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
try {
    $desktopRepo = [IO.Path]::GetFullPath((Split-Path $PSScriptRoot -Parent))
    if (!$Dotnet) {
        $desktopSdk = Join-Path $desktopRepo 'build/dotnet10/dotnet.exe'
        $Dotnet = if (Test-Path -LiteralPath $desktopSdk) { $desktopSdk } else { (Get-Command dotnet -ErrorAction Stop).Source }
    }
    $sdks = & $Dotnet --list-sdks
    if ($LASTEXITCODE -ne 0 -or !($sdks -match '^10\.')) { throw 'The .NET 10 SDK is required. Pass -Dotnet with its executable path.' }
    $desktopOutput = Join-Path $desktopRepo "build/$OutputName"
    $desktopPublish = Join-Path $desktopOutput 'win-x64'
    $desktopExe = Join-Path $desktopPublish 'RouterWorkbench.exe'
    $running = Get-Process -Name RouterWorkbench -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $desktopExe }
    if ($running) { throw "The target application is running. Close it, or use -OutputName windows-desktop-next. No process has been stopped." }
    $project = Join-Path $PSScriptRoot 'RouterWorkbench.Desktop/RouterWorkbench.Desktop.csproj'
    # Publish only to the selected build directory. No recursive cleanup, repository data or old client outputs are touched.
    & $Dotnet publish $project -c Release -r win-x64 --self-contained true -o $desktopPublish --nologo -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -p:EnableCompressionInSingleFile=true
    if ($LASTEXITCODE -ne 0) { throw 'Native C# desktop publish failed.' }
    if ($Verify) {
        $verifyRoot = Join-Path $desktopOutput 'verification'
        New-Item -ItemType Directory -Force -Path $verifyRoot | Out-Null
        Push-Location $desktopRepo
        try {
            & go build -o (Join-Path $verifyRoot 'router-server.exe') ./cmd/server
            if ($LASTEXITCODE -ne 0) { throw 'Verification server build failed.' }
            & $Dotnet run --project (Join-Path $PSScriptRoot 'RouterWorkbench.Desktop.Tests/RouterWorkbench.Desktop.Tests.csproj') -c Release -- (Join-Path $verifyRoot 'router-server.exe') $verifyRoot
            if ($LASTEXITCODE -ne 0) { throw 'Desktop verification failed.' }
        } finally { Pop-Location }
    }
    Write-Host "Native C# desktop: $desktopExe"
    Write-Host 'Native WPF executable; no WebView2 or TerminalAssets required.'
    if (!$BuildOnly) { Start-Process -FilePath $desktopExe -WorkingDirectory $desktopPublish -WindowStyle Normal | Out-Null }
} catch {
    Write-Error $_
    exit 1
}
