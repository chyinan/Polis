[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$frontendDirectory = Join-Path $repositoryRoot 'frontend'
$tauriProjectDirectory = Join-Path $repositoryRoot 'desktop\src-tauri'
$postgresRuntimeDirectory = Join-Path $repositoryRoot '.tools\pg-windows-runtime'

if (-not (Test-Path -LiteralPath $postgresRuntimeDirectory -PathType Container)) {
    throw 'The packaged Windows PostgreSQL runtime is missing at .tools/pg-windows-runtime.'
}
$signerPath = Join-Path $PSScriptRoot 'sign-windows.ps1'
& powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $signerPath -CheckOnly
if ($LASTEXITCODE -ne 0) {
    throw 'Windows signing prerequisites are not available.'
}

$tauriCli = Join-Path $frontendDirectory 'node_modules\.bin\tauri.cmd'
if (-not (Test-Path -LiteralPath $tauriCli -PathType Leaf)) {
    throw 'Tauri CLI is missing; install the frontend dependencies first.'
}

Push-Location $frontendDirectory
try {
    & npm.cmd run build
    if ($LASTEXITCODE -ne 0) {
        throw "Frontend production build failed with exit code $LASTEXITCODE."
    }
    Push-Location $tauriProjectDirectory
    try {
        & $tauriCli build --config tauri.signing.conf.json
        if ($LASTEXITCODE -ne 0) {
            throw "Signed Tauri build failed with exit code $LASTEXITCODE."
        }
    } finally {
        Pop-Location
    }
} finally {
    Pop-Location
}
