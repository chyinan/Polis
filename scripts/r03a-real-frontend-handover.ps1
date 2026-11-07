# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-real-frontend-handover-revised-v2',
    [string]$RecoveryRun = 'D:\Polis-recovery\r0.3a-backend-continuation-7\run-20260912135427-17a3e1a3b6c3',
    [string]$BackendRuntimeRoot = 'D:\Polis-recovery\r0.3a-backend-continuation-7-runtime',
    [string]$RuntimeRoot = 'D:\Polis-recovery\r0.3a-frontend-handover-revised-v2-runtime',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$L2OfflineEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-v2',
    [string]$L2LiveEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-live',
    [string]$BehavioralContractQualification = 'D:\Programs\Polis\summary\r0-3a-behavioral-contract-hardening.md',
    [string]$BlobDurabilityQualification = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json',
    [string]$CheckerFeedbackQualification = 'D:\Programs\Polis\evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json',
    [string]$RuntimeArtifactManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [int]$ToolCallLimit = 48,
    [string]$FrontendExe = 'D:\Programs\Polis\.runtime\windows\r0.3a-real-frontend\polis-r03a-real-frontend.exe'
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo
$pgStarted = $false
$restoredCreated = $false
$runID = (Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$restoreDB = 'polis_r0_3a_frontend_restore_' + $runID.Replace('-', '')
$runRoot = Join-Path $RuntimeRoot ('run-' + $runID)
$snapshotPath = Join-Path $RecoveryRun 'polis-authoritative-state.dump'
$snapshotManifestPath = Join-Path $RecoveryRun 'snapshot-manifest.json'
$blobManifestPath = Join-Path $RecoveryRun 'blob-cas-manifest.json'
$sourceStatePath = Join-Path $RecoveryRun 'source-state.json'
$restoredStatePath = Join-Path $runRoot 'restored-state.json'
$restoredStatePath = Join-Path $runRoot 'restored-state.json'
$postFrontendStatePath = Join-Path $runRoot 'post-frontend-state.json'
$frontendBlobRoot = Join-Path $RuntimeRoot 'blobs'

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

function Invoke-Checked([string]$File, [string[]]$Arguments) {
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$File exited with code $LASTEXITCODE" }
}

function To-WslPath([string]$Path) {
    return Convert-WindowsPathToWsl $Path
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
        if ($existing.Count -gt 0) { throw 'Frontend handover evidence path is not fresh; refusing retry/reset' }
    }
    foreach ($path in @($snapshotPath, $snapshotManifestPath, $blobManifestPath, $sourceStatePath, "$L1Evidence\result.json", "$L2OfflineEvidence\execution-manifest.json", "$L2LiveEvidence\result.json", $BehavioralContractQualification, $BlobDurabilityQualification, $CheckerFeedbackQualification, $RuntimeArtifactManifest)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "required Frontend input is unavailable: $path" }
    }
    New-Item -ItemType Directory -Force -Path $Evidence, $RuntimeRoot, $runRoot, $frontendBlobRoot | Out-Null

    $snapshotManifest = Get-Content -Raw -LiteralPath $snapshotManifestPath | ConvertFrom-Json
    $blobManifest = Get-Content -Raw -LiteralPath $blobManifestPath | ConvertFrom-Json
    $l1 = Get-Content -Raw -LiteralPath "$L1Evidence\result.json" | ConvertFrom-Json
    $l2 = Get-Content -Raw -LiteralPath "$L2OfflineEvidence\offline-result.json" | ConvertFrom-Json
    $l2Live = Get-Content -Raw -LiteralPath "$L2LiveEvidence\result.json" | ConvertFrom-Json
    $checkerFeedback = Get-Content -Raw -LiteralPath $CheckerFeedbackQualification | ConvertFrom-Json
    if (-not (Test-RecoveryManifest $snapshotManifest) -or (Hash-File $snapshotPath) -ne [string]$snapshotManifest.snapshot_sha256 -or [int64](Get-Item -LiteralPath $snapshotPath).Length -ne [int64]$snapshotManifest.snapshot_size -or (Hash-File $blobManifestPath) -ne [string]$snapshotManifest.blob_manifest.sha256 -or -not (Test-CAS $BackendRuntimeRoot\blobs $blobManifest)) { throw 'continuation-7 recovery package hash/CAS verification failed' }
    if ($l1.status -ne 'passed' -or $l1.current_binary_windows_l1 -ne 'QUALIFIED' -or $l2.qualification -ne 'R0.3A-CURRENT-BINARY-REVISED-FRONTEND-L2' -or $l2.status -ne 'PASSED' -or $l2.surface_role -ne 'peer_frontend' -or $l2Live.status -ne 'passed' -or $l2Live.current_frontend_revised_l2 -ne 'QUALIFIED' -or $l2Live.eligible_for_revised_frontend_initial_live -ne $true -or [int]$l2Live.registered_tool_count -ne [int]$l2.tool_count -or [string]$l2.tool_manifest_digest -ne [string]$l2Live.tool_manifest_digest -or [string]$l2.aggregate_schema_digest -ne [string]$l2Live.aggregate_schema_digest -or [int]$l2.aggregate_schema_bytes -ne [int]$l2Live.aggregate_schema_bytes) { throw 'current Frontend execution qualification is stale or ineligible' }
    if ($checkerFeedback.qualification -ne 'R0.3A-FRONTEND-CHECKER-FEEDBACK-L2' -or $checkerFeedback.status -ne 'PASSED' -or $checkerFeedback.provider_visible_registration_changed -ne $false -or [string]$checkerFeedback.policy_revisions.acceptance_checker_revision -ne 'peer-semantic-checker@3') { throw 'checker feedback semantic qualification is stale or ineligible' }
    if ([string]$l2.controlled_runtime_manifest_digest -ne (Hash-File $RuntimeArtifactManifest) -or [string]$l1.auth_credential_revision_fingerprint -ne [string]$l2Live.auth_credential_revision_fingerprint -or [string]$l1.effective_config_digest -ne [string]$l2Live.effective_config_digest -or [string]$l1.effective_transport_config_digest -ne [string]$l2Live.effective_transport_config_digest) { throw 'current runtime/auth/config qualification drifted' }

    $sourceState = ((Get-Content -Raw -LiteralPath $sourceStatePath).Trim() | ConvertFrom-Json)
    $companyID = [string]$sourceState.company.id
    $runtime = Get-Content -Raw -LiteralPath $RuntimeArtifactManifest | ConvertFrom-Json
    Copy-Item -LiteralPath (Join-Path (Join-Path $BackendRuntimeRoot 'blobs') $companyID) -Destination $frontendBlobRoot -Recurse
    if (-not (Test-CAS $frontendBlobRoot $blobManifest)) { throw 'Frontend restored CAS staging failed before model start' }

    $binding = [ordered]@{
        execution_fingerprint = [string]$l2.execution_fingerprint
        l1_fingerprint = [string]$l1.execution_fingerprint
        revised_l2_fingerprint = [string]$l2.execution_fingerprint
        controlled_runtime_manifest_digest = Hash-File $RuntimeArtifactManifest
        employee_id = 'emp-frontend'
        problem_key = 'r03a-real-peer-collaboration-v1'
        purpose = 'real_frontend_handover_from_recovered_state'
        model = 'gpt-5.6-luna'
        profile = 'gpt-5.6-luna/medium'
        effort = 'medium'
        allowance_limits = [ordered]@{ medium_limit = 2; high_limit = 0; concurrency = 1; tool_call_limit = $ToolCallLimit }
    }
    $bindingPath = Join-Path $RuntimeRoot 'authorization-binding.json'
    Write-Utf8Json $bindingPath $binding

    $owner = (& bash -lc 'id -un').Trim()
    $configPath = Join-Path $runRoot 'frontend-execution-config.windows.json'
    $wslConfigPath = Join-Path $runRoot 'frontend-execution-config.wsl.json'
    $configProbePath = Join-Path $runRoot 'frontend-execution-config.wsl-report.json'
    $postgresDumpPath = Join-Path $Repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump'
    $frontendConfig = [ordered]@{
        dsn = "host=127.0.0.1 port=55432 dbname=$restoreDB user=polis_runtime"
        binary = [string]$runtime.codex_binary_staged_path
        code_mode_host = [string]$runtime.code_mode_host_staged_path
        auth_file = 'C:\Users\chyinan\.codex\auth.json'
        selected_config_path = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml'
        execution_manifest_path = "$L2OfflineEvidence\execution-manifest.json"
        qualification_path = "$L2LiveEvidence\result.json"
        blob_durability_qualification_path = $BlobDurabilityQualification
        behavioral_contract_path = $BehavioralContractQualification
        checker_feedback_qualification_path = $CheckerFeedbackQualification
        postgres_dump_path = $postgresDumpPath
        postgres_snapshot_dsn = "host=127.0.0.1 port=55432 dbname=$restoreDB user=$owner"
        runtime_root = $RuntimeRoot
        evidence_root = $Evidence
        recovery_package_root = $RecoveryRun
        source_cas_root = Join-Path $BackendRuntimeRoot 'blobs'
        runtime_artifact_manifest_path = $RuntimeArtifactManifest
        authorization_binding_path = $bindingPath
        current_l1_evidence_path = $L1Evidence
        execution_fingerprint = [string]$l2.execution_fingerprint
        current_l1_fingerprint = [string]$l1.execution_fingerprint
        handover_boundary_evidence_path = $Evidence
        handover_boundary_snapshot_path = Join-Path $runRoot 'handover-boundary-state.dump'
        problem_key = 'r03a-real-peer-collaboration-v1'
        purpose = 'real_frontend_handover_from_recovered_state'
        employee_id = 'emp-frontend'
        model = 'gpt-5.6-luna'
        effort = 'medium'
        tool_call_limit = $ToolCallLimit
        medium_limit = 1
        high_limit = 0
        concurrency = 1
        retry = $false
        reset = $false
    }
    Write-FrontendConfigPair $frontendConfig $configPath $wslConfigPath
    $configProbe = Invoke-FrontendWslConfigProbe $Repo $wslConfigPath $configProbePath
    if ($configProbe.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'FRONTEND_EXECUTION_CONFIG_INVALID' }

    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $FrontendExe) | Out-Null
    Invoke-Checked 'bash' @('scripts/build-r03a-real-frontend-windows.sh')
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoreDB)
    $restoredCreated = $true
    Invoke-PgRestore $restoreDB $snapshotPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoreDB)
    $schemaPath = Join-Path $runRoot 'schema-version.txt'
    Invoke-PsqlFile $restoreDB (Join-Path $Repo 'scripts/r03a-recovery-schema-query.sql') $schemaPath
    if ((Get-Content -Raw -LiteralPath $schemaPath).Trim() -ne '5') { throw 'Frontend fresh restore schema version mismatch' }
    Invoke-PsqlFile $restoreDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $restoredStatePath
    $restoredState = (Get-Content -Raw -LiteralPath $restoredStatePath).Trim() | ConvertFrom-Json
    if (($sourceState | ConvertTo-Json -Compress -Depth 80) -ne ($restoredState | ConvertTo-Json -Compress -Depth 80)) { throw 'Frontend restored state differs from authoritative continuation-7 state' }
    & $FrontendExe -execution-config $configPath -initial-only
    $frontendExit = $LASTEXITCODE
    if ($frontendExit -ne 0) { throw "Frontend handover command exited with code $frontendExit" }
    $frontendResult = Get-Content -Raw -LiteralPath (Join-Path $Evidence 'result.json') | ConvertFrom-Json
    if ($frontendResult.frontend_initial_handover_boundary -eq 'PASSED') {
        if ([string]$frontendResult.frontend_successor -ne 'not_started' -or [int]$frontendResult.medium_started -ne 1) { throw 'Frontend initial handover boundary evidence was invalid' }
    } elseif ([string]$frontendResult.status -ne 'passed' -or [string]$frontendResult.real_frontend_handover -ne 'passed' -or [string]$frontendResult.real_peer_collaboration -ne 'passed') {
        throw 'Frontend handover business conditions failed'
    }

    Invoke-PsqlFile $restoreDB (Join-Path $Repo 'scripts/r03a-backend-state-query.sql') $postFrontendStatePath
} finally {
    if ($restoredCreated) { try { & bash scripts/r03a-real-backend-pg.sh drop $restoreDB } catch { Write-Warning "Frontend restored DB cleanup failed: $($_.Exception.Message)" } }
    if ($pgStarted) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch { Write-Warning "Frontend PostgreSQL cleanup failed: $($_.Exception.Message)" } }
    Set-Location -LiteralPath $oldLocation
}
