param([string]$Dotnet='dotnet')
$ErrorActionPreference='Stop'
$repoRoot=Split-Path $PSScriptRoot -Parent
$output=Join-Path $repoRoot ('build/react-shell/verification-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force $output | Out-Null
$server=Join-Path $repoRoot 'build/server/router-server-phase6.exe'
$fixtureDll=Join-Path $repoRoot 'windows/RouterWorkbench.Tests/bin/Release/net10.0-windows/RouterWorkbench.Tests.dll'
$fixture=Start-Process -FilePath (Get-Command $Dotnet).Source -ArgumentList ('"{0}" "{1}" "{2}"' -f $fixtureDll,$server,$output) -WindowStyle Hidden -RedirectStandardOutput (Join-Path $output 'native.log') -RedirectStandardError (Join-Path $output 'native-error.log') -PassThru
$ui=$null
try {
    for($i=0;$i -lt 100 -and !(Test-Path (Join-Path $output 'fixture-ready.txt'));$i++){if($fixture.HasExited){throw 'Fixture failed'};Start-Sleep -Milliseconds 100}
    if(!(Test-Path (Join-Path $output 'fixture-ready.txt'))){throw 'Fixture did not start'}
    $ui=Start-Process -FilePath (Join-Path $repoRoot 'build/react-shell/verification-app/RouterWorkbench.exe') -ArgumentList ('"{0}" "{1}"' -f $server,$output) -WindowStyle Hidden -PassThru
    Push-Location (Join-Path $repoRoot 'frontend')
    try {node tests/native.mjs $output $ui.Id; if($LASTEXITCODE -ne 0){throw "WebView2 tests failed: $output"}}
    finally {Pop-Location}
    if(!$ui.CloseMainWindow() -or !$ui.WaitForExit(15000) -or $ui.ExitCode -ne 0){throw 'Native app did not release and exit normally'}
    Get-Content (Join-Path $output 'native.log'),(Join-Path $output 'ui-result.txt')
    'PASS native WM_CLOSE and resource release'
} finally {
    if($ui -and !$ui.HasExited){$ui.Kill()}; if($ui){$ui.Dispose()}
    if(!$fixture.HasExited){try{Invoke-WebRequest http://127.0.0.1:18082/stop | Out-Null}catch{};if(!$fixture.WaitForExit(10000)){$fixture.Kill($true)}}
    $fixture.Dispose()
}
