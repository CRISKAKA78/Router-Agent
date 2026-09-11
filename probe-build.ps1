[CmdletBinding()]
param(
    [ValidatePattern('^[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+$')]
    [string]$SshTarget = 'root@10.1.1.128',
    [ValidateRange(1,65535)][int]$Port = 22,
    [ValidatePattern('^/root/[A-Za-z0-9_-]+$')]
    [string]$RemoteRoot = '/root/router-agent',
    [ValidatePattern('^/root/[A-Za-z0-9_.-]+$')]
    [string]$Toolchain = '/root/gcc-5.2',
    [ValidatePattern('^[A-Za-z0-9_,.-]*$')]
    [string]$NetworkInterfaces = 'br0,eth0,eth1,usb0',
    [string]$PasswordFile = '',
    [string]$ToolchainArchive = (Join-Path ([Environment]::GetFolderPath('Desktop')) 'gcc-5.4.tar.gz')
)

$ErrorActionPreference = 'Stop'
$savedEnvironment = @{}
$askpassDirectory = $null
try {
    if ($NetworkInterfaces -and ($NetworkInterfaces -notmatch '^[A-Za-z0-9_.-]{1,15}(,[A-Za-z0-9_.-]{1,15})*$' -or @($NetworkInterfaces.Split(',')).Count -gt 32 -or @($NetworkInterfaces.Split(',') | Select-Object -Unique).Count -ne @($NetworkInterfaces.Split(',')).Count -or @($NetworkInterfaces.Split(',') | Where-Object { $_ -in '.', '..' }).Count)) { throw 'Invalid interface names.' }
    $networkArgument = if ($NetworkInterfaces) { $NetworkInterfaces } else { '-' }
    $ssh = (Get-Command ssh.exe -ErrorAction Stop).Source
    $sftp = (Get-Command sftp.exe -ErrorAction Stop).Source
    $tar = (Get-Command tar.exe -ErrorAction Stop).Source
    if (!(Test-Path -LiteralPath (Join-Path $PSScriptRoot 'probe/CMakeLists.txt'))) {
        throw 'Missing probe/CMakeLists.txt next to this script.'
    }
    if (!$PasswordFile) { $PasswordFile = Join-Path $PSScriptRoot 'password.txt' }
    $passwordPath = (Resolve-Path -LiteralPath $PasswordFile -ErrorAction Stop).Path
    $password = [IO.File]::ReadAllText($passwordPath).TrimEnd([char[]]([char]13,[char]10))
    if ([string]::IsNullOrEmpty($password) -or $password.Contains([char]13) -or $password.Contains([char]10)) { throw 'password.txt must contain one non-empty password line.' }
    $password = $null
    $helperCompiler = Join-Path $env:WINDIR 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
    if (!(Test-Path -LiteralPath $helperCompiler)) { throw 'Windows .NET Framework C# compiler is required for the SSH password helper.' }
    # Avoid the OpenSSH AskPass launch failure under the Chinese repository path.
    if ($env:TEMP -match '[^\x00-\x7F]') { throw 'SSH helper requires an ASCII TEMP path. Set TEMP to an existing writable ASCII directory.' }
    $askpassDirectory = Join-Path $env:TEMP ('rmp-probe-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $askpassDirectory | Out-Null
    $askpass = Join-Path $askpassDirectory 'askpass.exe'
    & $helperCompiler /nologo /target:exe "/out:$askpass" (Join-Path $PSScriptRoot 'scripts/ssh-askpass.cs')
    if ($LASTEXITCODE -ne 0) { throw 'SSH password helper build failed.' }
    foreach ($name in @('SSH_ASKPASS', 'SSH_ASKPASS_REQUIRE', 'DISPLAY', 'RMP_KEY_PASSWORD_FILE')) {
        $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    }
    $env:SSH_ASKPASS = $askpass
    $env:SSH_ASKPASS_REQUIRE = 'force'
    $env:DISPLAY = 'rmp:0'
    $env:RMP_KEY_PASSWORD_FILE = $passwordPath
    $state = Join-Path $PSScriptRoot 'build/gcc54'
    New-Item -ItemType Directory -Force -Path $state | Out-Null
    $knownHosts = Join-Path $state 'known_hosts'
    # Honor both the existing GCC54 host key and the default OpenSSH record.
    $defaultHosts = (Join-Path $HOME '.ssh/known_hosts').Replace('\', '/')
    $hostFiles = 'UserKnownHostsFile="' + $knownHosts.Replace('\', '/') + '" "' + $defaultHosts + '"'
    $sshOptions = @('-o', 'ConnectTimeout=15', '-o', 'ServerAliveInterval=15', '-o', 'ServerAliveCountMax=3', '-o', 'StrictHostKeyChecking=accept-new', '-o', $hostFiles, '-o', 'BatchMode=no', '-o', 'PreferredAuthentications=password', '-o', 'PubkeyAuthentication=no', '-o', 'NumberOfPasswordPrompts=1')
    Write-Host "Checking SSH login to $SshTarget (password from local file)."
    & $ssh -T -p $Port @sshOptions $SshTarget 'true'
    if ($LASTEXITCODE -ne 0) { throw "SSH login failed (exit $LASTEXITCODE); see SSH diagnostic above. No sources uploaded." }
    $runId = (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0, 8)
    $runPath = "$RemoteRoot/runs/$runId"
    Write-Host "Uploading current probe sources to ${SshTarget}:$runPath"
    Write-Host 'Authenticating with the password from local password.txt.'

    $archive = Join-Path $askpassDirectory 'source.tar.gz'
    & $tar -czf $archive -C $PSScriptRoot probe scripts/build-probe-all.sh scripts/build-probe-gcc52.sh scripts/build-probe-gcc54.sh
    if ($LASTEXITCODE -ne 0) { throw 'Source archive failed.' }
    & $ssh -T -p $Port @sshOptions $SshTarget "umask 077; mkdir -p $RemoteRoot/runs && mkdir $runPath"
    if ($LASTEXITCODE -ne 0) { throw 'SSH login or remote directory preparation failed.' }
    $batch = Join-Path $askpassDirectory 'upload.sftp'
    [IO.File]::WriteAllText($batch, "put source.tar.gz $runPath/source.tar.gz`n", (New-Object Text.UTF8Encoding($false)))
    Push-Location $askpassDirectory
    try {
        & $sftp -P $Port @sshOptions -b $batch $SshTarget
        if ($LASTEXITCODE -ne 0) { throw 'Source upload failed.' }
    } finally { Pop-Location }
    & $ssh -T -p $Port @sshOptions $SshTarget "test -f $RemoteRoot/toolchain/gcc-5.4/.router-agent-installed || test -f /root/router-probe-gcc54/toolchain/gcc-5.4/.router-agent-installed"
    if ($LASTEXITCODE -eq 1) {
        $sdkArchive = (Resolve-Path -LiteralPath $ToolchainArchive -ErrorAction Stop).Path.Replace('\', '/')
        if ($sdkArchive.Contains('"') -or $sdkArchive.Contains("`r") -or $sdkArchive.Contains("`n")) { throw 'Invalid SDK archive path.' }
        Write-Host 'Uploading GCC 5.4 SDK (no installed cache found).'
        [IO.File]::WriteAllText($batch, "put `"$sdkArchive`" $runPath/gcc-5.4.tar.gz`n", (New-Object Text.UTF8Encoding($false)))
        & $sftp -P $Port @sshOptions -b $batch $SshTarget
        if ($LASTEXITCODE -ne 0) { throw 'SDK upload failed; compilation was not started.' }
    } elseif ($LASTEXITCODE -ne 0) { throw "Cannot check remote SDK (SSH exit $LASTEXITCODE)." }
    $remoteCommand = "bash -c 'set -euo pipefail; umask 077; mkdir $runPath/input; tar -xzf $runPath/source.tar.gz -C $runPath/input; sed -i s/\\r$// $runPath/input/scripts/build-probe-*.sh; bash $runPath/input/scripts/build-probe-all.sh $runPath $Toolchain $RemoteRoot $networkArgument 2>&1 | tee $runPath/build.log'"
    & $ssh -T -p $Port @sshOptions $SshTarget $remoteCommand
    if ($LASTEXITCODE -ne 0) { throw "Remote build failed (exit $LASTEXITCODE). See ${SshTarget}:$runPath/build.log" }
    Write-Host "Build succeeded: ${SshTarget}:$RemoteRoot/router-agent-armv7" -ForegroundColor Green
    Write-Host "Build succeeded: ${SshTarget}:$RemoteRoot/router-agent-mipsel" -ForegroundColor Green
    Write-Host "This build and its logs: ${SshTarget}:$runPath"
    exit 0
} catch {
    Write-Host "Build failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    foreach ($name in $savedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], 'Process')
    }
    if ($askpassDirectory -and (Test-Path -LiteralPath $askpassDirectory)) {
        foreach ($name in @('askpass.exe', 'source.tar.gz', 'upload.sftp')) {
            $temporaryFile = Join-Path $askpassDirectory $name
            if (Test-Path -LiteralPath $temporaryFile) { Remove-Item -LiteralPath $temporaryFile -Force }
        }
        Remove-Item -LiteralPath $askpassDirectory
    }
}
