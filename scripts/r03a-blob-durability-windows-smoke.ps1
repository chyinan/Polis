param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Root = 'D:\Programs\Polis\.runtime\windows\r0.3a-blob-durability-smoke',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\windows-smoke.json',
    [string]$Executable = 'D:\Programs\Polis\.runtime\windows\r0.3a-blob-durability\polis-r03a-blob-durability.exe'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo

try {
    if (Test-Path -LiteralPath $Evidence) { throw 'blob durability smoke evidence already exists; refusing overwrite' }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Executable), (Split-Path -Parent $Evidence) | Out-Null
    & bash scripts/build-r03a-blob-durability-windows.sh
    if ($LASTEXITCODE -ne 0) { throw "Windows blob durability build exited with code $LASTEXITCODE" }
    & $Executable -root $Root -evidence $Evidence
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) { throw "Windows blob durability smoke exited with code $exitCode" }
    Get-Content -Raw -LiteralPath $Evidence
}
finally {
    Set-Location -LiteralPath $oldLocation
}
