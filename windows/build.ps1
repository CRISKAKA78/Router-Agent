param([string]$Dotnet='dotnet',[switch]$Verify,[string]$OutputDirectory='',[string]$WebView2Runtime='')
$ErrorActionPreference='Stop'
$repoRoot=Split-Path $PSScriptRoot -Parent
if(!$OutputDirectory){$OutputDirectory=Join-Path $repoRoot 'build/windows-react/win-x64'}
Push-Location $repoRoot
try {
    npm --prefix frontend ci
    if($LASTEXITCODE -ne 0){throw 'Frontend dependency restore failed'}
    npm --prefix frontend run build
    if($LASTEXITCODE -ne 0){throw 'React production build failed'}
    if($Verify){npm --prefix frontend test; if($LASTEXITCODE -ne 0){throw 'Frontend tests failed'}}
    $runtimeCache=Join-Path $repoRoot 'build/react-shell/runtime-cache'
    New-Item -ItemType Directory -Force $runtimeCache | Out-Null
    if(!$WebView2Runtime){
        $WebView2Runtime=Join-Path $runtimeCache 'expanded/Microsoft.WebView2.FixedVersionRuntime.152.0.4191.62.x64'
        if(!(Test-Path (Join-Path $WebView2Runtime 'msedgewebview2.exe'))){
            $cab=Join-Path $runtimeCache 'WebView2.cab'
            if(!(Test-Path $cab)){Invoke-WebRequest 'https://msedge.sf.dl.delivery.mp.microsoft.com/filestreamingservice/files/0a4a34d9-ccaa-4cef-98b4-58cb313fbfeb/Microsoft.WebView2.FixedVersionRuntime.152.0.4191.62.x64.cab' -OutFile $cab}
            $expanded=Join-Path $runtimeCache 'expanded'; New-Item -ItemType Directory -Force $expanded | Out-Null
            expand.exe $cab '-F:*' $expanded | Out-Null
            if($LASTEXITCODE -ne 0){throw 'WebView2 extraction failed'}
        }
    }
    $runtimeBinary=Join-Path $WebView2Runtime 'msedgewebview2.exe'
    if(!(Test-Path $runtimeBinary) -or (Get-AuthenticodeSignature $runtimeBinary).Status -ne 'Valid'){throw 'A valid Microsoft Fixed Version WebView2 Runtime is required'}
    $vc=Join-Path $runtimeCache 'vc_redist.x64.exe'
    if(!(Test-Path $vc)){Invoke-WebRequest 'https://aka.ms/vs/17/release/vc_redist.x64.exe' -OutFile $vc}
    if((Get-AuthenticodeSignature $vc).Status -ne 'Valid'){throw 'Visual C++ runtime signature verification failed'}
    & $Dotnet publish windows/RouterWorkbench -c Release -r win-x64 --self-contained true -p:VerifyUI=false -p:PublishSingleFile=false -p:DebugType=None -o $OutputDirectory
    if($LASTEXITCODE -ne 0){throw 'Windows publish failed'}
    New-Item -ItemType Directory -Force (Join-Path $OutputDirectory 'WebView2Runtime') | Out-Null
    Copy-Item -Path (Join-Path $WebView2Runtime '*') -Destination (Join-Path $OutputDirectory 'WebView2Runtime') -Recurse -Force
    # The Microsoft Fixed Runtime includes its redistributable VC libraries.
    # App-local copies also satisfy WinUI without an elevated target-machine install.
    foreach($library in @('msvcp140.dll','msvcp140_codecvt_ids.dll','concrt140.dll','vccorlib140.dll','vcruntime140.dll','vcruntime140_1.dll')){
        $source=Join-Path $WebView2Runtime $library
        if(!(Test-Path $source)){throw "Missing bundled VC runtime: $library"}
        Copy-Item -LiteralPath $source -Destination $OutputDirectory -Force
    }
    New-Item -ItemType Directory -Force (Join-Path $OutputDirectory 'Prerequisites') | Out-Null
    Copy-Item -LiteralPath $vc -Destination (Join-Path $OutputDirectory 'Prerequisites/vc_redist.x64.exe') -Force
    Copy-Item windows/README.md,windows/Install-Prerequisites.ps1 -Destination $OutputDirectory -Force
    if($Verify){
        go build -o build/server/router-server-phase6.exe ./cmd/server
        if($LASTEXITCODE -ne 0){throw 'Server build failed'}
        & $Dotnet build windows/RouterWorkbench.Tests -c Release
        if($LASTEXITCODE -ne 0){throw 'Native tests build failed'}
        & $Dotnet build windows/RouterWorkbench -c Release -p:VerifyUI=true -o build/react-shell/verification-app
        if($LASTEXITCODE -ne 0){throw 'Native verification build failed'}
        & (Join-Path $PSScriptRoot 'verify-native.ps1') -Dotnet $Dotnet
        & (Join-Path $PSScriptRoot 'verify-published.ps1') -Executable (Join-Path $OutputDirectory 'RouterWorkbench.exe')
    }
    Write-Output "Windows release: $OutputDirectory"
}
finally {Pop-Location}
