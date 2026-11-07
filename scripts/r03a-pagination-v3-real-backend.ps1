# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-real-backend-v2',
    [string]$RuntimeRoot = 'D:\Polis-recovery\r0.3a-pagination-v3-real-backend-v2-runtime',
    [string]$PostgresRecoveryRootWsl = '',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe',
    [string]$CodeModeHost = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$BackendExecutionManifest = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-backend-l2-v2\execution-manifest.json',
    [string]$BackendL2Live = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-backend-l2-live-v3\result.json',
    [string]$FrontendExecutionManifest = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-v2\execution-manifest.json',
    [string]$FrontendL2Live = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-live\result.json',
    [string]$FrontendFreshnessEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-real-backend-v2\frontend-manifest-freshness',
    [string]$FrontendHandoverProtocol = 'D:\Programs\Polis\evidence\development\r0.3a-real-frontend-handover-v4\2b7a6fd917d335459b0dbf32dda34158\protocol.jsonl',
    [string]$AcceptanceQualification = 'D:\Programs\Polis\evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json',
    [string]$BlobQualification = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json',
    [string]$CheckerQualification = 'D:\Programs\Polis\evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json',
    [string]$RuntimeManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [string]$Executable = 'D:\Programs\Polis\.runtime\windows\r0.3a-pagination-v3-real-backend-v2\polis-r03a-pagination-v3.exe'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
$pgStarted = $false
$sourceDatabase = ''
$restoreDatabase = ''
$sourceDatabaseCreated = $false
$restoreDatabaseCreated = $false
$recoveryPackageComplete = $false
$backendRuntime = Join-Path $RuntimeRoot 'backend'
$restoreRuntime = Join-Path $RuntimeRoot 'restore'
$allowancePath = Join-Path $Evidence 'allowance.json'
$backendResultPath = Join-Path $Evidence 'backend-result.json'
$snapshotPath = Join-Path $RuntimeRoot 'authoritative-state.dump'
$snapshotManifestPath = Join-Path $RuntimeRoot 'authoritative-snapshot.json'
$casManifestPath = Join-Path $RuntimeRoot 'cas-manifest.json'
$restoreProofPath = Join-Path $RuntimeRoot 'restore-proof.json'
$backendBindingPath = Join-Path $Evidence 'authorization-binding.json'
$stateQueryPath = Join-Path $RuntimeRoot 'state-query.sql'
$authoritativeStatePath = Join-Path $Evidence 'authoritative-state.json'
$restoredStatePath = Join-Path $Evidence 'restored-state-before-runtime-recovery.json'
$resultPath = Join-Path $Evidence 'result.json'
$preservationManifestPath = Join-Path $Evidence 'PreservedPostgresAccessManifest.json'
$backend = $null
$frontendFreshness = $null

function Write-Utf8Text([string]$Path, [string]$Text) {
    $parent = Split-Path -Parent $Path
    if ($parent -ne '') { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Text, [Text.UTF8Encoding]::new($false))
}

function Write-Utf8Json([string]$Path, $Value) {
    Write-Utf8Text $Path (($Value | ConvertTo-Json -Depth 100) + [Environment]::NewLine)
}

function Hash-Bytes([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Hash-File([string]$Path) { return Hash-Bytes ([IO.File]::ReadAllBytes($Path)) }
function Hash-Text([string]$Text) { return Hash-Bytes ([Text.UTF8Encoding]::new($false).GetBytes($Text)) }

function Write-PreservedPostgresManifest([string]$Database, [string]$CompanyID) {
    if ([string]::IsNullOrWhiteSpace($PostgresRecoveryRootWsl)) { throw 'PostgresRecoveryRootWsl is required for preservation-aware Backend runs' }
    $pgDataWsl = "$PostgresRecoveryRootWsl/postgres/pgdata"
    $repoUnix = To-WslPath $Repo
    $pgBin = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $systemIdentifier = (& bash -lc "$ld '$pgBin/pg_controldata' '$pgDataWsl' | awk -F: '/Database system identifier/{gsub(/ /,\"\",\$2);print \$2}'").Trim()
    $pgVersion = (Invoke-Pg $Database 'SHOW server_version').Trim()
    $dbOID = (Invoke-Pg 'postgres' "SELECT oid::text FROM pg_database WHERE datname='$Database'").Trim()
    $dbOwner = (Invoke-Pg 'postgres' "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$Database'").Trim()
    $backendTaskID = (Invoke-Pg $Database "SELECT id FROM tasks WHERE company_id='$CompanyID' AND owner='emp-backend' AND kind='peer_backend' ORDER BY id LIMIT 1").Trim()
    $missionID = (Invoke-Pg $Database "SELECT mission_id FROM tasks WHERE company_id='$CompanyID' AND id='$backendTaskID'").Trim()
    $frontendTaskID = (Invoke-Pg $Database "SELECT id FROM tasks WHERE company_id='$CompanyID' AND mission_id='$missionID' AND owner='emp-frontend' AND kind='peer_frontend' ORDER BY id LIMIT 1").Trim()
    $pgVersionFile = "$pgDataWsl/PG_VERSION"
    $pgControlHash = (& bash -lc "sha256sum '$pgDataWsl/global/pg_control' | awk '{print \$1}'").Trim()
    $pgDataDigest = Hash-Text ((& bash -lc "cat '$pgVersionFile'").Trim() + '|' + $pgControlHash)
    $roleGrantDigest = Hash-Text 'role-provisioning@1|role=polis_runtime|public:USAGE|ALL TABLES:SELECT,INSERT,UPDATE'
    $migrationDigest = Hash-Text ((Get-ChildItem -LiteralPath (Join-Path $Repo 'db\migrations') -File | Sort-Object Name | ForEach-Object { $_.Name + ':' + (Hash-File $_.FullName) }) -join '|')
    Write-Utf8Json $preservationManifestPath ([ordered]@{ manifest_revision='r0.3a-preserved-postgres-access@1'; postgres_version=$pgVersion; pgdata_canonical_path=$pgDataWsl; pgdata_digest=$pgDataDigest; cluster_system_identifier=$systemIdentifier; database_name=$Database; database_oid=$dbOID; database_owner=$dbOwner; runtime_role_name='polis_runtime'; role_provisioning_revision='role-provisioning@1'; role_grant_spec_digest=$roleGrantDigest; migration_schema_digest=$migrationDigest; company_id=$CompanyID; mission_id=$missionID; backend_task_id=$backendTaskID; frontend_task_id=$frontendTaskID; created_at=[DateTime]::UtcNow.ToString('o'); preservation_reason='R0.3A Backend terminal authoritative recovery cut'; credentials_included=$false; manifest_creation_phase='pre_destructive_boundary'; historical_manifest_present=$false; source='production_authoritative_database'; business_state_source='existing_database_only' })
    Write-Utf8Text ($preservationManifestPath + '.sha256') (Hash-File $preservationManifestPath + [Environment]::NewLine)
}

function Test-PreservedPostgresManifestMachinery {
    if ([string]::IsNullOrWhiteSpace($PostgresRecoveryRootWsl) -or $PostgresRecoveryRootWsl -match '^(/tmp/|/mnt/[cd]/)') { throw 'PreservedPostgresAccessManifest machinery requires a durable WSL recovery root' }
    $repoUnix = To-WslPath $Repo
    $pgControlData = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_controldata"
    Invoke-BashChecked "test -x '$pgControlData' && '$pgControlData' --version >/dev/null"
    if (-not (Test-Path -LiteralPath (Split-Path -Parent $preservationManifestPath -ErrorAction Stop) -PathType Container)) { throw 'PreservedPostgresAccessManifest evidence directory is unavailable' }
    if (Test-Path -LiteralPath $preservationManifestPath -PathType Leaf) { throw 'PreservedPostgresAccessManifest already exists; refusing overwrite' }
}

function Assert-PreservedPostgresManifest {
    if (-not (Test-Path -LiteralPath $preservationManifestPath -PathType Leaf)) { throw 'PreservedPostgresAccessManifest emission failed' }
    $manifest = Get-Content -LiteralPath $preservationManifestPath -Raw | ConvertFrom-Json
    foreach ($field in @('manifest_revision','postgres_version','pgdata_canonical_path','pgdata_digest','cluster_system_identifier','database_name','database_oid','database_owner','runtime_role_name','role_provisioning_revision','role_grant_spec_digest','migration_schema_digest','company_id','mission_id','backend_task_id','frontend_task_id','created_at','preservation_reason','manifest_creation_phase','source','business_state_source')) {
        if ([string]::IsNullOrWhiteSpace([string]$manifest.$field)) { throw "PreservedPostgresAccessManifest missing required field: $field" }
    }
    if ([bool]$manifest.credentials_included -or $manifest.manifest_creation_phase -ne 'pre_destructive_boundary' -or $manifest.source -ne 'production_authoritative_database' -or $manifest.business_state_source -ne 'existing_database_only') { throw 'PreservedPostgresAccessManifest provenance is invalid' }
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
	$queryPath = Join-Path $RuntimeRoot ("query-" + [guid]::NewGuid().ToString('N') + '.sql')
	Write-Utf8Text $queryPath ($Command + [Environment]::NewLine)
	try { return Invoke-PgFile $Database $queryPath }
	finally { if (Test-Path -LiteralPath $queryPath) { Remove-Item -LiteralPath $queryPath -Force } }
}

function Invoke-PgFile([string]$Database, [string]$SqlPath) {
    $repoUnix = To-WslPath $Repo
    $sqlUnix = To-WslPath $SqlPath
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $psql = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/psql"
    Invoke-BashChecked "$ld '$psql' -h 127.0.0.1 -p 55432 -d '$Database' -v ON_ERROR_STOP=1 -Atf '$sqlUnix'"
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

function Get-StateExport([string]$Database, [string]$CompanyID) {
    $query = Get-Content -LiteralPath (Join-Path $Repo 'scripts\r03a-backend-state-query.sql') -Raw
    $query = $query -replace '(?m)^\\set ON_ERROR_STOP on\s*', ''
    $query = $query.Replace('r03a-t2-company-%', $CompanyID.Replace("'", "''"))
    Write-Utf8Text $stateQueryPath $query
    return (Invoke-PgFile $Database $stateQueryPath).Trim()
}

try {
    Set-Location -LiteralPath $Repo
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -ne 0) { throw 'V3 Backend evidence path is not fresh; refusing retry/reset' }
    }
    if (Test-Path -LiteralPath $RuntimeRoot) {
        $runtimeExisting = @(Get-ChildItem -LiteralPath $RuntimeRoot -Force)
        if ($runtimeExisting.Count -ne 0) { throw 'V3 Backend runtime path is not fresh; refusing retry/reset' }
    }
    foreach ($path in @($AuthSource, $SelectedConfig, (Join-Path $L1Evidence 'result.json'), $BackendExecutionManifest, $BackendL2Live, $AcceptanceQualification, $BlobQualification, $CheckerQualification, $RuntimeManifest, $FrontendExecutionManifest, $FrontendL2Live, $FrontendHandoverProtocol)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "required controlled input is unavailable: $path" }
    }
    New-Item -ItemType Directory -Force -Path $Evidence, $RuntimeRoot, $backendRuntime, $restoreRuntime | Out-Null
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
    $env:POLIS_V3_FRONTEND_RESULT = Join-Path $Evidence 'frontend-result.json'
    $env:POLIS_V3_BACKEND_BINDING = $backendBindingPath
    $env:POLIS_V3_FRONTEND_BINDING = Join-Path $Evidence 'frontend-authorization-binding.json'
    $env:POLIS_V3_POSTGRES_SNAPSHOT = $snapshotPath
    $env:POLIS_V3_POSTGRES_DUMP = (To-WslPath "$Repo\.tools\pg\usr\lib\postgresql\18\bin\pg_dump")
    $env:POLIS_V3_POSTGRES_SNAPSHOT_DSN = ''
    $env:POLIS_V3_RUNTIME_ROOT = $backendRuntime
    $env:POLIS_DSN = ''

    Test-PreservedPostgresManifestMachinery

    Invoke-Checked 'bash' @('scripts/build-r03a-pagination-v3-real-backend-windows.sh')
    Invoke-Checked $Executable @('-backend-preflight')
    Invoke-Checked $Executable @('-backend-freshness')
    $preflight = Get-Content -LiteralPath (Join-Path $Evidence 'preflight.json') -Raw | ConvertFrom-Json
    $freshness = Get-Content -LiteralPath (Join-Path $Evidence 'continuation-freshness.json') -Raw | ConvertFrom-Json
    if ($preflight.status -ne 'preflight_passed' -or $preflight.passed -ne $true -or [int]$preflight.medium_limit -ne 1 -or [int]$preflight.high_limit -ne 0 -or [int]$preflight.tool_call_limit -ne 48 -or $preflight.provider_egress -ne 0) { throw 'Backend-only freshness preflight did not pass exact one-Medium binding' }
    if ($freshness.parent_problem_key -ne 'r03a-real-peer-collaboration-v1' -or $freshness.contract_revision -ne 'r03a-pagination-contract@3' -or $freshness.implementation_binding_contract -ne 'r03a-backend-binding@2' -or $freshness.behavior_verifier -ne 'r03a-pagination-behavior@2' -or $freshness.allowance_created -ne $false -or $freshness.provider_egress -ne 0) { throw 'Backend-only continuation freshness binding is stale or incomplete' }

    $sourceDatabase = 'polis_r0_3a_pagination_v3_backend_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + $PID
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
    if ([string]$backend.backend_real_execution -ne 'PASSED' -or [string]$backend.authoritative_snapshot -ne 'PASSED' -or [int]$backend.medium_started -ne 1 -or [int]$backend.provider_egress -ne 1) { throw 'Backend V3 business execution did not pass the authorized Backend-only boundary' }

    $companyID = (Invoke-Pg $sourceDatabase "SELECT id FROM companies WHERE id LIKE 'r03a-pagination-v3-company-%' ORDER BY id LIMIT 1").Trim()
    if ([string]::IsNullOrEmpty($companyID)) { throw 'Backend authoritative company identity was not found' }
    Write-PreservedPostgresManifest $sourceDatabase $companyID
    Assert-PreservedPostgresManifest
    $sourceState = Get-StateExport $sourceDatabase $companyID
    Write-Utf8Text $authoritativeStatePath ($sourceState + [Environment]::NewLine)
    $sourceStateDigest = Hash-Text $sourceState
    $snapshotManifest = Get-Content -LiteralPath $snapshotManifestPath -Raw | ConvertFrom-Json
    $casManifest = Get-Content -LiteralPath $casManifestPath -Raw | ConvertFrom-Json
    if ((Hash-File $snapshotPath) -ne [string]$backend.snapshot_sha256 -or [string]$snapshotManifest.snapshot_sha256 -ne [string]$backend.snapshot_sha256 -or [string]$casManifest.manifest_digest -ne [string]$backend.cas_manifest_digest) { throw 'Backend authoritative snapshot/CAS digest binding failed' }
    Copy-Item -LiteralPath $snapshotManifestPath -Destination (Join-Path $Evidence 'authoritative-snapshot.json')
    Copy-Item -LiteralPath $casManifestPath -Destination (Join-Path $Evidence 'cas-manifest.json')

    $casDestination = Join-Path $restoreRuntime 'blobs'
    Copy-Item -LiteralPath (Join-Path $backendRuntime 'blobs') -Destination $casDestination -Recurse
    if (-not (Test-CAS $casDestination $casManifest)) { throw 'Backend CAS manifest did not survive controlled staging' }
	if (-not (Test-Path -LiteralPath $preservationManifestPath -PathType Leaf) -or -not (Test-Path -LiteralPath ($preservationManifestPath + '.sha256') -PathType Leaf)) { throw 'PreservedPostgresAccessManifest or its hash is missing before destructive boundary' }
	$manifestDeclaredHash = (Get-Content -LiteralPath ($preservationManifestPath + '.sha256') -Raw).Trim()
	if ([string]::IsNullOrWhiteSpace($manifestDeclaredHash) -or $manifestDeclaredHash -ne (Hash-File $preservationManifestPath)) { throw 'PreservedPostgresAccessManifest hash mismatch before destructive boundary' }
	$recoveryPackageComplete = $true

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'drop', $sourceDatabase)
    $sourceDatabaseCreated = $false
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'stop')
    $pgStarted = $false
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true

    $restoreDatabase = 'polis_r0_3a_pagination_v3_backend_restore_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + $PID
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoreDatabase)
    $restoreDatabaseCreated = $true
    Invoke-PgRestore $restoreDatabase $snapshotPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoreDatabase)
    $schema = (Invoke-Pg $restoreDatabase 'SELECT max(version_id) FROM goose_db_version WHERE is_applied').Trim()
    if ($schema -ne '5') { throw "fresh restore schema mismatch: $schema" }
    $restoredState = Get-StateExport $restoreDatabase $companyID
    Write-Utf8Text $restoredStatePath ($restoredState + [Environment]::NewLine)
    $restoredStateDigest = Hash-Text $restoredState
    if ($restoredStateDigest -ne $sourceStateDigest) { throw "fresh restore state digest mismatch: $sourceStateDigest vs $restoredStateDigest" }
    $restoreComparison = [ordered]@{
        status = 'PASSED'
        comparison_phase = 'before_native_runtime_recovery_open'
        source_state_digest = $sourceStateDigest
        restored_state_digest = $restoredStateDigest
        company_id = $companyID
        schema_version = [int]$schema
        company_mission_task_employee_identities = $true
        contract_revision_chain_preserved = $true
        message_obligation_checkpoint_artifact_workspace_preserved = $true
        event_ordering_preserved = $true
        cas_digest_preserved = (Test-CAS $casDestination $casManifest)
        synthesized_rows = 0
        historical_evidence_modified = $false
    }
    Write-Utf8Json (Join-Path $Evidence 'restore-comparison.json') $restoreComparison

    $env:POLIS_DSN = "host=127.0.0.1 port=55432 dbname=$restoreDatabase user=polis_runtime"
    $env:POLIS_V3_RUNTIME_ROOT = $restoreRuntime
    $env:POLIS_V3_POSTGRES_SNAPSHOT_DSN = ''
    Invoke-Checked $Executable @('-backend-restore-verify')
    $runtimeVerification = Get-Content -LiteralPath (Join-Path $Evidence 'backend-restored-runtime-verification.json') -Raw | ConvertFrom-Json
    if ($runtimeVerification.status -ne 'PASSED' -or $runtimeVerification.old_writer_denied -ne $true -or [int]$runtimeVerification.synthesized_rows -ne 0 -or $runtimeVerification.obligation_state -ne 'pending') { throw 'native restored Backend continuity verification failed' }

    if (Test-Path -LiteralPath $FrontendFreshnessEvidence) {
        $frontendExisting = @(Get-ChildItem -LiteralPath $FrontendFreshnessEvidence -Force)
        if ($frontendExisting.Count -ne 0) { throw 'Frontend freshness evidence path is not fresh; refusing overwrite' }
    }
    Invoke-Checked 'bash' @('scripts/go.sh', 'run', './cmd/polis-r03a-frontend-l2-offline', '-evidence', $FrontendFreshnessEvidence, '-l1-evidence', $L1Evidence, '-base-l2-evidence', (Split-Path -Parent $BackendExecutionManifest), '-handover-protocol', $FrontendHandoverProtocol)
    $freshFrontendManifest = Get-Content -LiteralPath (Join-Path $FrontendFreshnessEvidence 'execution-manifest.json') -Raw | ConvertFrom-Json
    $existingFrontendManifest = Get-Content -LiteralPath $FrontendExecutionManifest -Raw | ConvertFrom-Json
    $frontendSurfaceEqual = ([int]$freshFrontendManifest.tool_surface.tool_count -eq [int]$existingFrontendManifest.tool_surface.tool_count -and [string]$freshFrontendManifest.tool_surface.aggregate_manifest_digest -eq [string]$existingFrontendManifest.tool_surface.aggregate_manifest_digest -and [string]$freshFrontendManifest.tool_surface.aggregate_schema_digest -eq [string]$existingFrontendManifest.tool_surface.aggregate_schema_digest -and [int]$freshFrontendManifest.tool_surface.aggregate_schema_bytes -eq [int]$existingFrontendManifest.tool_surface.aggregate_schema_bytes -and [string]$freshFrontendManifest.tool_surface.thread_start_payload_digest -eq [string]$existingFrontendManifest.tool_surface.thread_start_payload_digest -and [int]$freshFrontendManifest.tool_surface.thread_start_payload_bytes -eq [int]$existingFrontendManifest.tool_surface.thread_start_payload_bytes -and [string]$freshFrontendManifest.handler_binding_digest -eq [string]$existingFrontendManifest.handler_binding_digest)
    $frontendFreshness = [ordered]@{
        status = if ($frontendSurfaceEqual) { 'CURRENT_SURFACE_EQUAL_REUSE_L2' } else { 'BLOCKED_PENDING_FRONTEND_L2' }
        provider_visible_surface_equal = $frontendSurfaceEqual
        existing_manifest = $FrontendExecutionManifest
        fresh_manifest = (Join-Path $FrontendFreshnessEvidence 'execution-manifest.json')
        existing_tool_manifest_digest = [string]$existingFrontendManifest.tool_surface.aggregate_manifest_digest
        fresh_tool_manifest_digest = [string]$freshFrontendManifest.tool_surface.aggregate_manifest_digest
        existing_schema_digest = [string]$existingFrontendManifest.tool_surface.aggregate_schema_digest
        fresh_schema_digest = [string]$freshFrontendManifest.tool_surface.aggregate_schema_digest
        historical_evidence_modified = $false
    }
    Write-Utf8Json (Join-Path $Evidence 'frontend-manifest-freshness.json') $frontendFreshness

    $restoreProof = [ordered]@{
        status = 'PASSED'
        source_database_destroyed_before_restore = $true
        source_state_digest = $sourceStateDigest
        restored_state_digest = $restoredStateDigest
        snapshot_path = $snapshotPath
        snapshot_sha256 = [string]$backend.snapshot_sha256
        snapshot_manifest = (Join-Path $Evidence 'authoritative-snapshot.json')
        cas_manifest = (Join-Path $Evidence 'cas-manifest.json')
        cas_manifest_digest = [string]$casManifest.manifest_digest
        runtime_verification = (Join-Path $Evidence 'backend-restored-runtime-verification.json')
        old_writer_denied = $true
        synthesized_rows = 0
        historical_evidence_modified = $false
    }
    Write-Utf8Json $restoreProofPath $restoreProof
    Copy-Item -LiteralPath $restoreProofPath -Destination (Join-Path $Evidence 'restore-proof.json')

    $overall = [ordered]@{
        qualification = 'R0.3A-PAGINATION-V3-REAL-BACKEND'
        status = 'BACKEND_PASSED_FRONTEND_PENDING'
        parent_problem_key = 'r03a-real-peer-collaboration-v1'
        subject_revision = 'r03a-pagination-v3-business-subject@1'
        public_contract_revision = 'r03a-pagination-contract@3'
        runtime_verifier_revision = 'r03a-pagination-behavior@2'
        allowance = (Get-Content -LiteralPath $allowancePath -Raw | ConvertFrom-Json)
        tool_call_limit = 48
        runtime_safety_hard_cap = 256
        backend_result = $backend
        backend_medium_started = [int]$backend.medium_started
        backend_provider_egress = [int]$backend.provider_egress
        backend_tool_calls_used = [int]$backend.tool_calls_used
        backend_tool_calls_remaining = [int]$backend.tool_calls_remaining
        authoritative_state_continuity = 'PASSED'
        restore_proof = $restoreProof
        frontend_started = $false
        frontend_manifest_freshness = $frontendFreshness
        eligible_for_frontend_phase = [bool]$frontendSurfaceEqual
        historical_evidence_modified = $false
    }
    Write-Utf8Json $resultPath $overall
} catch {
    $failedBackend = $null
    if (Test-Path -LiteralPath $backendResultPath) { try { $failedBackend = Get-Content -LiteralPath $backendResultPath -Raw | ConvertFrom-Json } catch {} }
    $businessFailed = $null -ne $failedBackend -and [int]$failedBackend.provider_egress -gt 0 -and [string]$failedBackend.backend_real_execution -eq 'FAILED'
    $failure = [ordered]@{
        qualification = 'R0.3A-PAGINATION-V3-REAL-BACKEND'
        status = if ($businessFailed) { 'FAILED' } else { 'INCONCLUSIVE' }
        error = $_.Exception.Message
        parent_problem_key = 'r03a-real-peer-collaboration-v1'
        public_contract_revision = 'r03a-pagination-contract@3'
        runtime_verifier_revision = 'r03a-pagination-behavior@2'
        backend_result = $failedBackend
        source_database_name = $sourceDatabase
        source_database_preserved = [bool]($sourceDatabaseCreated -and -not $recoveryPackageComplete)
        recovery_package_complete = [bool]$recoveryPackageComplete
        frontend_started = $false
        historical_evidence_modified = $false
    }
    try { Write-Utf8Json $resultPath $failure } catch {}
    throw
} finally {
    if ($restoreDatabaseCreated -and $restoreDatabase -ne '') { try { & bash scripts/r03a-real-backend-pg.sh drop $restoreDatabase } catch { Write-Warning "restore DB cleanup failed: $($_.Exception.Message)" } }
    if ($sourceDatabaseCreated -and $sourceDatabase -ne '' -and $recoveryPackageComplete) { try { & bash scripts/r03a-real-backend-pg.sh drop $sourceDatabase } catch { Write-Warning "source DB cleanup failed: $($_.Exception.Message)" } }
    if ($sourceDatabaseCreated -and $sourceDatabase -ne '' -and -not $recoveryPackageComplete) { Write-Warning "preserving source DB after incomplete recovery package: $sourceDatabase" }
    if ($pgStarted) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "PG cleanup failed: $($_.Exception.Message)" } }
    Set-Location -LiteralPath $oldLocation
}
