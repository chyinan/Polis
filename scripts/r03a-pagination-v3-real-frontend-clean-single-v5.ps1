param(
    [string]$EvidenceId = 'r03a-pagination-v3-real-frontend-clean-single-session-windows-v5-attempt-2',
    [string]$RuntimeId = 'r03a-frontend-business-v5-attempt-2',
    [string]$BaselineId = 'r03a-frontend-clean-baseline-v5',
    [string]$DbName = 'polis_r0_3a_frontend_clean_baseline_v5',
    [int]$Port = 55432,
    [string]$Socket = '/tmp/polis-pg-r03a-frontend-business-v5'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$cleanRootWsl = "/home/chyinan/.local/state/polis-recovery/$BaselineId"
$pgDataWsl = "$cleanRootWsl/postgres/pgdata"
$casRootWsl = "$cleanRootWsl/blobs"
$casRootWindows = Convert-WslPathToWindows $casRootWsl
$evidenceWindows = Join-Path $Repo "evidence\development\$EvidenceId"
$runtimeWindows = Join-Path $Repo ".runtime\windows\$RuntimeId"
$runnerExe = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$baselinePath = Join-Path $Repo "evidence\development\$BaselineId\FrontendExecutionBaselineManifest.json"
$baselineHash = (Get-Content -Raw -LiteralPath "$baselinePath.sha256").Trim()
$casManifestPath = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$frontendBindingPath = Join-Path $evidenceWindows 'frontend-binding.json'
$frontendResultPath = Join-Path $evidenceWindows 'result.json'
$allowancePath = Join-Path $evidenceWindows 'allowance.json'
$snapshotPath = Join-Path $runtimeWindows 'unused-frontend-snapshot.dump'
$expectedEnvelopeFingerprint = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51'
$policyRevision = 'r03a-transport-policy@1'
$company = 'r03a-pagination-v3-company-1789379324974307400'
$inventory = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'
$databaseHardeningEvidence = Join-Path $Repo 'evidence\development\r0.3a-windows-wsl-database-access-view-hardening-v1'
$databaseBindingPath = Join-Path $databaseHardeningEvidence 'runtime-database-binding.json'
$databaseViewPath = Join-Path $databaseHardeningEvidence 'windows-database-access-view.json'
$databaseBindingFingerprint = '3169383be36c40901474396f46171938ffc2987e522a5c4a5328f3ab953bb148'
$databaseStrategyRevision = 'r03a-windows-localhost-forwarding@1'

if (Test-Path -LiteralPath $evidenceWindows) {
    $existing = @(Get-ChildItem -LiteralPath $evidenceWindows -Force)
    if ($existing.Count -gt 0) { throw 'V5 Frontend business evidence path is not fresh' }
}
if (-not (Test-Path -LiteralPath $runnerExe -PathType Leaf)) { throw 'Windows-native business runner is unavailable' }
if (-not (Test-Path -LiteralPath $baselinePath -PathType Leaf)) { throw 'V5 baseline manifest is unavailable' }
if (-not (Test-Path -LiteralPath $casManifestPath -PathType Leaf)) { throw 'authoritative CAS manifest is unavailable' }
New-Item -ItemType Directory -Force -Path $runtimeWindows | Out-Null

$envMap = [ordered]@{
    POLIS_DSN = "host=127.0.0.1 port=$Port dbname=$DbName user=polis_runtime"
    POLIS_CODEX_BINARY = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
    POLIS_CODEX_CODE_MODE_HOST = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
    POLIS_CODEX_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
    POLIS_SELECTED_CODEX_CONFIG = Join-Path $Repo '.runtime\linux\r03a-t14c\home\config.toml'
    POLIS_V3_RUNTIME_ROOT = $runtimeWindows
    POLIS_V3_EVIDENCE = $evidenceWindows
    POLIS_V3_ACCEPTANCE_QUALIFICATION = Join-Path $Repo 'evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json'
    POLIS_BLOB_DURABILITY_QUALIFICATION = Join-Path $Repo 'evidence\development\r0.3a-blob-durability\qualification.json'
    POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION = Join-Path $Repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json'
    POLIS_CURRENT_L1_EVIDENCE = Join-Path $Repo 'evidence\development\r0.3a-current-binary-l1'
    POLIS_V3_BACKEND_EXECUTION_MANIFEST = Join-Path $Repo 'evidence\development\r0.3a-current-binary-backend-l2-v6\execution-manifest.json'
    POLIS_V3_BACKEND_QUALIFICATION = Join-Path $Repo 'evidence\development\r0.3a-current-binary-backend-l2-target-live-v1\result.json'
    POLIS_V3_BACKEND_L2_LIVE = Join-Path $Repo 'evidence\development\r0.3a-current-binary-backend-l2-target-live-v1\result.json'
    POLIS_V3_FRONTEND_EXECUTION_MANIFEST = Join-Path $Repo 'evidence\development\r0.3a-frontend-evidence-contract-hardening-l2-offline\execution-manifest.json'
    POLIS_V3_FRONTEND_QUALIFICATION = Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\result.json'
    POLIS_V3_FRONTEND_L2_LIVE = Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\result.json'
    POLIS_V3_BACKEND_BINDING = Join-Path $Repo 'evidence\development\r0.3a-pagination-v3-backend-sample-6\authorization-binding.json'
    POLIS_V3_FRONTEND_BINDING = $frontendBindingPath
    POLIS_V3_ALLOWANCE = $allowancePath
    POLIS_V3_BACKEND_RESULT = Join-Path $Repo 'evidence\development\r03a-sample-6-v9-continuity-adjudication-v3\backend-restored-runtime-verification.json'
    POLIS_V3_FRONTEND_RESULT = $frontendResultPath
    POLIS_V3_POSTGRES_SNAPSHOT = $snapshotPath
    POLIS_V3_POSTGRES_SNAPSHOT_DSN = "host=127.0.0.1 port=$Port dbname=$DbName user=chyinan"
    POLIS_V3_RESTORE_PROOF = Join-Path $Repo "evidence\development\$BaselineId\result.json"
    POLIS_V3_FRONTEND_BASELINE_MANIFEST = $baselinePath
    POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $baselineHash
    POLIS_V3_FRONTEND_CAS_MANIFEST = $casManifestPath
    POLIS_V3_FRONTEND_CAS_ROOT = $casRootWindows
    POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'
    POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = $company
    POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = $inventory
    POLIS_RUNTIME_DATABASE_BINDING = $databaseBindingPath
    POLIS_WINDOWS_DATABASE_ACCESS_VIEW = $databaseViewPath
    POLIS_RUNTIME_DATABASE_BINDING_FINGERPRINT = $databaseBindingFingerprint
    POLIS_DATABASE_ACCESS_STRATEGY_REVISION = $databaseStrategyRevision
    POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT = Join-Path $evidenceWindows 'windows-database-preflight.json'
    POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $expectedEnvelopeFingerprint
    POLIS_V3_TRANSPORT_POLICY_REVISION = $policyRevision
}
foreach ($entry in $envMap.GetEnumerator()) { Set-Item -Path ("Env:" + $entry.Key) -Value ([string]$entry.Value) }

$windowsConfig = [ordered]@{
    path_encoding='windows-native'; dsn=$envMap.POLIS_DSN; binary=$envMap.POLIS_CODEX_BINARY; code_mode_host=$envMap.POLIS_CODEX_CODE_MODE_HOST; auth_file=$envMap.POLIS_CODEX_AUTH_FILE; selected_config_path=$envMap.POLIS_SELECTED_CODEX_CONFIG; execution_manifest_path=$envMap.POLIS_V3_FRONTEND_EXECUTION_MANIFEST; qualification_path=$envMap.POLIS_V3_FRONTEND_QUALIFICATION; blob_durability_qualification_path=$envMap.POLIS_BLOB_DURABILITY_QUALIFICATION; behavioral_contract_path=$envMap.POLIS_V3_ACCEPTANCE_QUALIFICATION; checker_feedback_qualification_path=$envMap.POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION; postgres_dump_path=(Join-Path $Repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump'); postgres_snapshot_dsn=$envMap.POLIS_V3_POSTGRES_SNAPSHOT_DSN; runtime_root=$runtimeWindows; evidence_root=$evidenceWindows; recovery_package_root=(Join-Path $Repo "evidence\development\$BaselineId"); source_cas_root=$casRootWindows; runtime_artifact_manifest_path='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'; authorization_binding_path=$frontendBindingPath; runtime_database_binding_path=$databaseBindingPath; database_access_view_path=$databaseViewPath; runtime_database_binding_fingerprint=$databaseBindingFingerprint; database_access_strategy_revision=$databaseStrategyRevision; current_l1_evidence_path=$envMap.POLIS_CURRENT_L1_EVIDENCE; execution_fingerprint='708790c760a905f8eeced8ba426b024341eec789bdb79ad1d89f87cb31982091'; current_l1_fingerprint='c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'; handover_boundary_evidence_path=$evidenceWindows; handover_boundary_snapshot_path=$snapshotPath; problem_key='r03a-real-peer-collaboration-v1'; purpose='real_backend_peer_collaboration'; employee_id='emp-frontend'; model='gpt-5.6-luna'; effort='medium'; tool_call_limit=48; medium_limit=1; high_limit=0; concurrency=1; retry=$false; reset=$false; runtime_cas_binding=[ordered]@{canonical_root=$casRootWindows; layout_revision='r03a-cas-layout@1'; company_namespace=$company; required_blob_inventory_digest=$inventory; required_blob_count=4}; transport_policy=[ordered]@{transport_policy_revision=$policyRevision; initialize_timeout=30000000000; start_acknowledgement_timeout=30000000000; first_output_deadline=90000000000; reconnect_grace=30000000000; streaming_idle=90000000000; total_turn_deadline=600000000000; stop_reconciliation_timeout=5000000000}
}
$configWindows = Join-Path $runtimeWindows 'frontend-execution-config.windows.json'
$configWsl = Join-Path $runtimeWindows 'frontend-execution-config.wsl.json'
$configReport = Join-Path $runtimeWindows 'frontend-execution-config.wsl-report.json'
Write-FrontendConfigPair $windowsConfig $configWindows $configWsl
$bridgeReport = Invoke-FrontendWslConfigProbe $Repo $configWsl $configReport
if ($bridgeReport.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'typed Windows-to-WSL config bridge failed' }

function Invoke-WindowsV5([string]$Phase) {
    & $runnerExe $Phase
    if ($LASTEXITCODE -ne 0) { throw "Windows-native Frontend runner failed: $Phase ($LASTEXITCODE)" }
}

$pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
$pgLib = "$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu"
$pgStarted = $false
try {
    & bash -lc "export LD_LIBRARY_PATH='$pgLib'; mkdir -m 700 -p '$Socket'; '$pg/pg_ctl' -D '$pgDataWsl' -l '$cleanRootWsl/postgres/frontend-business-v5.log' -o '-k $Socket -h 127.0.0.1 -p $Port -c synchronous_commit=on' -w start"
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL startup failed' }
    $pgStarted = $true
    Invoke-WindowsV5 '-frontend-single-preflight'
    Invoke-WindowsV5 '-frontend-database-preflight'
    Invoke-WindowsV5 '-frontend-single-freshness'
    Invoke-WindowsV5 '-frontend-single'
} finally {
    if ($pgStarted) { & bash -lc "export LD_LIBRARY_PATH='$pgLib'; '$pg/pg_ctl' -D '$pgDataWsl' -m fast -w stop" }
    & bash -lc "rmdir -- '$Socket' 2>/dev/null || true"
}

Get-Content -Raw -LiteralPath $frontendResultPath
