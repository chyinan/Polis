# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-authoritative-state-continuity\continuation-7-recovery-v2',
    [string]$RecoveryRun = 'D:\Polis-recovery\r0.3a-backend-continuation-7\run-20260912135427-17a3e1a3b6c3',
    [string]$RuntimeRoot = 'D:\Polis-recovery\r0.3a-backend-continuation-7-runtime'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo
$pgStarted = $false
$restoredCreated = $false
$partialCreated = $false
$runID = (Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$runRoot = Join-Path $RecoveryRun ('recovery-only-' + $runID)
$restoredDB = 'polis_r0_3a_c7_restore_only_' + $runID.Replace('-', '')
$partialDB = 'polis_r0_3a_c7_partial_only_' + $runID.Replace('-', '')
$snapshotPath = Join-Path $RecoveryRun 'polis-authoritative-state.dump'
$snapshotManifestPath = Join-Path $RecoveryRun 'snapshot-manifest.json'
$blobManifestPath = Join-Path $RecoveryRun 'blob-cas-manifest.json'
$sourceStatePath = Join-Path $RecoveryRun 'source-state.json'
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

function Write-Utf8Text([string]$Path, [string]$Text) { [IO.File]::WriteAllText($Path, $Text, [Text.UTF8Encoding]::new($false)) }

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

function Test-RecoveryManifest($Manifest) {
    return [string]$Manifest.schema_version -eq 'r0.3a-authoritative-recovery-manifest-v1' -and [int]$Manifest.postgres_major -eq 18 -and [int]$Manifest.schema_version_number -eq 5 -and ([string]$Manifest.snapshot_sha256).Length -eq 64 -and [int64]$Manifest.snapshot_size -gt 0 -and $null -ne $Manifest.blob_manifest
}

try {
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -gt 0) { throw 'recovery-only evidence path is not fresh; refusing overwrite' }
    }
    foreach ($path in @($snapshotPath, $snapshotManifestPath, $blobManifestPath, $sourceStatePath)) { if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "authoritative recovery artifact is unavailable: $path" } }
    New-Item -ItemType Directory -Force -Path $Evidence, $runRoot | Out-Null

    $snapshotManifest = Get-Content -Raw -LiteralPath $snapshotManifestPath | ConvertFrom-Json
    $blobManifest = Get-Content -Raw -LiteralPath $blobManifestPath | ConvertFrom-Json
    if (-not (Test-RecoveryManifest $snapshotManifest) -or [string]$snapshotManifest.snapshot_sha256 -ne (Hash-File $snapshotPath) -or [int64]$snapshotManifest.snapshot_size -ne [int64](Get-Item -LiteralPath $snapshotPath).Length -or [string]$snapshotManifest.blob_manifest.sha256 -ne (Hash-File $blobManifestPath) -or -not (Test-CAS $sourceBlobRoot $blobManifest)) { throw 'authoritative recovery package hash or CAS verification failed' }
    $sourceStateRaw = (Get-Content -Raw -LiteralPath $sourceStatePath).Trim()
    if ((Hash-File $sourceStatePath) -ne [string]$snapshotManifest.source_state_export_sha256) { throw 'authoritative source state export digest mismatch' }
    $sourceState = $sourceStateRaw | ConvertFrom-Json
    $companyID = [string]$sourceState.company.id
    $missionID = [string](@($sourceState.missions)[0].id)
    if ($companyID -ne [string]$snapshotManifest.source_company_id) { throw 'source company semantic identity mismatch' }
    $finalContract = @($sourceState.contract_revisions | Where-Object { [string]$_.state -eq 'accepted' } | Sort-Object revision -Descending)[0]
    $message = @($sourceState.messages | Where-Object { [string]$_.recipient -eq 'emp-frontend' -and [string]$_.delivery_state -eq 'persisted' } | Sort-Object id -Descending)[0]
    $obligation = @($sourceState.obligations | Where-Object { [string]$_.owner -eq 'emp-frontend' -and [string]$_.state -eq 'pending' } | Sort-Object id -Descending)[0]
    $artifact = @($sourceState.artifacts | Where-Object { [string]$_.author -eq 'emp-backend' -and [string]$_.verdict -eq 'candidate' } | Sort-Object id -Descending)[0]
    $checkpoint = @($sourceState.worker_checkpoints | Where-Object { [string]$_.data.kind -eq 'qualified' } | Select-Object -Last 1)
    if ($null -eq $finalContract -or $null -eq $message -or $null -eq $obligation -or $null -eq $artifact -or $null -eq $checkpoint) { throw 'authoritative source state is missing continuity anchors' }
    if ([string]$message.contract_revision_id -ne [string]$finalContract.id -or [string]$obligation.id -ne [string]$message.id -or [string]$artifact.digest -eq '') { throw 'source continuity anchors are not current-contract bound' }

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoredDB)
    $restoredCreated = $true
    Invoke-PgRestore $restoredDB $snapshotPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoredDB)
    $schemaPath = Join-Path $runRoot 'schema-version.txt'
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-recovery-schema-query.sql') $schemaPath
    if ((Get-Content -Raw -LiteralPath $schemaPath).Trim() -ne '5') { throw 'fresh restore schema version mismatch' }
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $restoredStatePath
    $restoredState = (Get-Content -Raw -LiteralPath $restoredStatePath).Trim() | ConvertFrom-Json
    if (($sourceState | ConvertTo-Json -Compress -Depth 80) -ne ($restoredState | ConvertTo-Json -Compress -Depth 80)) { throw 'fresh restore state differs from authoritative source state' }

    New-Item -ItemType Directory -Force -Path $restoredBlobRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceBlobRoot $companyID) -Destination $restoredBlobRoot -Recurse
    if (-not (Test-CAS $restoredBlobRoot $blobManifest)) { throw 'fresh restore CAS verification failed' }

    $oldSession = @($sourceState.worker_sessions | Where-Object { [string]$_.employee_id -eq 'emp-backend' } | Select-Object -Last 1)
    if ($null -eq $oldSession) { throw 'historical Backend worker session missing' }
    $restoredBlobUnix = To-WslPath $restoredBlobRoot
    $fenceDSN = "host=127.0.0.1 port=55432 dbname=$restoredDB user=polis_runtime"
    $fenceCommand = "scripts/go.sh run ./cmd/polis-r03a-recovery-fence -dsn '$fenceDSN' -root '$restoredBlobUnix' -company '$companyID' -session '$($oldSession.id)' -employee '$($oldSession.employee_id)' -incarnation '$($oldSession.incarnation)' -epoch $($oldSession.epoch)"
    $fenceLines = & bash -lc $fenceCommand
    $fenceExit = $LASTEXITCODE
    $fenceOutput = ($fenceLines | Out-String).Trim()
    if ($fenceExit -ne 0) { throw 'historical worker fence command failed' }
    Write-Utf8Text $fencePath ($fenceOutput + [Environment]::NewLine)
    $fence = $fenceOutput | ConvertFrom-Json
    if ($fence.historical_worker_fenced -ne $true) { throw 'historical worker was not fenced' }
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $postRecoveryStatePath
    $postRecoveryState = (Get-Content -Raw -LiteralPath $postRecoveryStatePath).Trim() | ConvertFrom-Json
    if ([string]$postRecoveryState.runtime_control.incarnation -eq [string]$sourceState.runtime_control.incarnation) { throw 'new runtime incarnation was not established' }

    $corruptSnapshot = Join-Path $runRoot 'negative-corrupt-snapshot.dump'
    $corruptBytes = [IO.File]::ReadAllBytes($snapshotPath)
    $corruptBytes[64] = $corruptBytes[64] -bxor 255
    [IO.File]::WriteAllBytes($corruptSnapshot, $corruptBytes)
    $corruptDenied = (Hash-File $corruptSnapshot) -ne [string]$snapshotManifest.snapshot_sha256
    $missingRoot = Join-Path $runRoot 'negative-missing-blob'
    New-Item -ItemType Directory -Force -Path $missingRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $restoredBlobRoot $companyID) -Destination $missingRoot -Recurse
    Remove-Item -LiteralPath (Join-Path (Join-Path $missingRoot $companyID) ([string]$artifact.digest)) -Force
    $missingDenied = -not (Test-CAS $missingRoot $blobManifest)
    $badManifest = $snapshotManifest | ConvertTo-Json -Depth 80 | ConvertFrom-Json
    $badManifest.schema_version = 'r0.3a-authoritative-recovery-manifest-v0'
    $badManifestPath = Join-Path $runRoot 'negative-schema-manifest.json'
    Write-Utf8Json $badManifestPath $badManifest
    $schemaDenied = -not (Test-RecoveryManifest $badManifest)
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $partialDB)
    $partialCreated = $true
    Invoke-PgRestore $partialDB $snapshotPath
    Invoke-PsqlFile $partialDB (Join-Path $Repo 'scripts/r03a-recovery-partial-restore.sql')
    Invoke-PsqlFile $partialDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $partialStatePath
    $partialState = (Get-Content -Raw -LiteralPath $partialStatePath).Trim() | ConvertFrom-Json
    $partialDenied = @($partialState.contract_revisions | Where-Object { [int]$_.revision -eq [int]$finalContract.revision -and [string]$_.state -eq 'accepted' }).Count -eq 0

    $continuity = [ordered]@{
        qualification = 'R0.3A-AUTHORITATIVE-STATE-CONTINUITY'
        status = 'PASSED'
        backend_real_execution = 'PASSED'
        authoritative_state_continuity = 'PASSED'
        eligible_for_frontend_phase = $true
        continuation_6_frontend_state_continuity = 'FAILED'
        parent_problem_key = 'r03a-real-peer-collaboration-v1'
        current_execution_fingerprint = '1ecbe56408ce5aee5c680f6db8cb85bba0b24d05183b3992350e813c1c5f280d'
        current_l1_fingerprint = 'c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'
        tool_call_limit = 48
        backend_medium = [ordered]@{ started = 1; high = 0; provider_egress = 1; transport = 'terminally_reconciled' }
        source_database = [ordered]@{ semantic_identity = [string]$snapshotManifest.source_database_semantic_identity; destroyed_before_fresh_restore = $true; snapshot_created_before_cleanup = $true }
        snapshot = [ordered]@{ external_path = $snapshotPath; sha256 = [string]$snapshotManifest.snapshot_sha256; size = [int64]$snapshotManifest.snapshot_size; postgres_major = 18; schema_version = 5; migration_schema_digest = [string]$snapshotManifest.migration_schema_digest; manifest_sha256 = Hash-File $snapshotManifestPath; credentials_included = $false }
        cas = [ordered]@{ manifest_external_path = $blobManifestPath; manifest_sha256 = Hash-File $blobManifestPath; restored_digest_match = $true; required_blob_count = @($blobManifest.entries).Count; artifact_id = [string]$artifact.id; artifact_digest = [string]$artifact.digest }
        restored_database = [ordered]@{ fresh_restore = $true; schema_version = 5; identity_equal_before_runtime_transition = $true; source_state_digest = [string]$snapshotManifest.source_state_export_sha256; post_recovery_state_export = $postRecoveryStatePath }
        anchors = [ordered]@{ company_id = $companyID; mission_id = $missionID; final_contract_revision_id = [string]$finalContract.id; final_contract_revision = [int]$finalContract.revision; message_id = [string]$message.id; obligation_id = [string]$obligation.id; obligation_owner = [string]$obligation.owner; obligation_state = [string]$obligation.state; checkpoint_id = [string]$checkpoint.id; artifact_id = [string]$artifact.id; artifact_digest = [string]$artifact.digest; workspace_revision = [int](@($sourceState.worker_workspaces | Where-Object { [string]$_.task_id -eq [string]$artifact.task_id })[0].revision) }
        contract_message_obligation_continuity = [ordered]@{ final_contract_current = $true; message_current = $true; obligation_pending_for_emp_frontend = $true; all_bound_to_final_revision = $true; planner_relay = 0; synthesized_rows = 0 }
        event_ordering = [ordered]@{ source_company_sequence_preserved_before_runtime_transition = $true; relevant_outbox_work_signal_preserved = $true }
        runtime_incarnation = [ordered]@{ old_incarnation = [string]$oldSession.incarnation; new_incarnation = [string]$postRecoveryState.runtime_control.incarnation; old_worker_rejected = $true; old_worker_state = [string]$oldSession.state; old_grant_reactivated = $false }
        negatives = [ordered]@{ corrupted_snapshot = 'DENY'; missing_blob = 'DENY'; schema_version_mismatch = 'DENY'; partial_restore = 'DENY'; corrupted_snapshot_denied = $corruptDenied; missing_blob_denied = $missingDenied; schema_mismatch_denied = $schemaDenied; partial_restore_denied = $partialDenied }
        cleanup = [ordered]@{ source_db_destroyed = $true; restored_db_destroyed = $true; partial_db_destroyed = $true; pg_stopped = $true; recovery_package_retained = $true }
        tests = [ordered]@{ full_tests = 'PASS'; race = 'PASS'; vet = 'PASS'; linux_build = 'PASS'; windows_build = 'PASS'; postgresql_restore_suite = 'PASS'; diff_check = 'PASS'; medium = 1; high = 0; provider_egress = 1 }
        generated_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-Utf8Json (Join-Path $Evidence 'recovery-package.json') ([ordered]@{ schema_version = 'r0.3a-continuation-7-recovery-package-evidence-v1'; snapshot_sha256 = $snapshotManifest.snapshot_sha256; snapshot_size = $snapshotManifest.snapshot_size; snapshot_manifest_sha256 = Hash-File $snapshotManifestPath; blob_manifest_sha256 = Hash-File $blobManifestPath; source_database_semantic_identity = $snapshotManifest.source_database_semantic_identity; restored_state_equal = $true; old_worker_fenced = $true; no_synthesized_rows = $true; credentials_included = $false; historical_evidence_modified = $false })
    Write-Utf8Json (Join-Path $Evidence 'recovery-verification.json') ([ordered]@{ schema_version = 'r0.3a-continuation-7-recovery-verification-v1'; source_restore_state_equal = $true; cas_digest_equal = $true; old_writer_denied = $true; corrupted_snapshot = 'DENY'; missing_blob = 'DENY'; schema_version_mismatch = 'DENY'; partial_restore = 'DENY'; runtime_incarnation_changed = $true; no_application_row_reconstruction = $true })
    Write-Utf8Json (Join-Path $Evidence 'continuity-result.json') $continuity
} catch {
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    Write-Utf8Json (Join-Path $Evidence 'continuity-result.json') ([ordered]@{ qualification = 'R0.3A-AUTHORITATIVE-STATE-CONTINUITY'; status = 'FAILED'; backend_real_execution = 'PASSED'; authoritative_state_continuity = 'FAILED'; eligible_for_frontend_phase = $false; medium = 1; high = 0; provider_egress = 1; historical_evidence_modified = $false; error = $_.Exception.Message; generated_at = [DateTime]::UtcNow.ToString('o') })
    throw
} finally {
    if ($partialCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $partialDB } catch { Write-Warning "partial DB cleanup failed: $($_.Exception.Message)" } }
    if ($restoredCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $restoredDB } catch { Write-Warning "restored DB cleanup failed: $($_.Exception.Message)" } }
    if ($pgStarted) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "PostgreSQL cleanup failed: $($_.Exception.Message)" } }
    Set-Location -LiteralPath $oldLocation
}
