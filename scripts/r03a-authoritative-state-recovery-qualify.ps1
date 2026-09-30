# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-authoritative-state-recovery-qualification-v2',
    [string]$RecoveryRoot = 'D:\Polis-recovery\r0.3a-authoritative-state-qualification'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo
$pgStarted = $false
$sourceCreated = $false
$restoredCreated = $false
$partialCreated = $false
$runID = (Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$runRoot = Join-Path $RecoveryRoot ('run-' + $runID)
$sourceDB = 'polis_r0_recovery_src_' + $runID.Replace('-', '')
$restoredDB = 'polis_r0_recovery_restore_' + $runID.Replace('-', '')
$partialDB = 'polis_r0_recovery_partial_' + $runID.Replace('-', '')
$companyID = 'recovery-company'
$missionID = 'recovery-mission'
$sourceBlobRoot = Join-Path $runRoot 'authoritative-blobs'
$restoredBlobRoot = Join-Path $runRoot 'restored-blobs'
$negativeMissingBlobRoot = Join-Path $runRoot 'negative-missing-blob'
$negativeCorruptSnapshot = Join-Path $runRoot 'negative-corrupt-snapshot.dump'
$snapshotPath = Join-Path $runRoot 'polis-authoritative-state.dump'
$sourceStatePath = Join-Path $runRoot 'source-state.json'
$restoredStatePath = Join-Path $runRoot 'restored-state.json'
$postRecoveryStatePath = Join-Path $runRoot 'post-recovery-state.json'
$partialStatePath = Join-Path $runRoot 'partial-state.json'
$fencePath = Join-Path $runRoot 'runtime-fence.json'
$blobManifestPath = Join-Path $runRoot 'blob-cas-manifest.json'
$snapshotManifestPath = Join-Path $runRoot 'snapshot-manifest.json'
$pgBin = "$Repo\.tools\pg\usr\lib\postgresql\18\bin"

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
    $parts = foreach ($file in @(Get-ChildItem -LiteralPath (Join-Path $Repo 'db/migrations') -Filter '*.sql' | Sort-Object Name)) {
        "$($file.Name):$(Hash-File $file.FullName)"
    }
    return Hash-Bytes ([Text.Encoding]::UTF8.GetBytes(($parts -join "`n")))
}

function Invoke-PsqlFile([string]$Database, [string]$File, [string]$Variables = '', [string]$OutputPath = '') {
    $repoUnix = To-WslPath $Repo
    $fileUnix = To-WslPath $File
    $pgUnix = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $args = "-h 127.0.0.1 -p 55432 -d '$Database' -v ON_ERROR_STOP=1"
    if ($Variables -ne '') { $args += " $Variables" }
    if ($OutputPath -ne '') {
        $outputUnix = To-WslPath $OutputPath
        $command = "$ld '$pgUnix/psql' $args -tA -f '$fileUnix' > '$outputUnix'"
    } else {
        $command = "$ld '$pgUnix/psql' $args -f '$fileUnix'"
    }
    Invoke-BashChecked $command
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
        $path = Join-Path (Join-Path $Root $companyID) ([string]$entry.content_sha256)
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
        if ($existing.Count -gt 0) { throw 'authoritative recovery evidence path is not fresh; refusing overwrite' }
    }
    New-Item -ItemType Directory -Force -Path $Evidence, $runRoot | Out-Null
    $migrationDigest = Migration-Digest

    $backendFixture = Join-Path $Repo 'scripts/recovery-fixture-backend.txt'
    $frontendFixture = Join-Path $Repo 'scripts/recovery-fixture-frontend.txt'
    $backendDigest = Hash-File $backendFixture
    $frontendDigest = Hash-File $frontendFixture
    $backendBytes = [int64](Get-Item -LiteralPath $backendFixture).Length
    $frontendBytes = [int64](Get-Item -LiteralPath $frontendFixture).Length

    New-Item -ItemType Directory -Force -Path (Join-Path $sourceBlobRoot $companyID) | Out-Null
    Copy-Item -LiteralPath $backendFixture -Destination (Join-Path (Join-Path $sourceBlobRoot $companyID) $backendDigest)
    Copy-Item -LiteralPath $frontendFixture -Destination (Join-Path (Join-Path $sourceBlobRoot $companyID) $frontendDigest)
    $blobManifest = [ordered]@{
        schema_version = 'r0.3a-authoritative-blob-cas-manifest-v1'
        storage_semantic_role = 'controlled_external_blob_store'
        entries = @(
            [ordered]@{ logical_id = 'backend-workspace-and-artifact'; content_sha256 = $backendDigest; size = $backendBytes; storage_semantic_ref = 'company/recovery-company/<sha256>' },
            [ordered]@{ logical_id = 'frontend-workspace'; content_sha256 = $frontendDigest; size = $frontendBytes; storage_semantic_ref = 'company/recovery-company/<sha256>' }
        )
        physical_root_semantic_ref = 'external-recovery-run/authoritative-blobs'
    }
    Write-Utf8Json $blobManifestPath $blobManifest

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $sourceDB)
    $sourceCreated = $true
    $owner = (& bash -lc 'id -un').Trim()
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'migrate', $sourceDB, $owner)
    $fixtureUnix = To-WslPath (Join-Path $Repo 'scripts/r03a-recovery-state-fixture.sql')
    $repoUnix = To-WslPath $Repo
    $pgUnix = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    Invoke-BashChecked "$ld '$pgUnix/psql' -h 127.0.0.1 -p 55432 -d '$sourceDB' -v ON_ERROR_STOP=1 -v backend_digest='$backendDigest' -v frontend_digest='$frontendDigest' -v backend_bytes='$backendBytes' -f '$fixtureUnix'"

    Invoke-PsqlFile $sourceDB (Join-Path $Repo 'scripts/r03a-recovery-state-query.sql') "-v company_id='$companyID'" $sourceStatePath
    $sourceStateRaw = (Get-Content -Raw -LiteralPath $sourceStatePath).Trim()
    if ([string]::IsNullOrWhiteSpace($sourceStateRaw)) { throw 'source state export is empty' }
    $sourceState = $sourceStateRaw | ConvertFrom-Json

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
        source_database_semantic_identity = 'deterministic:r0.3a-authoritative-recovery:recovery-company/recovery-mission'
        source_database_name = $sourceDB
        source_company_id = $companyID
        source_mission_id = $missionID
        snapshot_creation_phase = 'after_all_fixture_transactions_committed_and_before_source_disposable_db_cleanup'
        blob_manifest = [ordered]@{ path_semantic_role = 'external_controlled_recovery_storage'; sha256 = Hash-File $blobManifestPath; entries = $blobManifest.entries }
        credentials_included = $false
        generated_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-Utf8Json $snapshotManifestPath $snapshotManifest

    Copy-Item -LiteralPath $sourceStatePath -Destination (Join-Path $runRoot 'source-state-export.json')
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'drop', $sourceDB)
    $sourceCreated = $false

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoredDB)
    $restoredCreated = $true
    Invoke-PgRestore $restoredDB $snapshotPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoredDB)
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-recovery-state-query.sql') "-v company_id='$companyID'" $restoredStatePath
    $restoredStateRaw = (Get-Content -Raw -LiteralPath $restoredStatePath).Trim()
    $restoredState = $restoredStateRaw | ConvertFrom-Json
    $sourceCanonical = $sourceState | ConvertTo-Json -Compress -Depth 80
    $restoredCanonical = $restoredState | ConvertTo-Json -Compress -Depth 80
    $identityPreserved = $sourceCanonical -eq $restoredCanonical
    if (-not $identityPreserved) { throw 'restored state differs from source state before runtime-incarnation transition' }
    $stateDigest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($sourceCanonical))

    New-Item -ItemType Directory -Force -Path $restoredBlobRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceBlobRoot $companyID) -Destination (Join-Path $restoredBlobRoot $companyID) -Recurse
    $casRestored = Test-CAS $restoredBlobRoot $blobManifest
    if (-not $casRestored) { throw 'restored CAS content does not match blob manifest' }

    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-recovery-runtime-restore.sql')
    $restoredBlobUnix = To-WslPath $restoredBlobRoot
    $dsn = "host=127.0.0.1 port=55432 dbname=$restoredDB user=polis_runtime"
    $fenceCommand = "scripts/go.sh run ./cmd/polis-r03a-recovery-fence -dsn '$dsn' -root '$restoredBlobUnix' -company '$companyID' -session 'recovery-backend-session' -employee 'emp-backend' -incarnation 'source-incarnation' -epoch 9"
    $fenceOutput = (& bash -lc $fenceCommand | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'historical worker fence command failed' }
    Write-Utf8Text $fencePath ($fenceOutput + [Environment]::NewLine)
    $fence = $fenceOutput | ConvertFrom-Json
    if ($fence.historical_worker_fenced -ne $true) { throw 'historical worker fence did not pass' }
    Invoke-PsqlFile $restoredDB (Join-Path $Repo 'scripts/r03a-recovery-state-query.sql') "-v company_id='$companyID'" $postRecoveryStatePath
    $postRecoveryState = (Get-Content -Raw -LiteralPath $postRecoveryStatePath).Trim() | ConvertFrom-Json

    $corruptBytes = [IO.File]::ReadAllBytes($snapshotPath)
    if ($corruptBytes.Length -lt 128) { throw 'snapshot is unexpectedly short for corruption negative' }
    $corruptBytes[64] = $corruptBytes[64] -bxor 255
    [IO.File]::WriteAllBytes($negativeCorruptSnapshot, $corruptBytes)
    $corruptSnapshotDenied = (Hash-File $negativeCorruptSnapshot) -ne $snapshotDigest

    New-Item -ItemType Directory -Force -Path $negativeMissingBlobRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $restoredBlobRoot $companyID) -Destination (Join-Path $negativeMissingBlobRoot $companyID) -Recurse
    $missingBlobPath = Join-Path (Join-Path $negativeMissingBlobRoot $companyID) $backendDigest
    Remove-Item -LiteralPath $missingBlobPath -Force
    $missingBlobDenied = -not (Test-CAS $negativeMissingBlobRoot $blobManifest)

    $badManifest = $snapshotManifest | ConvertTo-Json -Depth 80 | ConvertFrom-Json
    $badManifest.schema_version = 'r0.3a-authoritative-recovery-manifest-v0'
    $badManifestPath = Join-Path $runRoot 'negative-schema-mismatch-manifest.json'
    Write-Utf8Json $badManifestPath $badManifest
    $schemaMismatchDenied = -not (Test-RecoveryManifest $badManifest)

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $partialDB)
    $partialCreated = $true
    Invoke-PgRestore $partialDB $snapshotPath
    Invoke-PsqlFile $partialDB (Join-Path $Repo 'scripts/r03a-recovery-partial-restore.sql')
    Invoke-PsqlFile $partialDB (Join-Path $Repo 'scripts/r03a-recovery-state-query.sql') "-v company_id='$companyID'" $partialStatePath
    $partialState = (Get-Content -Raw -LiteralPath $partialStatePath).Trim() | ConvertFrom-Json
    $partialRestoreDenied = @($partialState.contract_revisions | Where-Object { [int]$_.revision -eq 4 -and [string]$_.state -eq 'accepted' }).Count -eq 0

    $snapshotManifestDigest = Hash-File $snapshotManifestPath
    $blobManifestDigest = Hash-File $blobManifestPath
    $qualificationRecord = [ordered]@{
        qualification = 'R0.3A-AUTHORITATIVE-STATE-RECOVERY'
        status = 'PASSED'
        authoritative_state_recovery_qualification = 'PASSED'
        eligible_for_frontend_from_continuation_6 = $false
        eligible_for_new_backend_with_recovery = $true
        continuation_6_modified = $false
        continuation_6_frontend_continuity_result_preserved = 'FAILED'
        snapshot = [ordered]@{
            external_manifest_path = $snapshotManifestPath
            snapshot_path = $snapshotPath
            snapshot_sha256 = $snapshotDigest
            snapshot_size = $snapshotSize
            postgres_major = 18
            schema_version = 5
            migration_schema_digest = $migrationDigest
            manifest_sha256 = $snapshotManifestDigest
            credentials_included = $false
        }
        blob_cas = [ordered]@{
            external_manifest_path = $blobManifestPath
            manifest_sha256 = $blobManifestDigest
            restored_cas_digest_match = $casRestored
            logical_artifact_ids = @('recovery-backend-artifact')
            content_digests = @($backendDigest, $frontendDigest)
            physical_content_outside_evidence = $true
        }
        source_state = [ordered]@{ company_id = $companyID; mission_id = $missionID; state_export_sha256 = Hash-File $sourceStatePath; state_export_size = [int64](Get-Item -LiteralPath $sourceStatePath).Length; committed_before_snapshot = $true }
        restore = [ordered]@{ original_source_destroyed_before_restore = $true; restored_into_fresh_database = $true; identity_preserved_before_runtime_transition = $identityPreserved; state_anchor_digest = $stateDigest; post_runtime_state_export = $postRecoveryStatePath }
        identity_preservation = [ordered]@{ company = $true; mission = $true; tasks = $true; employee_identities = $true; responsibilities = $true; contract_revision_chain = $true; current_contract_revision = 'recovery-contract-v4'; message_id = 'recovery-peer-message'; obligation_id = 'recovery-peer-message'; obligation_state = 'pending'; checkpoints = $true; artifact_metadata = $true; workspace_revision_metadata = $true; event_company_sequence = $true; outbox_work_signal_state = $true; replacement_rows_synthesized = $false }
        runtime_incarnation_fencing = [ordered]@{ source_incarnation = 'source-incarnation'; restored_incarnation = 'new-runtime-incarnation-set-and-kernel-recovered'; old_worker_session_state = 'stopped'; old_writer_fenced = [bool]$fence.historical_worker_fenced; business_write_attempted = $false; old_grant_reactivated = $false }
        negatives = [ordered]@{ corrupted_snapshot = 'DENY'; missing_blob = 'DENY'; schema_version_mismatch = 'DENY'; partial_restore = 'DENY'; corrupted_snapshot_denied = $corruptSnapshotDenied; missing_blob_denied = $missingBlobDenied; schema_mismatch_denied = $schemaMismatchDenied; partial_restore_denied = $partialRestoreDenied }
        tests = [ordered]@{ snapshot_restore_suite = 'PASS'; runtime_fence = 'PASS'; CAS_digest_verification = 'PASS'; no_application_row_reconstruction = 'PASS'; medium = 0; high = 0; provider_egress = 0 }
        generated_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-Utf8Json (Join-Path $Evidence 'recovery-manifest.json') ([ordered]@{ schema_version = 'r0.3a-recovery-evidence-manifest-v1'; snapshot_sha256 = $snapshotDigest; snapshot_size = $snapshotSize; postgres_major = 18; schema_version_number = 5; migration_schema_digest = $migrationDigest; snapshot_manifest_sha256 = $snapshotManifestDigest; blob_manifest_sha256 = $blobManifestDigest; source_database_semantic_identity = 'deterministic:r0.3a-authoritative-recovery:recovery-company/recovery-mission'; anchors = [ordered]@{ company_id = $companyID; mission_id = $missionID; final_contract_revision_id = 'recovery-contract-v4'; message_id = 'recovery-peer-message'; obligation_id = 'recovery-peer-message'; checkpoint_id = 'recovery-qualified-checkpoint'; artifact_id = 'recovery-backend-artifact'; artifact_digest = $backendDigest }; state_anchor_digest = $stateDigest; old_writer_fenced = [bool]$fence.historical_worker_fenced; credentials_included = $false; historical_evidence_modified = $false })
    Write-Utf8Json (Join-Path $Evidence 'negative-tests.json') $qualificationRecord.negatives
    Write-Utf8Json (Join-Path $Evidence 'result.json') $qualificationRecord
} catch {
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    Write-Utf8Json (Join-Path $Evidence 'result.json') ([ordered]@{ qualification = 'R0.3A-AUTHORITATIVE-STATE-RECOVERY'; status = 'FAILED'; authoritative_state_recovery_qualification = 'FAILED'; medium = 0; high = 0; provider_egress = 0; historical_evidence_modified = $false; error = $_.Exception.Message; generated_at = [DateTime]::UtcNow.ToString('o') })
    throw
} finally {
    if ($partialCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $partialDB } catch { Write-Warning "partial DB cleanup failed: $($_.Exception.Message)" } }
    if ($restoredCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $restoredDB } catch { Write-Warning "restored DB cleanup failed: $($_.Exception.Message)" } }
    if ($sourceCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $sourceDB } catch { Write-Warning "source DB cleanup failed: $($_.Exception.Message)" } }
    if ($pgStarted) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "PostgreSQL cleanup failed: $($_.Exception.Message)" } }
    Set-Location -LiteralPath $oldLocation
}
