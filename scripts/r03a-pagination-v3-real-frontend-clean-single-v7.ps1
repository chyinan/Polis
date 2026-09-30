param(
    [string]$EvidenceId = 'r03a-pagination-v3-real-frontend-clean-single-session-windows-v7',
    [string]$RuntimeId = 'r03a-frontend-business-v7',
    [string]$BaselineId = 'r03a-frontend-clean-baseline-v7',
    [string]$DbName = 'polis_r0_3a_frontend_clean_baseline_v7',
    [int]$Port = 55432,
    [string]$Socket = '/tmp/polis-pg-r03a-frontend-business-v7'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$Evidence = Join-Path $Repo "evidence\development\$EvidenceId"
$RuntimeWindows = Join-Path $Repo ".runtime\windows\$RuntimeId"
$BaselinePath = Join-Path $Repo "evidence\development\$BaselineId\FrontendExecutionBaselineManifest.json"
$BaselineHash = (Get-Content -Raw -LiteralPath "$BaselinePath.sha256").Trim()
$Runner = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$BindingPath = Join-Path $Repo "evidence\development\$BaselineId\runtime-database-binding.json"
$ViewPath = Join-Path $RuntimeWindows 'windows-database-access-view-v2.json'
$CasManifestPath = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$FrontendBindingPath = Join-Path $Evidence 'frontend-binding.json'
$FrontendResultPath = Join-Path $Evidence 'result.json'
$AllowancePath = Join-Path $Evidence 'allowance.json'
$RuntimeSnapshot = Join-Path $RuntimeWindows 'unused-frontend-snapshot.dump'
$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$Helper = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
$Auth = 'C:\Users\chyinan\.codex\auth.json'
$Company = 'r03a-pagination-v3-company-1789379324974307400'
$Inventory = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'
$NativeEnvelope = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51'
$PolicyRevision = 'r03a-transport-policy@1'
$DatabaseStrategy = 'r03a-windows-wsl-direct-tcp@1'
$ConsumptionQualification = Join-Path $Repo 'evidence\development\r0.3a-frontend-public-consumption-abi-remediation\qualification.json'
$ConsumptionContractRevision = 'r03a-frontend-consumption-contract@1'
$FrontendBindingRevision = 'r03a-frontend-binding@1'
$BehaviorVerifierRevision = 'r03a-frontend-pagination-behavior@1'

function Get-FileDigest([string]$Path) {
    $hash = [Security.Cryptography.SHA256]::Create().ComputeHash([IO.File]::ReadAllBytes($Path))
    return ([BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function Write-JsonValue([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    [IO.File]::WriteAllText($Path, (($Value | ConvertTo-Json -Depth 40) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

function Resolve-WSLHost {
    $raw = @(& wsl.exe -d Ubuntu-22.04 -- hostname -I | ForEach-Object { ([string]$_).Trim() })
    $resolved = ($raw -join ' ') -split '\s+' | Where-Object { $_ -match '^(?!127\.)(?:\d{1,3}\.){3}\d{1,3}$' } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($resolved)) { throw 'unable to resolve current WSL IPv4 address' }
    return $resolved
}

function Resolve-WSLGateway {
    $raw = @(& wsl.exe -d Ubuntu-22.04 -- sh -c 'ip route show default' | ForEach-Object { ([string]$_).Trim() })
    $resolved = ($raw -join ' ') -split '\s+' | Where-Object { $_ -match '^(?:\d{1,3}\.){3}\d{1,3}$' } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($resolved)) { throw 'unable to resolve current WSL gateway' }
    return $resolved
}

function Invoke-Runner([string]$Flag) {
    & $Runner $Flag
    if ($LASTEXITCODE -ne 0) { throw "Windows-native Frontend runner failed: $Flag ($LASTEXITCODE)" }
}

if (Test-Path -LiteralPath $Evidence) {
    if (@(Get-ChildItem -LiteralPath $Evidence -Force).Count -gt 0) { throw 'V7 business evidence path is not fresh' }
} else { New-Item -ItemType Directory -Force -Path $Evidence | Out-Null }
if (-not (Test-Path -LiteralPath $Runner -PathType Leaf)) { throw 'Windows-native business runner is unavailable' }
if (-not (Test-Path -LiteralPath $BaselinePath -PathType Leaf)) { throw 'qualified V7 baseline manifest is unavailable' }
if (-not (Test-Path -LiteralPath $BindingPath -PathType Leaf)) { throw 'qualified V7 RuntimeDatabaseBinding is unavailable' }

$binding = Get-Content -Raw -LiteralPath $BindingPath | ConvertFrom-Json
$resolvedHost = Resolve-WSLHost
$resolvedGateway = Resolve-WSLGateway
$view = [ordered]@{schema_version='r03a-database-access-view@2'; consumer_os='windows'; transport='tcp'; host=$resolvedHost; port=$Port; endpoint_source='v7-business-wsl-direct-tcp'; access_strategy_revision=$DatabaseStrategy; runtime_database_binding_fingerprint=$binding.fingerprint; target_wsl_distribution='Ubuntu-22.04'; resolution_method='wsl.exe -d Ubuntu-22.04 -- hostname -I'; hba_source_address="$resolvedGateway/32"}
Write-JsonValue $ViewPath $view

$windowsConfig = [ordered]@{
    path_encoding='windows-native'; dsn="host=$resolvedHost port=$Port dbname=$DbName user=polis_runtime"; binary=$Binary; code_mode_host=$Helper; auth_file=$Auth; selected_config_path=(Join-Path $Repo '.runtime\linux\r03a-t14c\home\config.toml'); execution_manifest_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-evidence-contract-hardening-l2-offline\execution-manifest.json'); qualification_path=(Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\result.json'); blob_durability_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-blob-durability\qualification.json'); behavioral_contract_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-public-consumption-abi-remediation\qualification.json'); checker_feedback_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json'); postgres_dump_path=(Join-Path $Repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump'); postgres_snapshot_dsn="host=$resolvedHost port=$Port dbname=$DbName user=chyinan"; runtime_root=$RuntimeWindows; evidence_root=$Evidence; recovery_package_root=(Join-Path $Repo "evidence\development\$BaselineId"); source_cas_root=(Convert-WslPathToWindows "/home/chyinan/.local/state/polis-recovery/$BaselineId/blobs"); runtime_artifact_manifest_path='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'; runtime_database_binding_path=$BindingPath; database_access_view_path=$ViewPath; runtime_database_binding_fingerprint=$binding.fingerprint; database_access_strategy_revision=$DatabaseStrategy; authorization_binding_path=$FrontendBindingPath; current_l1_evidence_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-l1'); execution_fingerprint=$NativeEnvelope; current_l1_fingerprint='c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'; handover_boundary_evidence_path=$Evidence; handover_boundary_snapshot_path=$RuntimeSnapshot; problem_key='r03a-real-peer-collaboration-v1'; purpose='real_backend_peer_collaboration'; employee_id='emp-frontend'; model='gpt-5.6-luna'; effort='medium'; tool_call_limit=48; medium_limit=1; high_limit=0; concurrency=1; retry=$false; reset=$false; runtime_cas_binding=[ordered]@{canonical_root=(Convert-WslPathToWindows "/home/chyinan/.local/state/polis-recovery/$BaselineId/blobs"); layout_revision='r03a-cas-layout@1'; company_namespace=$Company; required_blob_inventory_digest=$Inventory; required_blob_count=4}; transport_policy=[ordered]@{transport_policy_revision=$PolicyRevision; initialize_timeout=30000000000; start_acknowledgement_timeout=30000000000; first_output_deadline=90000000000; reconnect_grace=30000000000; streaming_idle=90000000000; total_turn_deadline=600000000000; stop_reconciliation_timeout=5000000000}
}
$ConfigWindows = Join-Path $RuntimeWindows 'frontend-execution-config.windows.json'
$ConfigWsl = Join-Path $RuntimeWindows 'frontend-execution-config.wsl.json'
$ConfigReport = Join-Path $RuntimeWindows 'frontend-execution-config.wsl-report.json'
Write-FrontendConfigPair $windowsConfig $ConfigWindows $ConfigWsl
$bridge = Invoke-FrontendWslConfigProbe $Repo $ConfigWsl $ConfigReport
if ($bridge.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'V7 business typed config bridge failed' }

$env:POLIS_DSN = $windowsConfig.dsn
$env:POLIS_V3_RUNTIME_ROOT = $RuntimeWindows
$env:POLIS_V3_EVIDENCE = $Evidence
$env:POLIS_CODEX_BINARY = $Binary; $env:POLIS_CODEX_CODE_MODE_HOST = $Helper; $env:POLIS_CODEX_AUTH_FILE = $Auth; $env:POLIS_SELECTED_CODEX_CONFIG = $windowsConfig.selected_config_path; $env:POLIS_V3_RUNTIME_ROOT = $RuntimeWindows; $env:POLIS_V3_EVIDENCE = $Evidence; $env:POLIS_V3_TRANSPORT_POLICY_REVISION = $PolicyRevision; $env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $NativeEnvelope; $env:POLIS_V3_FRONTEND_EXECUTION_MANIFEST = $windowsConfig.execution_manifest_path; $env:POLIS_V3_FRONTEND_QUALIFICATION = $windowsConfig.qualification_path; $env:POLIS_V3_FRONTEND_L2_LIVE = $windowsConfig.qualification_path; $env:POLIS_V3_ACCEPTANCE_QUALIFICATION = (Join-Path $Repo 'evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json'); $env:POLIS_FRONTEND_CONSUMPTION_QUALIFICATION = $ConsumptionQualification; $env:POLIS_FRONTEND_CONSUMPTION_CONTRACT_REVISION = $ConsumptionContractRevision; $env:POLIS_FRONTEND_BINDING_CONTRACT_REVISION = $FrontendBindingRevision; $env:POLIS_FRONTEND_BEHAVIOR_VERIFIER_REVISION = $BehaviorVerifierRevision; $env:POLIS_RUNTIME_DATABASE_BINDING = $BindingPath; $env:POLIS_WINDOWS_DATABASE_ACCESS_VIEW = $ViewPath; $env:POLIS_RUNTIME_DATABASE_BINDING_FINGERPRINT = $binding.fingerprint; $env:POLIS_DATABASE_ACCESS_STRATEGY_REVISION = $DatabaseStrategy; $env:POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT = Join-Path $RuntimeWindows 'windows-database-preflight.json'; $env:POLIS_V3_FRONTEND_BASELINE_MANIFEST = $BaselinePath; $env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $BaselineHash; $env:POLIS_V3_FRONTEND_CAS_MANIFEST = $CasManifestPath; $env:POLIS_V3_FRONTEND_CAS_ROOT = $windowsConfig.runtime_cas_binding.canonical_root; $env:POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'; $env:POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = $Company; $env:POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = $Inventory; $env:POLIS_CURRENT_L1_EVIDENCE = $windowsConfig.current_l1_evidence_path
$env:POLIS_BLOB_DURABILITY_QUALIFICATION = (Join-Path $Repo 'evidence\development\r0.3a-blob-durability\qualification.json')
$env:POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION = (Join-Path $Repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json')
$env:POLIS_CURRENT_L1_EVIDENCE = $windowsConfig.current_l1_evidence_path
$env:POLIS_V3_BACKEND_EXECUTION_MANIFEST = (Join-Path $Repo 'evidence\development\r0.3a-current-binary-backend-l2-v6\execution-manifest.json')
$env:POLIS_V3_BACKEND_QUALIFICATION = (Join-Path $Repo 'evidence\development\r0.3a-current-binary-backend-l2-target-live-v1\result.json')
$env:POLIS_V3_BACKEND_L2_LIVE = (Join-Path $Repo 'evidence\development\r0.3a-current-binary-backend-l2-target-live-v1\result.json')
$env:POLIS_V3_BACKEND_BINDING = (Join-Path $Repo 'evidence\development\r03a-pagination-v3-backend-sample-6\authorization-binding.json')
$env:POLIS_V3_FRONTEND_BINDING = $FrontendBindingPath
$env:POLIS_V3_ALLOWANCE = $AllowancePath
$env:POLIS_V3_FRONTEND_RESULT = $FrontendResultPath

try {
    $hbaPath = Convert-WslPathToWindows "/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/pgdata/pg_hba.conf"
    $hbaRule = "host $DbName polis_runtime $resolvedGateway/32 trust"
    if (-not (Select-String -LiteralPath $hbaPath -SimpleMatch -Pattern $hbaRule -Quiet)) { Add-Content -LiteralPath $hbaPath -Value $hbaRule }
    & bash -lc "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu'; mkdir -m 700 -p '$Socket'; '$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_ctl' -D '/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/pgdata' -l '/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/v7-business.log' -o '-k $Socket -h $resolvedHost -p $Port -c fsync=on -c synchronous_commit=on -c jit=off' -w start"
    if ($LASTEXITCODE -ne 0) { throw 'V7 business PostgreSQL startup failed' }
    Invoke-Runner '-frontend-database-preflight'
    Invoke-Runner '-frontend-single-preflight'
    Invoke-Runner '-frontend-single-freshness'
    Invoke-Runner '-frontend-single'
} finally {
    & bash -lc "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu'; '$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_ctl' -D '/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/pgdata' -m fast -w stop >/dev/null 2>&1 || true; rmdir -- '$Socket' 2>/dev/null || true"
}

Get-Content -Raw -LiteralPath $FrontendResultPath
