param([string]$Dotnet='dotnet',[string]$Image='router-agent-terminal-test:local')
$ErrorActionPreference='Stop'
$repoRoot=Split-Path $PSScriptRoot -Parent
$fixture=Join-Path $repoRoot 'build/conpty-services'
New-Item -ItemType Directory -Force $fixture | Out-Null
$container='router-agent-conpty-'+[Guid]::NewGuid().ToString('N')
$startup=@'
set -eu
ssh-keygen -A >/dev/null 2>&1
printf 'root:%s\n' "$(cat /fixture/password)" | chpasswd
printf '%s\n' 'Port 22' 'ListenAddress 0.0.0.0' 'PermitRootLogin yes' 'PasswordAuthentication yes' 'KbdInteractiveAuthentication no' 'AllowTcpForwarding no' 'HostKey /etc/ssh/ssh_host_ed25519_key' 'PidFile /tmp/sshd.pid' > /tmp/sshd_config
/usr/sbin/sshd -D -e -f /tmp/sshd_config &
busybox-extras telnetd -F -p 23 -l /bin/sh &
wait
'@
[IO.File]::WriteAllText((Join-Path $fixture 'start.sh'),$startup.Replace("`r`n","`n"))
[IO.File]::WriteAllText((Join-Path $fixture 'password'),[Guid]::NewGuid().ToString('N'))
try {
    docker run --rm -d --name $container -p 127.0.0.1:20222:22 -p 127.0.0.1:20223:23 --mount "type=bind,source=$fixture,target=/fixture,readonly" $Image /bin/sh /fixture/start.sh | Out-Null
    if($LASTEXITCODE -ne 0){throw 'Terminal fixture start failed; verify image and free loopback ports'}
    $key=''
    for($i=0;$i -lt 50;$i++) {
        $key=docker exec $container cat /etc/ssh/ssh_host_ed25519_key.pub 2>$null
        if($LASTEXITCODE -eq 0){break}
        Start-Sleep -Milliseconds 100
    }
    if(!$key){throw 'Fixture host key unavailable'}
    [IO.File]::WriteAllText((Join-Path $fixture 'known_hosts'),"[127.0.0.1]:20222 $key`n")
    & $Dotnet build (Join-Path $repoRoot 'windows/RouterWorkbench.Tests') -c Release -o (Join-Path $fixture 'bin')
    if($LASTEXITCODE -ne 0){throw 'Terminal test build failed'}
    & $Dotnet (Join-Path $fixture 'bin/RouterWorkbench.Tests.dll') --terminal-services $fixture
    if($LASTEXITCODE -ne 0){throw 'Terminal service tests failed'}
} finally {
    docker rm -f $container 2>$null | Out-Null
    Remove-Item -LiteralPath (Join-Path $fixture 'password') -ErrorAction SilentlyContinue
}
