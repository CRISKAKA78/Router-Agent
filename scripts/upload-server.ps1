[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$ArtifactDirectory)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$projectRoot = Split-Path $PSScriptRoot -Parent
$privateKey = Join-Path $projectRoot 'id_rsa'
$passwordFile = Join-Path $projectRoot 'password.txt'
foreach ($required in @($privateKey, $passwordFile, (Join-Path $ArtifactDirectory 'router-server'), (Join-Path $ArtifactDirectory 'start.sh'))) {
    if (!(Test-Path -LiteralPath $required -PathType Leaf)) { throw "Missing file: $required" }
}
$sftp = (Get-Command sftp.exe -ErrorAction Stop).Source
$compiler = Join-Path $env:WINDIR 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
if (!(Test-Path -LiteralPath $compiler)) { throw 'Windows .NET Framework C# compiler is required for the SSH passphrase helper.' }
$runId = [Guid]::NewGuid().ToString('N')
$temporaryDirectory = Join-Path $env:TEMP "rmp-server-$runId"
$savedEnvironment = @{}
try {
    New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $directoryAcl = New-Object Security.AccessControl.DirectorySecurity
    $directoryAcl.SetOwner($identity)
    $directoryAcl.SetAccessRuleProtection($true, $false)
    $directoryAcl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')))
    Set-Acl -LiteralPath $temporaryDirectory -AclObject $directoryAcl
    $keyCopy = Join-Path $temporaryDirectory 'id_rsa'
    Copy-Item -LiteralPath $privateKey -Destination $keyCopy
    $askpass = Join-Path $temporaryDirectory 'askpass.exe'
    & $compiler /nologo /target:exe "/out:$askpass" (Join-Path $PSScriptRoot 'ssh-askpass.cs')
    if ($LASTEXITCODE -ne 0) { throw 'SSH passphrase helper build failed.' }
    foreach ($variableName in @('SSH_ASKPASS', 'SSH_ASKPASS_REQUIRE', 'DISPLAY', 'RMP_KEY_PASSWORD_FILE')) {
        $savedEnvironment[$variableName] = [Environment]::GetEnvironmentVariable($variableName, 'Process')
    }
    $env:SSH_ASKPASS = $askpass
    $env:SSH_ASKPASS_REQUIRE = 'force'
    $env:DISPLAY = 'rmp:0'
    $env:RMP_KEY_PASSWORD_FILE = $passwordFile
    $batch = Join-Path $temporaryDirectory 'upload.sftp'
    $commands = @(
        '-mkdir /root/agent-server',
        'cd /root/agent-server',
        "put router-server .router-server-$runId", "chmod 755 .router-server-$runId",
        "put start.sh .start-$runId", "chmod 755 .start-$runId",
        "rename .router-server-$runId router-server", "rename .start-$runId start.sh"
    )
    [IO.File]::WriteAllLines($batch, $commands, (New-Object Text.UTF8Encoding($false)))
    Push-Location $ArtifactDirectory
    try {
        & $sftp -P 22 -i $keyCopy -o IdentitiesOnly=yes -o BatchMode=no -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15 -b $batch root@47.119.168.150
        if ($LASTEXITCODE -ne 0) { throw 'SFTP upload failed; local build artifacts are retained.' }
    } finally { Pop-Location }
    Write-Host 'Uploaded: root@47.119.168.150:/root/agent-server'
    Write-Host 'Start on Linux: /root/agent-server/start.sh'
} finally {
    foreach ($variableName in $savedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($variableName, $savedEnvironment[$variableName], 'Process')
    }
    foreach ($name in @('id_rsa', 'askpass.exe', 'upload.sftp')) {
        $temporaryFile = Join-Path $temporaryDirectory $name
        if (Test-Path -LiteralPath $temporaryFile) { Remove-Item -LiteralPath $temporaryFile -Force }
    }
    if (Test-Path -LiteralPath $temporaryDirectory) { Remove-Item -LiteralPath $temporaryDirectory }
}
