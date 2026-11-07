# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-real-peer-collaboration',
    [string]$RuntimeRoot = 'D:\Polis-recovery\r0.3a-pagination-v3-real-peer-collaboration-runtime',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe',
    [string]$CodeModeHost = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$BackendExecutionManifest = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2\execution-manifest.json',
    [string]$BackendL2Live = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2-live\result.json',
    [string]$FrontendExecutionManifest = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-v2\execution-manifest.json',
    [string]$FrontendL2Live = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-live\result.json',
    [string]$AcceptanceQualification = 'D:\Programs\Polis\evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json',
    [string]$BlobQualification = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json',
    [string]$CheckerQualification = 'D:\Programs\Polis\evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json',
    [string]$RuntimeManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [string]$Executable = 'D:\Programs\Polis\.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
$pgStarted = $false
$sourceDatabase = ''
$restoreDatabase = ''
$sourceDatabaseCreated = $false
$restoreDatabaseCreated = $false
$backendRuntime = Join-Path $RuntimeRoot 'backend'
$frontendRuntime = Join-Path $RuntimeRoot 'frontend'
$allowancePath = Join-Path $Evidence 'allowance.json'
$backendResultPath = Join-Path $Evidence 'backend-result.json'
$frontendResultPath = Join-Path $Evidence 'frontend-result.json'
$snapshotPath = Join-Path $RuntimeRoot 'authoritative-state.dump'
$snapshotManifestPath = Join-Path $RuntimeRoot 'authoritative-snapshot.json'
$casManifestPath = Join-Path $RuntimeRoot 'cas-manifest.json'
$restoreProofPath = Join-Path $RuntimeRoot 'restore-proof.json'
$backendBindingPath = Join-Path $Evidence 'authorization-binding.json'
$frontendBindingPath = Join-Path $Evidence 'frontend-authorization-binding.json'

function Write-Utf8Json([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if ($parent -ne '') { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    $json = $Value | ConvertTo-Json -Depth 80
    [IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

function Hash-File([string]$Path) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash([IO.File]::ReadAllBytes($Path)))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Invoke-Checked([string]$File, [string[]]$Arguments) {
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$File exited with code $LASTEXITCODE" }
}

function To-WslPath([string]$Path) {
    if ($Path -notmatch '^([A-Za-z]):\\(.*)$') { throw "expected absolute Windows path: $Path" }
    return '/mnt/' + $Matches[1].ToLowerInvariant() + '/' + ($Matches[2] -replace '\\', '/')
}

function Invoke-BashChecked([string]$Command) {
    & bash -lc $Command
    if ($LASTEXITCODE -ne 0) { throw "bash command exited with code $LASTEXITCODE" }
}

function Invoke-Pg([string]$Database, [string]$Command) {
    $repoUnix = To-WslPath $Repo
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $psql = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/psql"
    Invoke-BashChecked "$ld '$psql' -h 127.0.0.1 -p 55432 -d '$Database' -v ON_ERROR_STOP=1 -Atc '$Command'"
}

function Invoke-PgRestore([string]$Database, [string]$Dump) {
    $repoUnix = To-WslPath $Repo
    $dumpUnix = To-WslPath $Dump
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $restore = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_restore"
    Invoke-BashChecked "$ld '$restore' -h 127.0.0.1 -p 55432 -d '$Database' --no-owner --no-acl --exit-on-error '$dumpUnix'"
}

function Test-CAS([string]$Root, $Manifest) {
    foreach ($entry in @($Manifest.entries)) {
        $path = Join-Path (Join-Path $Root ([string]$entry.company_id)) ([string]$entry.content_sha256)
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { return $false }
        if ((Get-Item -LiteralPath $path).Length -ne [int64]$entry.size) { return $false }
        if ((Hash-File $path) -ne [string]$entry.content_sha256) { return $false }
    }
    return $true
}

try {
    Set-Location -LiteralPath $Repo
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -ne 0) { throw 'pagination V3 evidence path is not fresh; refusing retry/reset' }
    }
    foreach ($path in @($AuthSource, $SelectedConfig, (Join-Path $L1Evidence 'result.json'), $BackendExecutionManifest, $BackendL2Live, $FrontendExecutionManifest, $FrontendL2Live, $AcceptanceQualification, $BlobQualification, $CheckerQualification, $RuntimeManifest)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "required controlled input is unavailable: $path" }
    }
    New-Item -ItemType Directory -Force -Path $Evidence, $RuntimeRoot, $backendRuntime, $frontendRuntime | Out-Null
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Executable) | Out-Null

    $env:POLIS_CODEX_BINARY = $Binary
    $env:POLIS_CODEX_CODE_MODE_HOST = $CodeModeHost
    $env:POLIS_CODEX_AUTH_FILE = $AuthSource
    $env:POLIS_SELECTED_CODEX_CONFIG = $SelectedConfig
    $env:POLIS_WINDOWS_RUNTIME_MANIFEST = $RuntimeManifest
    $env:POLIS_CURRENT_L1_EVIDENCE = $L1Evidence
    $env:POLIS_BLOB_DURABILITY_QUALIFICATION = $BlobQualification
    $env:POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION = $CheckerQualification
    $env:POLIS_V3_ACCEPTANCE_QUALIFICATION = $AcceptanceQualification
    $env:POLIS_V3_BACKEND_EXECUTION_MANIFEST = $BackendExecutionManifest
    $env:POLIS_V3_BACKEND_QUALIFICATION = $BackendL2Live
    $env:POLIS_V3_BACKEND_L2_LIVE = $BackendL2Live
    $env:POLIS_V3_FRONTEND_EXECUTION_MANIFEST = $FrontendExecutionManifest
    $env:POLIS_V3_FRONTEND_QUALIFICATION = $FrontendL2Live
    $env:POLIS_V3_FRONTEND_L2_LIVE = $FrontendL2Live
    $env:POLIS_V3_EVIDENCE = $Evidence
    $env:POLIS_V3_ALLOWANCE = $allowancePath
    $env:POLIS_V3_BACKEND_RESULT = $backendResultPath
    $env:POLIS_V3_FRONTEND_RESULT = $frontendResultPath
    $env:POLIS_V3_BACKEND_BINDING = $backendBindingPath
    $env:POLIS_V3_FRONTEND_BINDING = $frontendBindingPath
    $env:POLIS_V3_POSTGRES_SNAPSHOT = $snapshotPath
    $env:POLIS_V3_RESTORE_PROOF = $restoreProofPath
    $env:POLIS_V3_RUNTIME_ROOT = $backendRuntime
    $env:POLIS_V3_POSTGRES_DUMP = (To-WslPath "$Repo\.tools\pg\usr\lib\postgresql\18\bin\pg_dump")
    $env:POLIS_V3_POSTGRES_SNAPSHOT_DSN = ''
    $env:POLIS_DSN = ''

    Invoke-Checked 'bash' @('scripts/build-r03a-pagination-v3-windows.sh')
    Invoke-Checked $Executable @('-preflight')
    Invoke-Checked $Executable @('-continuation-freshness')

    $sourceDatabase = 'polis_r0_3a_pagination_v3_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + $PID
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $sourceDatabase)
    $sourceDatabaseCreated = $true
    $owner = (& bash -lc 'id -un').Trim()
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'migrate', $sourceDatabase, $owner)
    $env:POLIS_DSN = "host=127.0.0.1 port=55432 dbname=$sourceDatabase user=polis_runtime"
    $env:POLIS_V3_POSTGRES_SNAPSHOT_DSN = "host=127.0.0.1 port=55432 dbname=$sourceDatabase user=$owner"
    Invoke-Checked $Executable @('-backend')
    $backend = Get-Content -LiteralPath $backendResultPath -Raw | ConvertFrom-Json
    if ([string]$backend.status -ne 'BACKEND_PASSED_FRONTEND_PENDING' -or [string]$backend.backend_real_execution -ne 'PASSED' -or [string]$backend.authoritative_snapshot -ne 'PASSED') { throw 'Backend V3 business boundary did not pass' }

    $casSource = Join-Path $backendRuntime 'blobs'
    $casDestination = Join-Path $frontendRuntime 'blobs'
    Copy-Item -LiteralPath $casSource -Destination $casDestination -Recurse
    $casManifest = Get-Content -LiteralPath $casManifestPath -Raw | ConvertFrom-Json
    if (-not (Test-CAS $casDestination $casManifest)) { throw 'Backend CAS manifest did not survive controlled staging' }

    $restoreDatabase = 'polis_r0_3a_pagination_v3_restore_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + $PID
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoreDatabase)
    $restoreDatabaseCreated = $true
    Invoke-PgRestore $restoreDatabase $snapshotPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoreDatabase)
    $schema = (Invoke-Pg $restoreDatabase 'SELECT max(version_id) FROM goose_db_version WHERE is_applied').Trim()
    if ($schema -ne '5') { throw "fresh restore schema mismatch: $schema" }
    $restoreProof = [ordered]@{
        status = 'PASSED'
        source_database = $sourceDatabase
        restore_database = $restoreDatabase
        snapshot_path = $snapshotPath
        snapshot_sha256 = Hash-File $snapshotPath
        cas_manifest_path = $casManifestPath
        cas_manifest_digest = [string]$casManifest.manifest_digest
        schema_version = [int]$schema
        cas_verified = $true
        historical_evidence_modified = $false
    }
    Write-Utf8Json $restoreProofPath $restoreProof

    $env:POLIS_V3_RUNTIME_ROOT = $frontendRuntime
    $env:POLIS_DSN = "host=127.0.0.1 port=55432 dbname=$restoreDatabase user=polis_runtime"
    $env:POLIS_V3_POSTGRES_SNAPSHOT_DSN = ''
    Invoke-Checked $Executable @('-frontend')
    $frontend = Get-Content -LiteralPath $frontendResultPath -Raw | ConvertFrom-Json
    if ([string]$frontend.status -ne 'PASSED' -or [string]$frontend.new_subject_real_peer_collaboration -ne 'PASSED' -or [string]$frontend.eligible_for_new_independent_high_review -ne 'True') { throw 'Frontend V3 business completion did not pass' }

    $allowance = Get-Content -LiteralPath $allowancePath -Raw | ConvertFrom-Json
    $final = [ordered]@{
        qualification = 'R0.3A-PAGINATION-V3-REAL-PEER-COLLABORATION'
        status = 'PASSED'
        parent_problem_key = 'r03a-real-peer-collaboration-v1'
        subject_revision = 'r03a-pagination-v3-business-subject@1'
        public_contract_revision = 'r03a-pagination-contract@3'
        runtime_verifier_revision = 'r03a-pagination-behavior@2'
        backend_result = $backend
        frontend_result = $frontend
        allowance = $allowance
        medium_started = [int]$allowance.medium_turns
        high_started = [int]$allowance.high_turns
        provider_egress = [int]$frontend.provider_egress + [int]$backend.provider_egress
        transport_state = 'TERMINALLY_RECONCILED'
        snapshot_restore = $restoreProof
        historical_evidence_modified = $false
        new_subject_real_peer_collaboration = 'PASSED'
        eligible_for_new_independent_high_review = $true
    }
    Write-Utf8Json (Join-Path $Evidence 'result.json') $final
    Get-Content -LiteralPath (Join-Path $Evidence 'result.json') -Raw
} finally {
    if ($restoreDatabaseCreated -and $restoreDatabase -ne '') { try { & bash scripts/r03a-real-backend-pg.sh drop $restoreDatabase } catch { Write-Warning "restore DB cleanup failed: $($_.Exception.Message)" } }
    if ($sourceDatabaseCreated -and $sourceDatabase -ne '') { try { & bash scripts/r03a-real-backend-pg.sh drop $sourceDatabase } catch { Write-Warning "source DB cleanup failed: $($_.Exception.Message)" } }
    if ($pgStarted) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "PG cleanup failed: $($_.Exception.Message)" } }
    Set-Location -LiteralPath $oldLocation
}
