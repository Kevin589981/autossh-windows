$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
Push-Location $root
try {
    go build -trimpath -ldflags '-s -w' -o (Join-Path $root 'autossh.exe') ./cmd/autossh
}
finally {
    Pop-Location
}
Write-Host "Built $root\autossh.exe"
