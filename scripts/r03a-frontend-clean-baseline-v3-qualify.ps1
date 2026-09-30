param(
    [string]$BaselineId = 'r03a-frontend-clean-baseline-v3',
    [string]$DbName = 'polis_r0_3a_frontend_clean_baseline_v3',
    [int]$Port = 55438,
    [string]$Socket = '/tmp/polis-pg-r03a-frontend-baseline-v3',
    [string]$TransportPolicyRevision = 'r03a-transport-policy@1',
    [string]$ExecutionEnvelopeFingerprint = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51',
    [string]$CASManifestPath = '',
    [string]$RuntimeEvidenceName = 'runtime-evidence-v2'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$id = $BaselineId
$evidenceWindows = Join-Path $Repo "evidence\development\$id"
$evidenceWsl = Convert-WindowsPathToWsl $evidenceWindows
$runtimeWindows = Join-Path $evidenceWindows 'windows-runtime'
$cleanRootWsl = "/home/chyinan/.local/state/polis-recovery/$id"
$pgDataWsl = "$cleanRootWsl/postgres/pgdata"
$casRootWsl = "$cleanRootWsl/blobs"
$casRootWindows = Convert-WslPathToWindows $casRootWsl
$socket = $Socket
$port = $Port
$db = $DbName
$company = 'r03a-pagination-v3-company-1789379324974307400'
$runnerExe = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$baselinePath = Join-Path $Repo "evidence\development\$id\FrontendExecutionBaselineManifest.json"
$baselineHash = (Get-Content -Raw -LiteralPath "$baselinePath.sha256").Trim()
$casManifestPathOverride = $CASManifestPath
$casManifestPath = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
if (-not [string]::IsNullOrWhiteSpace($casManifestPathOverride)) { $casManifestPath = $casManifestPathOverride }
$pgLib = "$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu"

if (-not (Test-Path -LiteralPath $runnerExe -PathType Leaf)) { throw 'Windows runner executable is missing' }
New-Item -ItemType Directory -Force -Path $evidenceWindows,$runtimeWindows | Out-Null

$windowsConfig = [ordered]@{
    path_encoding='windows-native'; dsn="host=127.0.0.1 port=$port dbname=$db user=polis_runtime"; binary='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'; code_mode_host='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'; auth_file='C:\Users\chyinan\.codex\auth.json'; selected_config_path=(Join-Path $Repo '.runtime\linux\r03a-t14c\home\config.toml'); execution_manifest_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-frontend-l2\execution-manifest.json'); qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-frontend-l2\offline-result.json'); blob_durability_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-blob-durability\qualification.json'); behavioral_contract_path=(Join-Path $Repo 'evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json'); checker_feedback_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json'); postgres_dump_path=(Join-Path $Repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump'); postgres_snapshot_dsn="host=127.0.0.1 port=$port dbname=$db user=chyinan"; runtime_root=$runtimeWindows; evidence_root=$evidenceWindows; recovery_package_root=(Join-Path $Repo "evidence\development\$id"); source_cas_root=$casRootWindows; runtime_artifact_manifest_path='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'; authorization_binding_path=(Join-Path $Repo 'evidence\development\r0.3a-pagination-v3-real-frontend-clean-single-session-v4\frontend-binding.json'); current_l1_evidence_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-l1'); execution_fingerprint='708790c760a905f8eeced8ba426b024341eec789bdb79ad1d89f87cb31982091'; current_l1_fingerprint='c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'; handover_boundary_evidence_path=$evidenceWindows; handover_boundary_snapshot_path=(Join-Path $runtimeWindows 'unused.dump'); problem_key='r03a-real-peer-collaboration-v1'; purpose='real_backend_peer_collaboration'; employee_id='emp-frontend'; model='gpt-5.6-luna'; effort='medium'; tool_call_limit=48; medium_limit=1; high_limit=0; concurrency=1; retry=$false; reset=$false; runtime_cas_binding=[ordered]@{canonical_root=$casRootWindows; layout_revision='r03a-cas-layout@1'; company_namespace=$company; required_blob_inventory_digest='e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'; required_blob_count=4}; transport_policy=[ordered]@{transport_policy_revision=$TransportPolicyRevision; initialize_timeout=30000000000; start_acknowledgement_timeout=30000000000; first_output_deadline=90000000000; reconnect_grace=30000000000; streaming_idle=90000000000; total_turn_deadline=600000000000; stop_reconciliation_timeout=5000000000}
}
$configWindows = Join-Path $evidenceWindows 'frontend-execution-config.windows.json'
$configWsl = Join-Path $evidenceWindows 'frontend-execution-config.wsl.json'
$configReport = Join-Path $evidenceWindows 'frontend-execution-config.wsl-report.json'
Write-FrontendConfigPair $windowsConfig $configWindows $configWsl
$bridgeReport = Invoke-FrontendWslConfigProbe $Repo $configWsl $configReport
if ($bridgeReport.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'v3 typed config bridge failed' }

$env:POLIS_CODEX_BINARY = $windowsConfig.binary
$env:POLIS_CODEX_CODE_MODE_HOST = $windowsConfig.code_mode_host
$env:POLIS_CODEX_AUTH_FILE = $windowsConfig.auth_file
$env:POLIS_V3_TRANSPORT_POLICY_REVISION = $TransportPolicyRevision
$env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $ExecutionEnvelopeFingerprint
$env:POLIS_V3_RUNTIME_ROOT = $runtimeWindows
$runtimeEvidenceWindows = Join-Path $evidenceWindows $RuntimeEvidenceName
New-Item -ItemType Directory -Force -Path $runtimeEvidenceWindows | Out-Null
$env:POLIS_V3_EVIDENCE = $runtimeEvidenceWindows
$env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT = Join-Path $evidenceWindows 'frontend-runtime-preflight.json'
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST = $baselinePath
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $baselineHash
$env:POLIS_V3_FRONTEND_CAS_MANIFEST = $casManifestPath
$env:POLIS_V3_FRONTEND_CAS_ROOT = $casRootWindows
$env:POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'
$env:POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = $company
$env:POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'
$env:POLIS_DSN = "host=127.0.0.1 port=$port dbname=$db user=polis_runtime"
$env:R03A_BASELINE_ID = $id
$env:R03A_WSL_CAS_ROOT = $casRootWsl
$env:R03A_WINDOWS_CAS_ROOT = $casRootWindows

& $runnerExe -frontend-runtime-preflight
if ($LASTEXITCODE -ne 0) { throw 'v3 Windows-native runtime preflight failed' }
$env:POLIS_V3_EVIDENCE = $evidenceWindows

$pgStarted = $false
try {
    & bash -lc "export LD_LIBRARY_PATH='$pgLib'; mkdir -m 700 -p '$socket'; '$pg/pg_ctl' -D '$pgDataWsl' -l '$cleanRootWsl/postgres/v3-qualification.log' -o '-k $socket -h 127.0.0.1 -p $port -c synchronous_commit=on' -w start"
    if ($LASTEXITCODE -ne 0) { throw 'v3 PostgreSQL startup failed' }
    $pgStarted = $true
    $dsn = "host=127.0.0.1 port=$port dbname=$db user=polis_runtime"
    $probeOutputs = @('starting-state-probe-1.json','starting-state-probe-2.json','starting-state-probe-post-1.json','starting-state-probe-post-2.json')
    & bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn '$dsn' -output '$evidenceWsl/starting-state-probe-1.json'"
    & bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn '$dsn' -output '$evidenceWsl/starting-state-probe-2.json'"
    $env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT = Join-Path $evidenceWindows 'frontend-activation-preflight-1.json'
    & $runnerExe -frontend-activation-preflight
    if ($LASTEXITCODE -ne 0) { throw 'v3 activation preflight 1 failed' }
    $env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT = Join-Path $evidenceWindows 'frontend-activation-preflight-2.json'
    & $runnerExe -frontend-activation-preflight
    if ($LASTEXITCODE -ne 0) { throw 'v3 activation preflight 2 failed' }
    & bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn '$dsn' -output '$evidenceWsl/starting-state-probe-post-1.json'"
    & bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn '$dsn' -output '$evidenceWsl/starting-state-probe-post-2.json'"
} finally {
    if ($pgStarted) { & bash -lc "export LD_LIBRARY_PATH='$pgLib'; '$pg/pg_ctl' -D '$pgDataWsl' -m fast -w stop" }
    & bash -lc "rmdir -- '$socket' 2>/dev/null || true"
}

python3 (Join-Path $Repo 'scripts\r03a-frontend-clean-baseline-v3-finalize.py') $evidenceWindows
