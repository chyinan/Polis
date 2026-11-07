# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-h1-independent-peer-collaboration-review',
    [string]$RuntimeRoot = 'D:\Polis-recovery\r0.3a-h1-review-runtime',
    [string]$SourceState = 'D:\Polis-recovery\r0.3a-backend-continuation-7\run-20260912135427-17a3e1a3b6c3\source-state.json',
    [string]$FrontendResult = 'D:\Programs\Polis\evidence\development\r0.3a-real-frontend-handover-revised-v2\result.json',
    [string]$FrontendState = 'D:\Polis-recovery\r0.3a-frontend-handover-revised-v2-runtime\run-20260913115037-887737d3a289\post-frontend-state.json',
    [string]$BackendBlobs = 'D:\Polis-recovery\r0.3a-backend-continuation-7-runtime\blobs',
    [string]$FrontendBlobs = 'D:\Polis-recovery\r0.3a-frontend-handover-revised-v2-runtime\blobs',
    [string]$OldWriterEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-real-frontend-handover-revised-v2\old-writer-rejection.json',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe',
    [string]$CodeModeHost = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe',
    [string]$AuthFile = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [string]$H1Exe = 'D:\Programs\Polis\.runtime\windows\r0.3a-h1-review\polis-r03a-h1-review.exe'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo
try {
    if (Test-Path -LiteralPath $Evidence) {
        if (@(Get-ChildItem -LiteralPath $Evidence -Force).Count -ne 0) { throw 'H1 evidence path is not fresh' }
    }
    foreach ($path in @($SourceState, $FrontendResult, $FrontendState, $BackendBlobs, $FrontendBlobs, $OldWriterEvidence, "$L1Evidence\result.json", $Binary, $CodeModeHost, $AuthFile, $SelectedConfig)) {
        if (-not (Test-Path -LiteralPath $path)) { throw "H1 input unavailable: $path" }
    }
    New-Item -ItemType Directory -Force -Path $Evidence, $RuntimeRoot, (Split-Path -Parent $H1Exe) | Out-Null
    & bash scripts/build-r03a-h1-review-windows.sh
    if ($LASTEXITCODE -ne 0) { throw "H1 Windows build exited with code $LASTEXITCODE" }
    $l1 = Get-Content -Raw -LiteralPath "$L1Evidence\result.json" | ConvertFrom-Json
    if ([string]$l1.status -ne 'passed' -or [string]$l1.current_binary_windows_l1 -ne 'QUALIFIED' -or [string]$l1.model -ne 'gpt-5.6-luna' -or [string]$l1.runtime_profile -ne 'windows-native') { throw 'current Windows L1 is not qualified for H1' }
    $env:POLIS_CODEX_BINARY = $Binary
    $env:POLIS_CODEX_CODE_MODE_HOST = $CodeModeHost
    $env:POLIS_CODEX_AUTH_FILE = $AuthFile
    $env:POLIS_SELECTED_CODEX_CONFIG = $SelectedConfig
    $env:POLIS_H1_RUNTIME_ROOT = $RuntimeRoot
    $env:POLIS_H1_SOURCE_STATE = $SourceState
    $env:POLIS_H1_FRONTEND_RESULT = $FrontendResult
    $env:POLIS_H1_FRONTEND_STATE = $FrontendState
    $env:POLIS_H1_BACKEND_BLOBS = $BackendBlobs
    $env:POLIS_H1_FRONTEND_BLOBS = $FrontendBlobs
    $env:POLIS_H1_OLD_WRITER_EVIDENCE = $OldWriterEvidence
    $env:POLIS_CURRENT_L1_EVIDENCE = $L1Evidence
    $env:POLIS_CURRENT_L1_FINGERPRINT = [string]$l1.execution_fingerprint
    $env:POLIS_NATIVE_PROXY = ''
    & $H1Exe
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) { throw "H1 reviewer exited with code $exitCode" }
} finally {
    Set-Location -LiteralPath $oldLocation
}
