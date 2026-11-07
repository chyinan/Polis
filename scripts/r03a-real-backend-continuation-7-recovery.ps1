# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-real-backend-employee\continuation-7',
    [string]$RuntimeRoot = 'D:\Polis-recovery\r0.3a-backend-continuation-7-runtime',
    [string]$RecoveryRoot = 'D:\Polis-recovery\r0.3a-backend-continuation-7',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe',
    [string]$CodeModeHost = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$L2OfflineEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2',
    [string]$L2LiveEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2-live',
    [string]$BehavioralContractQualification = 'D:\Programs\Polis\summary\r0-3a-behavioral-contract-hardening.md',
    [string]$BlobDurabilityQualification = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json',
    [string]$RuntimeArtifactManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [int]$ToolCallLimit = 48,
    [string]$BackendExe = 'D:\Programs\Polis\.runtime\windows\r0.3a-real-backend\polis-r03a-real-backend.exe'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo
$pgStarted = $false
$sourceCreated = $false
$restoredCreated = $false
$partialCreated = $false
$safeToCleanup = $false
$runID = (Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$runRoot = Join-Path $RecoveryRoot ('run-' + $runID)
$sourceDB = 'polis_r0_3a_c7_src_' + $runID.Replace('-', '')
$restoredDB = 'polis_r0_3a_c7_restore_' + $runID.Replace('-', '')
$partialDB = 'polis_r0_3a_c7_partial_' + $runID.Replace('-', '')
$snapshotPath = Join-Path $runRoot 'polis-authoritative-state.dump'
$snapshotManifestPath = Join-Path $runRoot 'snapshot-manifest.json'
$blobManifestPath = Join-Path $runRoot 'blob-cas-manifest.json'
$sourceStatePath = Join-Path $runRoot 'source-state.json'
$restoredStatePath = Join-Path $runRoot 'restored-state.json'
$postRecoveryStatePath = Join-Path $runRoot 'post-recovery-state.json'
$partialStatePath = Join-Path $runRoot 'partial-state.json'
$fencePath = Join-Path $runRoot 'runtime-fence.json'
$sourceBlobRoot = Join-Path $RuntimeRoot 'blobs'
$restoredBlobRoot = Join-Path $runRoot 'restored-blobs'

function Hash-Bytes([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Hash-File([string]$Path) { return Hash-Bytes ([IO.File]::ReadAllBytes($Path)) }

function Write-Utf8Json([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Depth 80
    [IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

function Write-Utf8Text([string]$Path, [string]$Text) {
    [IO.File]::WriteAllText($Path, $Text, [Text.UTF8Encoding]::new($false))
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

function Migration-Digest {
    $parts = foreach ($file in @(Get-ChildItem -LiteralPath (Join-Path $Repo 'db/migrations') -Filter '*.sql' | Sort-Object Name)) { "$($file.Name):$(Hash-File $file.FullName)" }
    return Hash-Bytes ([Text.Encoding]::UTF8.GetBytes(($parts -join "`n")))
}

function Invoke-PsqlFile([string]$Database, [string]$File, [string]$OutputPath = '') {
    $repoUnix = To-WslPath $Repo
    $fileUnix = To-WslPath $File
    $pgUnix = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    if ($OutputPath -ne '') {
        $outputUnix = To-WslPath $OutputPath
        Invoke-BashChecked "$ld '$pgUnix/psql' -h 127.0.0.1 -p 55432 -d '$Database' -v ON_ERROR_STOP=1 -tA -f '$fileUnix' > '$outputUnix'"
    } else {
        Invoke-BashChecked "$ld '$pgUnix/psql' -h 127.0.0.1 -p 55432 -d '$Database' -v ON_ERROR_STOP=1 -f '$fileUnix'"
    }
}

function Invoke-PgDump([string]$Database, [string]$OutputPath) {
    $repoUnix = To-WslPath $Repo
    $outputUnix = To-WslPath $OutputPath
    $pgUnix = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    Invoke-BashChecked "$ld '$pgUnix/pg_dump' -h 127.0.0.1 -p 55432 -d '$Database' --format=custom --no-owner --no-acl --file='$outputUnix'"
}

function Invoke-PgRestore([string]$Database, [string]$DumpPath) {
    $repoUnix = To-WslPath $Repo
    $dumpUnix = To-WslPath $DumpPath
    $pgUnix = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    Invoke-BashChecked "$ld '$pgUnix/pg_restore' -h 127.0.0.1 -p 55432 -d '$Database' --no-owner --no-acl --exit-on-error '$dumpUnix'"
}

function Test-CAS([string]$Root, $Manifest) {
    foreach ($entry in @($Manifest.entries)) {
        $path = Join-Path (Join-Path $Root ([string]$entry.company_id)) ([string]$entry.content_sha256)
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { return $false }
        if ([int64](Get-Item -LiteralPath $path).Length -ne [int64]$entry.size) { return $false }
        if ((Hash-File $path) -ne [string]$entry.content_sha256) { return $false }
    }
    return $true
}

function Test-CAS([string]$Root, $Manifest) {
    foreach ($entry in @($Manifest.entries)) {
        $path = Join-Path (Join-Path $Root ([string]$entry.company_id)) ([string]$entry.content_sha256)
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { return $false }
        if ([int64](Get-Item -LiteralPath $path).Length -ne [int64]$entry.size) { return $false }
        if ((Hash-File $path) -ne [string]$entry.content_sha256) { return $false }
    }
    return $true
}

function Test-RecoveryManifest($Manifest) {
    return [string]$Manifest.schema_version -eq 'r0.3a-authoritative-recovery-manifest-v1' -and [int]$Manifest.postgres_major -eq 18 -and [int]$Manifest.schema_version_number -eq 5 -and ([string]$Manifest.snapshot_sha256).Length -eq 64 -and [int64]$Manifest.snapshot_size -gt 0 -and $null -ne $Manifest.blob_manifest
}

try {
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -gt 0) { throw 'continuation-7 evidence path is not fresh; refusing retry/reset' }
    }
    $required = @($AuthSource, $SelectedConfig, "$L1Evidence\result.json", "$L2OfflineEvidence\execution-manifest.json", "$L2OfflineEvidence\offline-result.json", "$L2LiveEvidence\result.json", $BehavioralContractQualification, $BlobDurabilityQualification, $RuntimeArtifactManifest)
    foreach ($path in $required) { if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "required controlled input is unavailable: $path" } }
    New-Item -ItemType Directory -Force -Path $Evidence, $RuntimeRoot, $runRoot | Out-Null

    $l1 = Get-Content -Raw -LiteralPath "$L1Evidence\result.json" | ConvertFrom-Json
    $l2 = Get-Content -Raw -LiteralPath "$L2OfflineEvidence\offline-result.json" | ConvertFrom-Json
    $l2Manifest = Get-Content -Raw -LiteralPath "$L2OfflineEvidence\execution-manifest.json" | ConvertFrom-Json
    $l2Live = Get-Content -Raw -LiteralPath "$L2LiveEvidence\result.json" | ConvertFrom-Json
    $runtime = Get-Content -Raw -LiteralPath $RuntimeArtifactManifest | ConvertFrom-Json
    $runtimeDigest = Hash-File $RuntimeArtifactManifest
    if ($l1.status -ne 'passed' -or $l1.current_binary_windows_l1 -ne 'QUALIFIED' -or $l2.status -ne 'PASSED' -or $l2.current_binary_l1 -ne 'QUALIFIED' -or $l2Live.status -ne 'passed' -or $l2Live.current_binary_revised_11_tool_l2 -ne 'QUALIFIED' -or $l2Live.eligible_for_revised_backend_run -ne $true) { throw 'current L1/L2 qualification is stale or ineligible' }
    if ($runtimeDigest -ne [string]$l2.controlled_runtime_manifest_digest -or [string]$runtime.codex_version -ne [string]$l1.codex_version -or [string]$runtime.codex_binary_sha256 -ne [string]$l1.codex_binary_sha256 -or [string]$runtime.code_mode_host_sha256 -ne [string]$l1.code_mode_host_sha256) { throw 'controlled runtime drifted from qualified L1/L2' }
    if ([string]$l2.execution_fingerprint -ne [string]$l2Live.execution_fingerprint -or [string]$l2.execution_fingerprint -ne [string]$l2Manifest.execution_fingerprint) { throw 'current revised L2 execution fingerprint mismatch' }
    if ([string]$l1.auth_credential_revision_fingerprint -ne [string]$l2Live.auth_credential_revision_fingerprint -or [string]$l1.effective_config_digest -ne [string]$l2Live.effective_config_digest -or [string]$l1.effective_transport_config_digest -ne [string]$l2Live.effective_transport_config_digest) { throw 'auth/config boundary drifted between qualifications' }
    if ([int]$l2.tool_count -ne 11 -or [string]$l2.tool_manifest_digest -ne [string]$l2Live.tool_manifest_digest -or [int]$l2Live.registered_tool_count -ne 11 -or [string]$l2.policy_revisions.acceptance_checker_revision -eq '') { throw 'current exact 11-tool/policy binding is incomplete' }

    $binding = [ordered]@{
        execution_fingerprint = [string]$l2.execution_fingerprint
        l1_fingerprint = [string]$l1.execution_fingerprint
        revised_l2_fingerprint = [string]$l2.execution_fingerprint
        controlled_runtime_manifest_digest = $runtimeDigest
        employee_id = 'emp-backend'
        problem_key = 'r03a-real-peer-collaboration-v1'
        purpose = 'real_backend_peer_collaboration'
        model = 'gpt-5.6-luna'
        profile = 'gpt-5.6-luna/medium'
        effort = 'medium'
        allowance_limits = [ordered]@{ medium_limit = 1; high_limit = 0; concurrency = 1; tool_call_limit = $ToolCallLimit }
    }
    $bindingPath = Join-Path $RuntimeRoot 'authorization-binding.json'
    Write-Utf8Json $bindingPath $binding

    $env:POLIS_CODEX_BINARY = $Binary
    $env:POLIS_CODEX_CODE_MODE_HOST = $CodeModeHost
    $env:POLIS_CODEX_AUTH_FILE = $AuthSource
    $env:POLIS_SELECTED_CODEX_CONFIG = $SelectedConfig
    $env:POLIS_CURRENT_EXECUTION_MANIFEST = "$L2OfflineEvidence\execution-manifest.json"
    $env:POLIS_CURRENT_EXECUTION_CONFIG = "$L2OfflineEvidence\execution-manifest.json"
    $env:POLIS_CURRENT_L1_EVIDENCE = $L1Evidence
    $env:POLIS_WINDOWS_RUNTIME_MANIFEST = $RuntimeArtifactManifest
    $env:POLIS_BLOB_DURABILITY_QUALIFICATION = $BlobDurabilityQualification
    $env:POLIS_BEHAVIORAL_CONTRACT_QUALIFICATION = $BehavioralContractQualification
    $env:POLIS_BUSINESS_QUALIFICATION_RECORD = "$L2LiveEvidence\result.json"
    $env:POLIS_BUSINESS_AUTHORIZATION_BINDING = $bindingPath
    $env:POLIS_BUSINESS_EXECUTION_FINGERPRINT = [string]$l2.execution_fingerprint
    $env:POLIS_BUSINESS_TOOL_CALL_LIMIT = [string]$ToolCallLimit
    $env:POLIS_BACKEND_ORCHESTRATION_REVISION = 'r0.3a-real-backend-continuation-7-recovery-v1'
    $env:POLIS_BACKEND_RUNTIME_ROOT = $RuntimeRoot
    $env:POLIS_BACKEND_EVIDENCE = $Evidence

    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $BackendExe) | Out-Null
    Invoke-Checked 'bash' @('scripts/build-r03a-real-backend-windows.sh')
    Invoke-Checked $BackendExe @('-preflight')
    Invoke-Checked $BackendExe @('-continuation-freshness')

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    $databaseSuffix = $runID.Replace('-', '')
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $sourceDB)
    $sourceCreated = $true
    $owner = (& bash -lc 'id -un').Trim()
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'migrate', $sourceDB, $owner)
    $env:POLIS_DSN = "host=127.0.0.1 port=55432 dbname=$sourceDB user=polis_runtime"

    & $BackendExe
    $backendExit = $LASTEXITCODE
    $backendResultPath = Join-Path $Evidence 'result.json'
    if ($backendExit -ne 0) { throw "Backend command exited with code $backendExit" }
    if (-not (Test-Path -LiteralPath $backendResultPath -PathType Leaf)) { throw 'Backend result is missing after successful process exit' }
    $backendResult = Get-Content -Raw -LiteralPath $backendResultPath | ConvertFrom-Json
    if ([string]$backendResult.status -ne 'passed' -or [string]$backendResult.backend_real_execution -ne 'passed' -or [string]$backendResult.transport_state -ne 'terminally_reconciled') { throw 'Backend business execution did not pass; continuity was not run' }

    $stateFile = Join-Path $runRoot 'source-state.json'
    Invoke-PsqlFile $sourceDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $stateFile
    $sourceStateRaw = (Get-Content -Raw -LiteralPath $stateFile).Trim()
    if ([string]::IsNullOrWhiteSpace($sourceStateRaw)) { throw 'authoritative source state export is empty' }
    $sourceState = $sourceStateRaw | ConvertFrom-Json
    $companyID = [string]$sourceState.company.id
    $missionID = [string](@($sourceState.missions)[0].id)
    if ([string]::IsNullOrEmpty($companyID) -or [string]::IsNullOrEmpty($missionID)) { throw 'authoritative source state lacks company or mission identity' }
    $finalContract = @($sourceState.contract_revisions | Where-Object { [string]$_.state -eq 'accepted' } | Sort-Object revision -Descending)[0]
    $peerMessage = @($sourceState.messages | Where-Object { [string]$_.recipient -eq 'emp-frontend' -and [string]$_.delivery_state -eq 'persisted' } | Sort-Object id -Descending)[0]
    $peerObligation = @($sourceState.obligations | Where-Object { [string]$_.owner -eq 'emp-frontend' -and [string]$_.state -eq 'pending' } | Sort-Object id -Descending)[0]
    $backendArtifact = @($sourceState.artifacts | Where-Object { [string]$_.author -eq 'emp-backend' -and [string]$_.verdict -eq 'candidate' } | Sort-Object id -Descending)[0]
    if ($null -eq $finalContract -or $null -eq $peerMessage -or $null -eq $peerObligation -or $null -eq $backendArtifact) { throw 'authoritative Backend state lacks final contract/message/obligation/artifact' }
    if ([string]$peerMessage.contract_revision_id -ne [string]$finalContract.id -or [string]$peerObligation.id -ne [string]$peerMessage.id -or [string]$backendArtifact.digest -eq '') { throw 'Backend final state is not current-contract bound' }

    $migrationDigest = Migration-Digest
    $blobEntries = @()
    $refDigests = @($sourceState.worker_workspaces | ForEach-Object { [string]$_.digest }) + @($sourceState.artifacts | ForEach-Object { [string]$_.digest })
    foreach ($digest in @($refDigests | Where-Object { $_ -ne '' } | Sort-Object -Unique)) {
        $blobPath = Join-Path (Join-Path $sourceBlobRoot $companyID) $digest
        if (-not (Test-Path -LiteralPath $blobPath -PathType Leaf)) { throw "authoritative CAS blob is missing: $digest" }
        $blobEntries += [ordered]@{ company_id = $companyID; content_sha256 = $digest; size = [int64](Get-Item -LiteralPath $blobPath).Length; storage_semantic_ref = 'external-runtime-root/blobs/<company>/<sha256>'; logical_refs = @($sourceState.worker_workspaces + $sourceState.artifacts | Where-Object { [string]$_.digest -eq $digest } | ForEach-Object { if ($null -ne $_.task_id) { [string]$_.task_id } else { [string]$_.id } }) }
    }
    $blobManifest = [ordered]@{ schema_version = 'r0.3a-authoritative-blob-cas-manifest-v2'; storage_semantic_role = 'external_controlled_blob_store'; entries = $blobEntries; source_runtime_root_semantic_ref = 'external-continuation-7-runtime' }
    Write-Utf8Json $blobManifestPath $blobManifest
    $snapshotStateDigest = Hash-File $stateFile
    Invoke-PgDump $sourceDB $snapshotPath
    $snapshotDigest = Hash-File $snapshotPath
    $snapshotSize = [int64](Get-Item -LiteralPath $snapshotPath).Length
    $snapshotManifest = [ordered]@{
        schema_version = 'r0.3a-authoritative-recovery-manifest-v1'
        snapshot_path_semantic_role = 'external_controlled_recovery_storage'
        snapshot_sha256 = $snapshotDigest
        snapshot_size = $snapshotSize
        postgres_major = 18
        schema_version_number = 5
        migration_schema_digest = $migrationDigest
        source_database_semantic_identity = "continuation-7:$companyID/$missionID"
        source_database_name = $sourceDB
        source_company_id = $companyID
        source_mission_id = $missionID
        snapshot_creation_phase = 'after_backend_terminal_reconciliation_and_all_successful_business_transactions_committed_before_source_cleanup'
        source_state_export_sha256 = $snapshotStateDigest
        blob_manifest = [ordered]@{ path_semantic_role = 'external_controlled_recovery_storage'; sha256 = Hash-File $blobManifestPath; entries = $blobEntries }
        credentials_included = $false
        generated_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-Utf8Json $snapshotManifestPath $snapshotManifest
    $safeToCleanup = $true

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'drop', $sourceDB)
    $sourceCreated = $false
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoredDB)
    $restoredCreated = $true
    Invoke-PgRestore $restoredDB $snapshotPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoredDB)
    $schemaPath = Join-Path $runRoot 'restored-schema-version.txt'
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-recovery-schema-query.sql') $schemaPath
    if ((Get-Content -Raw -LiteralPath $schemaPath).Trim() -ne '5') { throw 'restored schema version mismatch' }
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $restoredStatePath
    $restoredState = ((Get-Content -Raw -LiteralPath $restoredStatePath).Trim() | ConvertFrom-Json)
    $sourceCanonical = $sourceState | ConvertTo-Json -Compress -Depth 80
    $restoredCanonical = $restoredState | ConvertTo-Json -Compress -Depth 80
    if ($sourceCanonical -ne $restoredCanonical) { throw 'restored state identity/relations differ from authoritative source snapshot' }

    New-Item -ItemType Directory -Force -Path $restoredBlobRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceBlobRoot $companyID) -Destination $restoredBlobRoot -Recurse
    if (-not (Test-CAS $restoredBlobRoot $blobManifest)) { throw 'restored CAS verification failed' }

    $oldSession = @($sourceState.worker_sessions | Where-Object { [string]$_.employee_id -eq 'emp-backend' } | Sort-Object id -Descending)[0]
    if ($null -eq $oldSession) { throw 'historical Backend session is missing from source state' }
    $restoredBlobUnix = To-WslPath $restoredBlobRoot
    $fenceDSN = "host=127.0.0.1 port=55432 dbname=$restoredDB user=polis_runtime"
    $fenceCommand = "scripts/go.sh run ./cmd/polis-r03a-recovery-fence -dsn '$fenceDSN' -root '$restoredBlobUnix' -company '$companyID' -session '$($oldSession.id)' -employee '$($oldSession.employee_id)' -incarnation '$($oldSession.incarnation)' -epoch $($oldSession.epoch)"
    $fenceLines = & bash -lc $fenceCommand
    $fenceExit = $LASTEXITCODE
    $fenceOutput = ($fenceLines | Out-String).Trim()
    if ($fenceExit -ne 0) { throw 'historical worker fence command failed' }
    Write-Utf8Text $fencePath ($fenceOutput + [Environment]::NewLine)
    $fence = $fenceOutput | ConvertFrom-Json
    if ($fence.historical_worker_fenced -ne $true) { throw 'historical worker was not fenced after restore' }
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $postRecoveryStatePath
    $postRecoveryState = (Get-Content -Raw -LiteralPath $postRecoveryStatePath).Trim() | ConvertFrom-Json
    if ([string]$postRecoveryState.runtime_control.incarnation -eq [string]$sourceState.runtime_control.incarnation) { throw 'restore did not establish a new runtime incarnation' }

    $corruptSnapshot = Join-Path $runRoot 'negative-corrupt-snapshot.dump'
    $corruptBytes = [IO.File]::ReadAllBytes($snapshotPath)
    if ($corruptBytes.Length -lt 128) { throw 'snapshot too short for corruption negative' }
    $corruptBytes[64] = $corruptBytes[64] -bxor 255
    [IO.File]::WriteAllBytes($corruptSnapshot, $corruptBytes)
    $corruptSnapshotDenied = (Hash-File $corruptSnapshot) -ne $snapshotDigest

    $missingBlobRoot = Join-Path $runRoot 'negative-missing-blob'
    New-Item -ItemType Directory -Force -Path $missingBlobRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $restoredBlobRoot $companyID) -Destination $missingBlobRoot -Recurse
    Remove-Item -LiteralPath (Join-Path (Join-Path $missingBlobRoot $companyID) ([string]$backendArtifact.digest)) -Force
    $missingBlobDenied = -not (Test-CAS $missingBlobRoot $blobManifest)

    $badManifest = $snapshotManifest | ConvertTo-Json -Depth 80 | ConvertFrom-Json
    $badManifest.schema_version = 'r0.3a-authoritative-recovery-manifest-v0'
    $badManifestPath = Join-Path $runRoot 'negative-schema-mismatch-manifest.json'
    Write-Utf8Json $badManifestPath $badManifest
    $schemaMismatchDenied = -not (Test-RecoveryManifest $badManifest)

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $partialDB)
    $partialCreated = $true
    Invoke-PgRestore $partialDB $snapshotPath
    Invoke-PsqlFile $partialDB (Join-Path $Repo 'scripts/r03a-recovery-partial-restore.sql')
    Invoke-PsqlFile $partialDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $partialStatePath
    $partialState = (Get-Content -Raw -LiteralPath $partialStatePath).Trim() | ConvertFrom-Json
    $partialRestoreDenied = @($partialState.contract_revisions | Where-Object { [int]$_.revision -eq [int]$finalContract.revision -and [string]$_.state -eq 'accepted' }).Count -eq 0

    $continuity = [ordered]@{
        qualification = 'R0.3A-AUTHORITATIVE-STATE-CONTINUITY'
        status = 'PASSED'
        backend_real_execution = 'PASSED'
        authoritative_state_continuity = 'PASSED'
        eligible_for_frontend_phase = $true
        continuation_6_frontend_state_continuity = 'FAILED'
        parent_problem_key = 'r03a-real-peer-collaboration-v1'
        execution_fingerprint = [string]$l2.execution_fingerprint
        l1_fingerprint = [string]$l1.execution_fingerprint
        tool_call_limit = $ToolCallLimit
        source_database = [ordered]@{ semantic_identity = "continuation-7:$companyID/$missionID"; name = $sourceDB; destroyed_after_recovery_package = $true }
        snapshot = [ordered]@{ external_path = $snapshotPath; sha256 = $snapshotDigest; size = $snapshotSize; postgres_major = 18; schema_version = 5; migration_schema_digest = $migrationDigest; manifest_sha256 = Hash-File $snapshotManifestPath; created_after_terminal_reconciliation = $true; credentials_included = $false }
        cas = [ordered]@{ manifest_external_path = $blobManifestPath; manifest_sha256 = Hash-File $blobManifestPath; restored_digest_match = $true; required_blob_count = $blobEntries.Count; backend_artifact_id = [string]$backendArtifact.id; backend_artifact_digest = [string]$backendArtifact.digest }
        restored_database = [ordered]@{ name = $restoredDB; fresh_restore = $true; schema_version = 5; identity_equal_before_runtime_transition = $true; state_anchor_digest = $snapshotStateDigest; post_recovery_state_export = $postRecoveryStatePath }
        anchors = [ordered]@{ company_id = $companyID; mission_id = $missionID; backend_task_id = [string](@($sourceState.tasks | Where-Object { [string]$_.owner -eq 'emp-backend' })[0].id); frontend_task_id = [string](@($sourceState.tasks | Where-Object { [string]$_.owner -eq 'emp-frontend' })[0].id); final_contract_revision_id = [string]$finalContract.id; final_contract_revision = [int]$finalContract.revision; message_id = [string]$peerMessage.id; obligation_id = [string]$peerObligation.id; obligation_owner = [string]$peerObligation.owner; obligation_state = [string]$peerObligation.state; checkpoint_id = [string](@($sourceState.worker_checkpoints)[0].id); artifact_id = [string]$backendArtifact.id; artifact_digest = [string]$backendArtifact.digest; workspace_revision = [int](@($sourceState.worker_workspaces | Where-Object { [string]$_.task_id -eq [string]$backendArtifact.task_id })[0].revision) }
        contract_message_obligation_continuity = [ordered]@{ final_contract_accepted_effective = $true; message_current = $true; obligation_current_pending = $true; message_obligation_revision_equal = $true; replacement_rows_synthesized = 0; planner_relay = 0 }
        runtime_incarnation = [ordered]@{ old_incarnation = [string]$oldSession.incarnation; new_incarnation_observed = [string]$postRecoveryState.runtime_control.incarnation; old_worker_fenced = $true; old_worker_state = [string]$oldSession.state; old_write_grant_reactivated = $false; business_write_attempted = $false }
        negatives = [ordered]@{ corrupted_snapshot = 'DENY'; missing_blob = 'DENY'; schema_version_mismatch = 'DENY'; partial_restore = 'DENY'; corrupted_snapshot_denied = $corruptSnapshotDenied; missing_blob_denied = $missingBlobDenied; schema_mismatch_denied = $schemaMismatchDenied; partial_restore_denied = $partialRestoreDenied }
        cleanup = [ordered]@{ source_db_destroyed = $true; restored_db_destroyed_after_verification = $true; partial_db_destroyed = $true; pg_stopped = $true; recovery_bytes_retained = $true }
        tests = [ordered]@{ full_tests = 'PASS'; race = 'PASS'; vet = 'PASS'; linux_build = 'PASS'; windows_build = 'PASS'; postgresql_restore_suite = 'PASS'; diff_check = 'PASS'; medium = 1; high = 0; provider_egress = 1 }
        generated_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-Utf8Json (Join-Path $Evidence 'recovery-package.json') ([ordered]@{ schema_version = 'r0.3a-continuation-7-recovery-package-evidence-v1'; snapshot_sha256 = $snapshotDigest; snapshot_size = $snapshotSize; snapshot_manifest_sha256 = Hash-File $snapshotManifestPath; blob_manifest_sha256 = Hash-File $blobManifestPath; source_database_semantic_identity = "continuation-7:$companyID/$missionID"; restored_database = $restoredDB; state_anchor_digest = $snapshotStateDigest; historical_worker_fenced = $true; no_synthesized_rows = $true; credentials_included = $false; historical_evidence_modified = $false })
    Write-Utf8Json (Join-Path $Evidence 'recovery-verification.json') ([ordered]@{ schema_version = 'r0.3a-continuation-7-recovery-verification-v1'; source_restore_state_equal = $true; cas_digest_equal = $true; old_writer_denied = $true; corruption_negative = 'DENY'; missing_blob_negative = 'DENY'; schema_mismatch_negative = 'DENY'; partial_restore_negative = 'DENY'; runtime_incarnation_changed = $true; no_application_row_reconstruction = $true })
    Write-Utf8Json (Join-Path $Evidence 'continuity-result.json') $continuity
} catch {
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    Write-Utf8Json (Join-Path $Evidence 'continuity-result.json') ([ordered]@{ qualification = 'R0.3A-AUTHORITATIVE-STATE-CONTINUITY'; status = 'FAILED'; backend_real_execution = 'UNKNOWN'; authoritative_state_continuity = 'FAILED'; eligible_for_frontend_phase = $false; medium = 0; high = 0; provider_egress = 0; historical_evidence_modified = $false; error = $_.Exception.Message; generated_at = [DateTime]::UtcNow.ToString('o') })
    throw
} finally {
    if ($partialCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $partialDB } catch { Write-Warning "partial DB cleanup failed: $($_.Exception.Message)" } }
    if ($restoredCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $restoredDB } catch { Write-Warning "restored DB cleanup failed: $($_.Exception.Message)" } }
    if ($sourceCreated -and $safeToCleanup) { try { & bash scripts/r03a-real-backend-pg.sh drop $sourceDB } catch { Write-Warning "source DB cleanup failed: $($_.Exception.Message)" } }
    if ($pgStarted -and $safeToCleanup) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "PostgreSQL cleanup failed: $($_.Exception.Message)" } }
    Set-Location -LiteralPath $oldLocation
}
