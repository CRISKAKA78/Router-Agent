param([string]$ReleaseDirectory='',[string]$OutputDirectory='')
$ErrorActionPreference='Stop'
$repoRoot=Split-Path $PSScriptRoot -Parent
if(!$ReleaseDirectory){$ReleaseDirectory=Join-Path $repoRoot 'build/windows-react/win-x64'}
if(!$OutputDirectory){$OutputDirectory=Join-Path $repoRoot ('build/react-shell/sandbox-'+(Get-Date -Format 'yyyyMMdd-HHmmss'))}
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
$release=[Security.SecurityElement]::Escape((Resolve-Path $ReleaseDirectory).Path)
$result=[Security.SecurityElement]::Escape((Resolve-Path $OutputDirectory).Path)
$tests=[Security.SecurityElement]::Escape((Join-Path $PSScriptRoot 'RouterWorkbench.Tests'))
$configuration=@"
<Configuration><Networking>Disable</Networking><VGpu>Disable</VGpu><ClipboardRedirection>Disable</ClipboardRedirection><MemoryInMB>4096</MemoryInMB><MappedFolders>
<MappedFolder><HostFolder>$release</HostFolder><SandboxFolder>C:\Release</SandboxFolder><ReadOnly>true</ReadOnly></MappedFolder>
<MappedFolder><HostFolder>$tests</HostFolder><SandboxFolder>C:\Tests</SandboxFolder><ReadOnly>true</ReadOnly></MappedFolder>
<MappedFolder><HostFolder>$result</HostFolder><SandboxFolder>C:\Results</SandboxFolder><ReadOnly>false</ReadOnly></MappedFolder>
</MappedFolders><LogonCommand><Command>powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Tests\sandbox-smoke.ps1</Command></LogonCommand></Configuration>
"@
$file=Join-Path $OutputDirectory 'verify.wsb';Set-Content $file $configuration -Encoding utf8
$sandbox=Start-Process WindowsSandbox.exe -ArgumentList ('"{0}"' -f $file) -WindowStyle Hidden -PassThru
try {
    for($i=0;$i -lt 180;$i++){
        if(Test-Path (Join-Path $OutputDirectory 'failure.txt')){throw (Get-Content (Join-Path $OutputDirectory 'failure.txt') -Raw)}
        if(Test-Path (Join-Path $OutputDirectory 'result.txt')){Get-Content (Join-Path $OutputDirectory 'result.txt');return}
        Start-Sleep -Seconds 2
    }
    throw "Sandbox verification timed out; inspect $OutputDirectory"
} finally {$sandbox.Dispose()}
