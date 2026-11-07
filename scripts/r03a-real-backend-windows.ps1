param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-real-backend-employee\continuation-6',
    [string]$RuntimeRoot = 'D:\Programs\Polis\.runtime\windows\r0.3a-real-backend-continuation-6',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe',
    [string]$CodeModeHost = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$QualificationEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2',
    [string]$L2LiveEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2-live',
    [string]$BehavioralContractQualification = 'D:\Programs\Polis\summary\r0-3a-behavioral-contract-hardening.md',
    [string]$BlobDurabilityQualification = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json',
    [string]$RuntimeArtifactManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [int]$ToolCallLimit = 48,
    [string]$BackendExe = 'D:\Programs\Polis\.runtime\windows\r0.3a-real-backend\polis-r03a-real-backend.exe'
)

$ErrorActionPreference = 'Stop'
$database = ''
$pgStarted = $false
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo

function Write-Utf8Json([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Depth 20
    [IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

function Invoke-Checked([string]$File, [string[]]$Arguments) {
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$File exited with code $LASTEXITCODE" }
}

try {
    $existing = if (Test-Path -LiteralPath $Evidence) { @(Get-ChildItem -LiteralPath $Evidence -Force) } else { @() }
    $preflightPath = Join-Path $Evidence 'preflight.json'
    $freshnessPath = Join-Path $Evidence 'continuation-freshness.json'
    $allowedExisting = @('preflight.json', 'continuation-freshness.json')
    if (@($existing | Where-Object { $allowedExisting -notcontains $_.Name }).Count -ne 0) { throw 'real Backend evidence has unexpected files; refusing retry/reset' }
    $controlledInputs = @($AuthSource, $SelectedConfig, "$L1Evidence\result.json", "$QualificationEvidence\execution-manifest.json", "$QualificationEvidence\offline-result.json", "$L2LiveEvidence\result.json", $BlobDurabilityQualification, $BehavioralContractQualification, $RuntimeArtifactManifest)
    if ([string]::IsNullOrEmpty($RuntimeArtifactManifest)) { throw 'current-binary controlled runtime manifest is required; fallback is forbidden' }
    foreach ($path in $controlledInputs) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "required controlled input is unavailable: $path" }
    }

    New-Item -ItemType Directory -Force -Path $RuntimeRoot | Out-Null
    $manifest = Get-Content -Raw -LiteralPath "$QualificationEvidence\execution-manifest.json" | ConvertFrom-Json
    if ([string]$manifest.schema_version -ne 'r03a-current-binary-revised-11-tool-l2-manifest-v1' -or [string]$manifest.qualification -ne 'R0.3A-CURRENT-BINARY-REVISED-11-TOOL-L2') { throw 'current revised L2 execution manifest schema or qualification mismatch' }
    $executionFingerprint = [string]$manifest.execution_fingerprint
    if ([string]::IsNullOrEmpty($executionFingerprint)) { throw 'current revised L2 execution fingerprint is empty' }
    $binding = [ordered]@{
        execution_fingerprint = $executionFingerprint
        employee_id = 'emp-backend'
        problem_key = 'r03a-real-peer-collaboration-v1'
        purpose = 'real_backend_peer_collaboration'
        model = 'gpt-5.6-luna'
        profile = 'gpt-5.6-luna/medium'
        effort = 'medium'
        allowance_limits = [ordered]@{ medium_limit = 1; high_limit = 0; concurrency = 1; tool_call_limit = $ToolCallLimit }
        l1_fingerprint = [string]$manifest.l1_fingerprint
        controlled_runtime_manifest_digest = [string]$manifest.controlled_runtime_manifest_digest
        revised_l2_tool_manifest_digest = [string]$manifest.tool_manifest_digest
    }
    $bindingPath = Join-Path $RuntimeRoot 'authorization-binding.json'
    Write-Utf8Json $bindingPath $binding

    $env:POLIS_CODEX_BINARY = $Binary
    $env:POLIS_CODEX_CODE_MODE_HOST = $CodeModeHost
    $env:POLIS_CODEX_AUTH_FILE = $AuthSource
    $env:POLIS_SELECTED_CODEX_CONFIG = $SelectedConfig
    $env:POLIS_CURRENT_EXECUTION_MANIFEST = "$QualificationEvidence\execution-manifest.json"
    $env:POLIS_CURRENT_EXECUTION_CONFIG = "$QualificationEvidence\execution-manifest.json"
    $env:POLIS_BLOB_DURABILITY_QUALIFICATION = $BlobDurabilityQualification
    $env:POLIS_WINDOWS_RUNTIME_MANIFEST = $RuntimeArtifactManifest
    $env:POLIS_BEHAVIORAL_CONTRACT_QUALIFICATION = $BehavioralContractQualification
    $env:POLIS_BUSINESS_QUALIFICATION_RECORD = "$L2LiveEvidence\result.json"
    $env:POLIS_CURRENT_L1_EVIDENCE = $L1Evidence
    $env:POLIS_EXPECTED_NATIVE_VERSION = '0.154.0-alpha.6.2'
    $env:POLIS_BUSINESS_AUTHORIZATION_BINDING = $bindingPath
    $env:POLIS_BUSINESS_EXECUTION_FINGERPRINT = $executionFingerprint
    $env:POLIS_BACKEND_ORCHESTRATION_REVISION = 'r0.3a-real-backend-continuation-6-orchestration-v1'
    $env:POLIS_BACKEND_RUNTIME_ROOT = $RuntimeRoot
    $env:POLIS_BACKEND_EVIDENCE = $Evidence
    $env:POLIS_BUSINESS_TOOL_CALL_LIMIT = [string]$ToolCallLimit

    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $BackendExe) | Out-Null
    Invoke-Checked 'bash' @('scripts/build-r03a-real-backend-windows.sh')

    if (-not (Test-Path -LiteralPath $preflightPath -PathType Leaf)) {
        Invoke-Checked $BackendExe @('-preflight')
    }
    if (-not (Test-Path -LiteralPath $freshnessPath -PathType Leaf)) {
        Invoke-Checked $BackendExe @('-continuation-freshness')
    }

    $database = 'polis_r0_3a_backend_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + $PID
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $database)
    $owner = (& bash -lc 'id -un').Trim()
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'migrate', $database, $owner)
    $env:POLIS_DSN = "host=127.0.0.1 port=55432 dbname=$database user=polis_runtime"

    & $BackendExe
    $backendExit = $LASTEXITCODE
    if ($backendExit -ne 0) { throw "real Backend command exited with code $backendExit" }
    Get-Content -Raw -LiteralPath "$Evidence\result.json"
}
finally {
    if ($database -ne '') {
        try { & bash scripts/r03a-real-backend-pg.sh drop $database } catch { Write-Warning "dedicated test database cleanup failed: $($_.Exception.Message)" }
    }
    if ($pgStarted) {
        try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "dedicated test PostgreSQL cleanup failed: $($_.Exception.Message)" }
    }
    Set-Location -LiteralPath $oldLocation
}
