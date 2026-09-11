[CmdletBinding()]
param(
    [ValidatePattern('^[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+$')][string]$SshTarget = 'root@10.1.1.128',
    [ValidateRange(1,65535)][int]$Port = 22,
    [ValidatePattern('^/root/[A-Za-z0-9_-]+$')][string]$RemoteRoot = '/root/router-probe-gcc54',
    [string]$PasswordFile = '',
    [string]$ToolchainArchive = (Join-Path ([Environment]::GetFolderPath('Desktop')) 'gcc-5.4.tar.gz')
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$processes = New-Object System.Collections.Generic.List[System.Diagnostics.Process]
function Start-Ssh([string]$Command, [bool]$Capture = $false) {
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = $script:ssh
    $info.Arguments = "-T -p $Port -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=3 -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=`"$script:knownHosts`" -o PreferredAuthentications=password -o PubkeyAuthentication=no -o NumberOfPasswordPrompts=1 $SshTarget `"$Command`""
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $Capture
    # Password is read by askpass, never interpolated into a command or argument.
    $info.EnvironmentVariables['SSH_ASKPASS'] = Join-Path $PSScriptRoot 'scripts/ssh-password-file.cmd'
    $info.EnvironmentVariables['SSH_ASKPASS_REQUIRE'] = 'force'
    $info.EnvironmentVariables['DISPLAY'] = 'router-probe-build'
    $info.EnvironmentVariables['RMP_SSH_PASSWORD_FILE'] = $script:passwordPath
    $p = [System.Diagnostics.Process]::Start($info)
    $script:processes.Add($p)
    return $p
}
try {
    if (!$PasswordFile) { $PasswordFile = Join-Path $PSScriptRoot 'password.txt' }
    $script:ssh = (Get-Command ssh.exe -ErrorAction Stop).Source
    $tar = (Get-Command tar.exe -ErrorAction Stop).Source
    $script:passwordPath = (Resolve-Path -LiteralPath $PasswordFile -ErrorAction Stop).Path
    # Reject empty/multiline files before any SSH or upload; allow spaces and shell punctuation.
    $password = [IO.File]::ReadAllText($script:passwordPath).TrimEnd([char[]]([char]13,[char]10))
    if ([string]::IsNullOrEmpty($password) -or $password.Contains([char]13) -or $password.Contains([char]10)) { throw 'password.txt must contain one non-empty password line.' }
    $password = $null
    if (!(Test-Path -LiteralPath (Join-Path $PSScriptRoot 'probe/CMakeLists.txt'))) { throw 'Missing probe sources next to the script.' }
    $state = Join-Path $PSScriptRoot 'build/gcc54'
    New-Item -ItemType Directory -Force -Path $state | Out-Null
    $script:knownHosts = Join-Path $state 'known_hosts'
    $run = "$RemoteRoot/runs/$((Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,8))"
    Write-Host "Uploading current Probe sources to ${SshTarget}:$run"
    $remote = Start-Ssh "set -eu; umask 077; mkdir -p $RemoteRoot/runs; mkdir $run; mkdir $run/input; tar -xzf - -C $run/input; tr -d '\015' < $run/input/scripts/build-probe-gcc54.sh > $run/build-driver.sh"
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = $tar; $info.Arguments = '-czf - probe scripts/build-probe-gcc54.sh'
    $info.WorkingDirectory = $PSScriptRoot; $info.UseShellExecute = $false; $info.RedirectStandardOutput = $true; $info.CreateNoWindow = $true
    $pack = [System.Diagnostics.Process]::Start($info); $processes.Add($pack)
    try { $pack.StandardOutput.BaseStream.CopyTo($remote.StandardInput.BaseStream) } finally { $remote.StandardInput.Close() }
    $pack.WaitForExit(); $remote.WaitForExit()
    if ($pack.ExitCode -ne 0 -or $remote.ExitCode -ne 0) { throw 'Source upload failed; build was not started.' }
    $check = Start-Ssh "test -f $RemoteRoot/toolchain/gcc-5.4/.router-agent-installed" $true
    $check.StandardInput.Close(); $null = $check.StandardOutput.ReadToEnd(); $check.WaitForExit()
    if ($check.ExitCode -eq 1) {
        $archivePath = (Resolve-Path -LiteralPath $ToolchainArchive -ErrorAction Stop).Path
        Write-Host "Installing SDK from $archivePath (first successful installation only)"
        $upload = Start-Ssh "umask 077; cat > $run/gcc-5.4.tar.gz"
        $file = [IO.File]::OpenRead($archivePath)
        try { $file.CopyTo($upload.StandardInput.BaseStream) } finally { $file.Dispose(); $upload.StandardInput.Close() }
        $upload.WaitForExit(); if ($upload.ExitCode -ne 0) { throw 'SDK upload failed.' }
    } elseif ($check.ExitCode -ne 0) { throw 'Cannot check remote SDK; refusing to continue after SSH failure.' }
    $buildCommand = "bash -c 'set -euo pipefail; bash $run/build-driver.sh $run $RemoteRoot 2>&1 | tee $run/build.log'"
    $build = Start-Ssh $buildCommand
    $build.StandardInput.Close(); $build.WaitForExit()
    if ($build.ExitCode -ne 0) { throw "Build failed (exit $($build.ExitCode)); see ${SshTarget}:$run/build.log. Last successful output is preserved." }
    [IO.File]::WriteAllText((Join-Path $state 'latest-build.txt'), "$SshTarget`n$run`n", [Text.UTF8Encoding]::new($false))
    Write-Host "SUCCESS: ${SshTarget}:$RemoteRoot/output/router-probe" -ForegroundColor Green
    Write-Host "Build records: ${SshTarget}:$run"
    exit 0
} catch {
    Write-Host "Build failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    foreach ($p in $processes) { if (!$p.HasExited) { $p.Kill(); $p.WaitForExit() }; $p.Dispose() }
}
