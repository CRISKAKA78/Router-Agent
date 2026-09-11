# Compatibility alias: both architectures are now built by probe-build.ps1.
& (Join-Path $PSScriptRoot 'probe-build.ps1') @args
exit $LASTEXITCODE
