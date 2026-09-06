$ErrorActionPreference='Stop'
$installer=Join-Path $PSScriptRoot 'Prerequisites/vc_redist.x64.exe'
if(!(Test-Path (Join-Path $PSScriptRoot 'vcruntime140.dll')) -and !(Test-Path (Join-Path $env:WINDIR 'System32/vcruntime140.dll'))){
    $install=Start-Process -FilePath $installer -ArgumentList '/install /quiet /norestart' -WindowStyle Hidden -PassThru -Wait
    if($install.ExitCode -notin 0,1638,3010){throw "Visual C++ runtime installation failed: $($install.ExitCode)"}
}
Write-Output 'Ready. Start RouterWorkbench.exe from this complete directory.'
