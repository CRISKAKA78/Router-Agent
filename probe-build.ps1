param(
    [ValidatePattern('^[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+$')]
    [string]$SshTarget = 'root@10.1.1.128',
    [ValidatePattern('^/root/[A-Za-z0-9_-]+$')]
    [string]$RemoteRoot = '/root/codex-probe-20260908-2123',
    [ValidatePattern('^/root/[A-Za-z0-9_.-]+$')]
    [string]$Toolchain = '/root/gcc-5.2',
    [ValidatePattern('^[A-Za-z0-9_,.-]*$')]
    [string]$NetworkInterfaces = ''
)

$ErrorActionPreference = 'Stop'
$archiveProcess = $null
$sshProcess = $null
try {
    if (!$PSBoundParameters.ContainsKey('NetworkInterfaces')) { $NetworkInterfaces = Read-Host 'Network interfaces (e.g. eth0,br0; Enter = all)' }
    if ($NetworkInterfaces -and ($NetworkInterfaces -notmatch '^[A-Za-z0-9_.-]{1,15}(,[A-Za-z0-9_.-]{1,15})*$' -or @($NetworkInterfaces.Split(',')).Count -gt 32 -or @($NetworkInterfaces.Split(',') | Select-Object -Unique).Count -ne @($NetworkInterfaces.Split(',')).Count -or @($NetworkInterfaces.Split(',') | Where-Object { $_ -in '.', '..' }).Count)) { throw 'Invalid interface names.' }
    $networkArgument = if ($NetworkInterfaces) { $NetworkInterfaces } else { '-' }
    $ssh = (Get-Command ssh.exe -ErrorAction Stop).Source
    $tar = (Get-Command tar.exe -ErrorAction Stop).Source
    if (!(Test-Path -LiteralPath (Join-Path $PSScriptRoot 'probe/CMakeLists.txt'))) {
        throw 'Missing probe/CMakeLists.txt next to this script.'
    }
    $runId = (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0, 8)
    $runPath = "$RemoteRoot/runs/$runId"
    Write-Host "Uploading current probe sources to ${SshTarget}:$runPath"
    Write-Host 'Enter the SSH password when prompted (or use your configured SSH key).'

    # Stream binary tar bytes directly: Windows PowerShell text pipelines corrupt archives.
    # OpenSSH reads the password from the console, independently of its redirected stdin.
    # Normalize the uploaded shell script before Bash parses it, even after a CRLF checkout/edit.
    $remoteCommand = "bash -c 'set -euo pipefail; umask 077; mkdir -p $RemoteRoot/runs; mkdir $runPath; mkdir $runPath/input; tar -xzf - -C $runPath/input; sed -i `"s/\r$//`" $runPath/input/scripts/build-probe-gcc52.sh; bash $runPath/input/scripts/build-probe-gcc52.sh $runPath $Toolchain $RemoteRoot $networkArgument 2>&1 | tee $runPath/build.log'"
    $sshInfo = New-Object System.Diagnostics.ProcessStartInfo
    $sshInfo.FileName = $ssh
    $sshInfo.Arguments = "-o ConnectTimeout=15 $SshTarget `"$remoteCommand`""
    $sshInfo.UseShellExecute = $false
    $sshInfo.RedirectStandardInput = $true
    $sshProcess = [System.Diagnostics.Process]::Start($sshInfo)

    $tarInfo = New-Object System.Diagnostics.ProcessStartInfo
    $tarInfo.FileName = $tar
    $tarInfo.WorkingDirectory = $PSScriptRoot
    $tarInfo.Arguments = '-czf - probe scripts/build-probe-gcc52.sh'
    $tarInfo.UseShellExecute = $false
    $tarInfo.RedirectStandardOutput = $true
    $archiveProcess = [System.Diagnostics.Process]::Start($tarInfo)
    try {
        $archiveProcess.StandardOutput.BaseStream.CopyTo($sshProcess.StandardInput.BaseStream)
    } finally {
        $sshProcess.StandardInput.Close()
    }
    $archiveProcess.WaitForExit()
    $sshProcess.WaitForExit()
    if ($archiveProcess.ExitCode -ne 0) { throw "Source archive failed (exit $($archiveProcess.ExitCode))." }
    if ($sshProcess.ExitCode -ne 0) { throw "Remote build failed (exit $($sshProcess.ExitCode)). See ${SshTarget}:$runPath/build.log" }
    Write-Host "Build succeeded: ${SshTarget}:$RemoteRoot/output/router-probe" -ForegroundColor Green
    Write-Host "This build and its logs: ${SshTarget}:$runPath"
    exit 0
} catch {
    Write-Host "Build failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    foreach ($process in @($archiveProcess, $sshProcess)) {
        if ($null -ne $process) {
            if (!$process.HasExited) { $process.Kill(); $process.WaitForExit() }
            $process.Dispose()
        }
    }
}
