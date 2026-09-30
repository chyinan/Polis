param(
    [string]$Repo = 'D:\Programs\Polis'
)

$ErrorActionPreference = 'Stop'
$windowsScript = Join-Path $Repo 'scripts\r03a-t19-windows-qualify.ps1'
& powershell -NoProfile -ExecutionPolicy Bypass -File $windowsScript
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$wslRepo = '/mnt/d/Programs/Polis'
& wsl.exe -d Ubuntu-22.04 -- env -u POLIS_NATIVE_PROXY -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u NO_PROXY POLIS_CODEX_AUTH_FILE=/mnt/c/Users/chyinan/.codex/auth.json bash scripts/go.sh run ./cmd/polis-r03a-t19
exit $LASTEXITCODE
