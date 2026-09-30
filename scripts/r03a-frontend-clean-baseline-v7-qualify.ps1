param(
    [string]$BaselineId = 'r03a-frontend-clean-baseline-v7',
    [string]$DbName = 'polis_r0_3a_frontend_clean_baseline_v7',
    [int]$Port = 55432,
    [string]$Socket = '/tmp/polis-pg-r03a-frontend-clean-baseline-v7'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$Evidence = Join-Path $Repo "evidence\development\$BaselineId"
$RootWsl = "/home/chyinan/.local/state/polis-recovery/$BaselineId"
$PgDataWsl = "$RootWsl/postgres/pgdata"
$CasRootWsl = "$RootWsl/blobs"
$CasRootWindows = Convert-WslPathToWindows $CasRootWsl
$Runner = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$BindingPath = Join-Path $Evidence 'runtime-database-binding.json'
$ViewPath = Join-Path $Evidence 'windows-database-access-view-v2.json'
$BaselinePath = Join-Path $Evidence 'FrontendExecutionBaselineManifest.json'
$CasManifestPath = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$CurrentL2Path = Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\frontend-l2-binding.json'
$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$Helper = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
$Auth = 'C:\Users\chyinan\.codex\auth.json'
$RuntimeWindows = Join-Path $Evidence 'windows-runtime'
$RuntimeEvidence = Join-Path $Evidence ("runtime-evidence-v7-" + [DateTime]::UtcNow.ToString('yyyyMMddHHmmss'))
$PolicyRevision = 'r03a-transport-policy@1'
$NativeEnvelope = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51'
$DatabaseStrategy = 'r03a-windows-wsl-direct-tcp@1'
$Company = 'r03a-pagination-v3-company-1789379324974307400'
$Inventory = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'

function Get-JsonDigest($Value) {
    $raw = $Value | ConvertTo-Json -Depth 40 -Compress
    $bytes = [Text.Encoding]::UTF8.GetBytes($raw)
    $hash = [Security.Cryptography.SHA256]::Create().ComputeHash($bytes)
    return ([BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function Get-FileDigest([string]$Path) {
    $hash = [Security.Cryptography.SHA256]::Create().ComputeHash([IO.File]::ReadAllBytes($Path))
    return ([BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
}

function Write-JsonValue([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    [IO.File]::WriteAllText($Path, (($Value | ConvertTo-Json -Depth 40) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

function Set-JsonProperty($Object, [string]$Name, $Value) {
    if ($null -ne $Object.PSObject.Properties[$Name]) {
        $Object.PSObject.Properties[$Name].Value = $Value
    } else {
        $Object | Add-Member -NotePropertyName $Name -NotePropertyValue $Value
    }
}

function Invoke-Runner([string]$Flag) {
    & $Runner $Flag
    if ($LASTEXITCODE -ne 0) { throw "runner failed: $Flag ($LASTEXITCODE)" }
}

function Resolve-WSLHost {
    $raw = @(& wsl.exe -d Ubuntu-22.04 -- hostname -I | ForEach-Object { ([string]$_).Trim() })
    $resolved = ($raw -join ' ') -split '\s+' | Where-Object { $_ -match '^(?!127\.)(?:\d{1,3}\.){3}\d{1,3}$' } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($resolved)) { throw 'unable to resolve a non-loopback WSL IPv4 address' }
    return $resolved
}

function Resolve-WSLGateway {
    $raw = @(& wsl.exe -d Ubuntu-22.04 -- sh -c 'ip route show default' | ForEach-Object { ([string]$_).Trim() })
    $resolved = ($raw -join ' ') -split '\s+' | Where-Object { $_ -match '^(?:\d{1,3}\.){3}\d{1,3}$' } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($resolved)) { throw 'unable to resolve the WSL default gateway for PostgreSQL HBA binding' }
    return $resolved
}

function Update-Endpoint([string]$EndpointHost) {
    $script:ResolvedHost = $EndpointHost
    $script:ResolvedGateway = Resolve-WSLGateway
    $script:view = [ordered]@{schema_version='r03a-database-access-view@2'; consumer_os='windows'; transport='tcp'; host=$EndpointHost; port=$Port; endpoint_source='v7-qualified-wsl-direct-tcp'; access_strategy_revision=$DatabaseStrategy; runtime_database_binding_fingerprint=$binding.fingerprint; target_wsl_distribution='Ubuntu-22.04'; resolution_method='wsl.exe -d Ubuntu-22.04 -- hostname -I'; hba_source_address="$script:ResolvedGateway/32"}
    Write-JsonValue $ViewPath $script:view
    $windowsConfig['dsn'] = "host=$EndpointHost port=$Port dbname=$DbName user=polis_runtime"
    $windowsConfig['postgres_snapshot_dsn'] = "host=$EndpointHost port=$Port dbname=$DbName user=chyinan"
    Write-FrontendConfigPair $windowsConfig $ConfigWindows $ConfigWsl
    $bridge = Invoke-FrontendWslConfigProbe $Repo $ConfigWsl $ConfigReport
    if ($bridge.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'V7 typed config bridge failed after endpoint resolution' }
    $env:POLIS_DSN = "host=$EndpointHost port=$Port dbname=$DbName user=polis_runtime"
}

function Start-V7Postgres {
    $hbaWindows = Convert-WslPathToWindows "$PgDataWsl/pg_hba.conf"
    $hbaRule = "host $DbName polis_runtime $script:ResolvedGateway/32 trust"
    if (-not (Select-String -LiteralPath $hbaWindows -SimpleMatch -Pattern $hbaRule -Quiet)) {
        Add-Content -LiteralPath $hbaWindows -Value $hbaRule
    }
    & bash -lc "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu'; mkdir -m 700 -p '$Socket'; '$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_ctl' -D '$PgDataWsl' -l '$RootWsl/postgres/v7-qualification.log' -o '-k $Socket -h $script:ResolvedHost -p $Port -c fsync=on -c synchronous_commit=on -c jit=off' -w start"
    if ($LASTEXITCODE -ne 0) { throw 'V7 PostgreSQL startup failed' }
}

function Stop-V7Postgres {
    & bash -lc "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu'; '$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_ctl' -D '$PgDataWsl' -m fast -w stop >/dev/null 2>&1 || true; rmdir -- '$Socket' 2>/dev/null || true"
    Start-Sleep -Milliseconds 500
}

function Wait-WindowsDatabase([string]$CycleName) {
    for ($attempt = 1; $attempt -le 60; $attempt++) {
        $output = Join-Path $Evidence "windows-database-preflight-v2-$CycleName-$attempt.json"
        $env:POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT = $output
        & $Runner '-frontend-database-preflight'
        $exitCode = $LASTEXITCODE
        if (Test-Path -LiteralPath $output) {
            $report = Get-Content -Raw -LiteralPath $output | ConvertFrom-Json
            if ($report.status -eq 'WINDOWS_DATABASE_ACCESS_PREFLIGHT_PASSED' -and -not $report.mutation) {
                return [ordered]@{cycle=$CycleName; attempts=$attempt; report=$report}
            }
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Windows database access did not become ready for cycle $CycleName"
}

if (-not (Test-Path -LiteralPath $Runner -PathType Leaf)) { throw 'Windows runner executable is unavailable' }
if (-not (Test-Path -LiteralPath $BindingPath -PathType Leaf)) { throw 'V7 RuntimeDatabaseBinding is unavailable' }
if (-not (Test-Path -LiteralPath $BaselinePath -PathType Leaf)) { throw 'V7 baseline manifest is unavailable' }

$binding = Get-Content -Raw -LiteralPath $BindingPath | ConvertFrom-Json
$ResolvedHost = Resolve-WSLHost
$ResolvedGateway = Resolve-WSLGateway
$view = [ordered]@{schema_version='r03a-database-access-view@2'; consumer_os='windows'; transport='tcp'; host=$ResolvedHost; port=$Port; endpoint_source='v7-qualified-wsl-direct-tcp'; access_strategy_revision=$DatabaseStrategy; runtime_database_binding_fingerprint=$binding.fingerprint; target_wsl_distribution='Ubuntu-22.04'; resolution_method='wsl.exe -d Ubuntu-22.04 -- hostname -I'; hba_source_address="$ResolvedGateway/32"}
Write-JsonValue $ViewPath $view
$l2 = Get-Content -Raw -LiteralPath $CurrentL2Path | ConvertFrom-Json
$baselineHash = Get-FileDigest $BaselinePath
$stack = [ordered]@{
    frontend_consumption_contract = 'r03a-frontend-consumption-contract@1'
    frontend_binding = 'r03a-frontend-binding@1'
    frontend_behavior_verifier = 'r03a-frontend-pagination-behavior@1'
    qualification_evidence = 'r0.3a-frontend-public-consumption-abi-remediation/qualification.json'
}
$stackDigest = Get-JsonDigest $stack

$windowsConfig = [ordered]@{
    path_encoding='windows-native'
    dsn="host=$ResolvedHost port=$Port dbname=$DbName user=polis_runtime"
    binary=$Binary
    code_mode_host=$Helper
    auth_file=$Auth
    selected_config_path=(Join-Path $Repo '.runtime\linux\r03a-t14c\home\config.toml')
    execution_manifest_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-evidence-contract-hardening-l2-offline\execution-manifest.json')
    qualification_path=(Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\result.json')
    blob_durability_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-blob-durability\qualification.json')
    behavioral_contract_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-public-consumption-abi-remediation\qualification.json')
    checker_feedback_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json')
    postgres_dump_path=(Join-Path $Repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump')
    postgres_snapshot_dsn="host=$ResolvedHost port=$Port dbname=$DbName user=chyinan"
    runtime_root=$RuntimeWindows
    evidence_root=$Evidence
    recovery_package_root=(Join-Path $Repo 'evidence\development\r03a-frontend-clean-baseline-v7')
    source_cas_root=$CasRootWindows
    runtime_artifact_manifest_path='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'
    runtime_database_binding_path=$BindingPath
    database_access_view_path=$ViewPath
    runtime_database_binding_fingerprint=$binding.fingerprint
    database_access_strategy_revision=$DatabaseStrategy
    authorization_binding_path=(Join-Path $Evidence 'frontend-binding.json')
    current_l1_evidence_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-l1')
    execution_fingerprint=$NativeEnvelope
    current_l1_fingerprint='c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'
    handover_boundary_evidence_path=$Evidence
    handover_boundary_snapshot_path=(Join-Path $RuntimeWindows 'unused.dump')
    problem_key='r03a-real-peer-collaboration-v1'
    purpose='real_backend_peer_collaboration'
    employee_id='emp-frontend'
    model='gpt-5.6-luna'
    effort='medium'
    tool_call_limit=48
    medium_limit=1
    high_limit=0
    concurrency=1
    retry=$false
    reset=$false
    runtime_cas_binding=[ordered]@{canonical_root=$CasRootWindows; layout_revision='r03a-cas-layout@1'; company_namespace=$Company; required_blob_inventory_digest=$Inventory; required_blob_count=4}
    transport_policy=[ordered]@{transport_policy_revision=$PolicyRevision; initialize_timeout=30000000000; start_acknowledgement_timeout=30000000000; first_output_deadline=90000000000; reconnect_grace=30000000000; streaming_idle=90000000000; total_turn_deadline=600000000000; stop_reconciliation_timeout=5000000000}
}
$ConfigWindows = Join-Path $Evidence 'frontend-execution-config.windows.json'
$ConfigWsl = Join-Path $Evidence 'frontend-execution-config.wsl.json'
$ConfigReport = Join-Path $Evidence 'frontend-execution-config.wsl-report.json'
Write-FrontendConfigPair $windowsConfig $ConfigWindows $ConfigWsl
$bridge = Invoke-FrontendWslConfigProbe $Repo $ConfigWsl $ConfigReport
if ($bridge.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'V7 typed config bridge failed' }

$env:POLIS_DSN = "host=$ResolvedHost port=$Port dbname=$DbName user=polis_runtime"
$env:POLIS_CODEX_BINARY = $Binary
$env:POLIS_CODEX_CODE_MODE_HOST = $Helper
$env:POLIS_CODEX_AUTH_FILE = $Auth
$env:POLIS_V3_RUNTIME_ROOT = $RuntimeWindows
$env:POLIS_V3_EVIDENCE = $Evidence
$env:POLIS_V3_TRANSPORT_POLICY_REVISION = $PolicyRevision
$env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $NativeEnvelope
$env:POLIS_V3_FRONTEND_EXECUTION_MANIFEST = (Join-Path $Repo 'evidence\development\r0.3a-frontend-evidence-contract-hardening-l2-offline\execution-manifest.json')
$env:POLIS_V3_FRONTEND_QUALIFICATION = (Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\result.json')
$env:POLIS_V3_FRONTEND_L2_LIVE = (Join-Path $Repo 'evidence\development\r03a-frontend-current-l2-live-v5\result.json')
$env:POLIS_RUNTIME_DATABASE_BINDING = $BindingPath
$env:POLIS_WINDOWS_DATABASE_ACCESS_VIEW = $ViewPath
$env:POLIS_RUNTIME_DATABASE_BINDING_FINGERPRINT = $binding.fingerprint
$env:POLIS_DATABASE_ACCESS_STRATEGY_REVISION = $DatabaseStrategy
$env:POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT = Join-Path $Evidence 'windows-database-preflight.json'
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST = $BaselinePath
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $baselineHash
$env:POLIS_V3_FRONTEND_CAS_MANIFEST = $CasManifestPath
$env:POLIS_V3_FRONTEND_CAS_ROOT = $CasRootWindows
$env:POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'
$env:POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = $Company
$env:POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = $Inventory
$env:POLIS_CURRENT_L1_EVIDENCE = Join-Path $Repo 'evidence\development\r0.3a-current-binary-l1'

try {
    Start-V7Postgres
    $cycleReports = @()
    $cycleReports += Wait-WindowsDatabase 'start-1'

    Stop-V7Postgres
    Update-Endpoint (Resolve-WSLHost)
    Start-V7Postgres
    $cycleReports += Wait-WindowsDatabase 'restart-2'

    Stop-V7Postgres
    & wsl.exe --shutdown
    if ($LASTEXITCODE -ne 0) { throw 'WSL shutdown boundary failed' }
    Update-Endpoint (Resolve-WSLHost)
    Start-V7Postgres
    $cycleReports += Wait-WindowsDatabase 'wsl-restart-3'
    Write-JsonValue (Join-Path $Evidence 'database-access-cycles-v2.json') ([ordered]@{strategy=$DatabaseStrategy; endpoint_resolution='runtime WSL IPv4'; cycles=$cycleReports; mutation=$false})
    $dbReport = $cycleReports[-1].report

    New-Item -ItemType Directory -Force -Path $RuntimeEvidence | Out-Null
    $env:POLIS_V3_EVIDENCE = $RuntimeEvidence
    $env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT = Join-Path $Evidence 'frontend-runtime-preflight-v7.json'
    Invoke-Runner '-frontend-runtime-preflight'
    $runtime = Get-Content -Raw -LiteralPath $env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT | ConvertFrom-Json
    if ($runtime.status -ne 'FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED' -or $runtime.registered_tool_count -ne 12 -or $runtime.pagination_runtime_preflight -ne 'PASSED' -or $runtime.provider_egress -or $runtime.business_mutation -or $runtime.allowance_created -or -not $runtime.stop_confirmed) { throw 'V7 runtime activation preflight failed' }
	$env:POLIS_V3_EVIDENCE = $Evidence

    $env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT = Join-Path $Evidence 'frontend-activation-preflight-before-manifest-v7.json'
    Invoke-Runner '-frontend-activation-preflight'

    $manifest = Get-Content -Raw -LiteralPath $BaselinePath | ConvertFrom-Json
    Set-JsonProperty $manifest 'baseline_generation_id' $BaselineId
    Set-JsonProperty $manifest 'baseline_manifest_revision' 'r03a-frontend-execution-baseline-manifest@7'
    Set-JsonProperty $manifest.postgres 'cluster_system_identifier' ([string]$binding.cluster_system_identifier)
    Set-JsonProperty $manifest.postgres 'database_name' ([string]$binding.database_name)
    Set-JsonProperty $manifest.postgres 'database_oid' ([string]$binding.database_oid)
    Set-JsonProperty $manifest 'runtime_database_binding' $binding
    Set-JsonProperty $manifest 'database_access_strategy' $DatabaseStrategy
    Set-JsonProperty $manifest 'windows_database_access_view' ([ordered]@{access_path="tcp://$ResolvedHost`:$Port"; access_view_fingerprint=(Get-JsonDigest $view); access_strategy_revision=$DatabaseStrategy; runtime_database_binding_fingerprint=$binding.fingerprint; endpoint_source='v7-qualified-wsl-direct-tcp'; authoritative_inventory_digest=$Inventory; consumer_os='windows'; transport='tcp'; host=$ResolvedHost; port=$Port; target_wsl_distribution='Ubuntu-22.04'; resolution_method='wsl.exe -d Ubuntu-22.04 -- hostname -I'; hba_source_address="$ResolvedGateway/32"; source_wsl_path=$CasRootWsl; translation_method='wslpath -a -w'})
    Set-JsonProperty $manifest 'frontend_consumption_stack' $stack
    Set-JsonProperty $manifest 'frontend_consumption_stack_digest' $stackDigest
    Set-JsonProperty $manifest 'frontend_l2_binding' ([ordered]@{qualification='R0.3A-CURRENT-BINARY-REVISED-FRONTEND-L2'; execution_fingerprint=[string]$l2.execution_envelope_fingerprint; tool_manifest_digest=[string]$l2.frontend_tool_manifest_digest; tool_count=[int]$l2.tool_count; aggregate_schema_digest=[string]$l2.aggregate_schema_digest; aggregate_schema_bytes=[int]$l2.aggregate_schema_bytes})
    Set-JsonProperty $manifest 'execution_envelope_fingerprint' $NativeEnvelope
    Set-JsonProperty $manifest 'native_launch_fingerprint' $NativeEnvelope
    Set-JsonProperty $manifest 'transport_policy_revision' $PolicyRevision
    Set-JsonProperty $manifest 'pagination_runtime_binding' ([ordered]@{revision='r03a-pagination-runtime-binding@1'; runtime_os='windows'; preflight='PASSED'; required_blob_count=4; inventory_digest=$Inventory})
    Set-JsonProperty $manifest 'provider_surface_changed' $false
    Set-JsonProperty $manifest 'frontend_sessions' 0
    Set-JsonProperty $manifest 'live_writer' 0
    Set-JsonProperty $manifest 'obligation_state' 'pending'
    Set-JsonProperty $manifest 'planner_relay' 0
    Set-JsonProperty $manifest 'synthesized_rows' 0
    Set-JsonProperty $manifest 'synthesized_blobs' 0
    Write-JsonValue $BaselinePath $manifest
    $manifestHash = Get-FileDigest $BaselinePath
    [IO.File]::WriteAllText("$BaselinePath.sha256", "$manifestHash`n", [Text.UTF8Encoding]::new($false))
    $env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $manifestHash

    $activationReports = @()
    foreach ($index in 1..2) {
        $env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT = Join-Path $Evidence "frontend-activation-preflight-v7-$index.json"
        Invoke-Runner '-frontend-activation-preflight'
        $activationReports += Get-Content -Raw -LiteralPath $env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT | ConvertFrom-Json
    }
    foreach ($report in $activationReports) {
        if ($report.status -ne 'FRONTEND_ACTIVATION_PREFLIGHT_PASSED' -or $report.anchor_probe -ne 'PASSED' -or $report.mutation -or $report.starting_state.frontend_session_count -ne 0 -or $report.starting_state.live_writer_count -ne 0 -or $report.starting_state.obligation_state -ne 'pending' -or $report.starting_state.planner_relay_count -ne 0) { throw 'V7 activation preflight did not preserve clean starting state' }
    }
    if (($activationReports[0].starting_state | ConvertTo-Json -Depth 30 -Compress) -ne ($activationReports[1].starting_state | ConvertTo-Json -Depth 30 -Compress)) { throw 'V7 repeated activation preflight changed observed state' }
    Write-JsonValue (Join-Path $Evidence 'starting-state-probe-v7-repeat.json') ([ordered]@{status='FRONTEND_STARTING_STATE_READY'; read_only=$true; mutation=$false; first=$activationReports[0].starting_state; second=$activationReports[1].starting_state})

    $finalResult = [ordered]@{
        qualification='R0.3A-FRONTEND-CLEAN-BASELINE-CONSUMPTION-ABI'
        baseline_generation_id=$BaselineId
        status='PASSED'
        clean_restore='PASSED'
        semantic_closure='PASSED'
        RuntimeDatabaseBinding='PASSED'
        WindowsDatabaseAccessPreflight='PASSED'
        RuntimeCASBinding='PASSED'
        WindowsCASAccessView='PASSED'
        PeerRecoveryAnchorsProbe='PASSED'
        FrontendStartingStateProbe='PASSED'
        FrontendActivationPreflight='PASSED'
        FrontendRuntimeActivationPreflight='PASSED'
        PaginationRuntimePreflight='PASSED'
        frontend_public_consumption_abi='PASSED'
        frontend_behavior_observability='PASSED'
        frontend_checker_public_coherence='PASSED'
        frontend_consumption_stack_digest=$stackDigest
        provider_surface_changed=$false
        frontend_L2_status='QUALIFIED_REUSABLE'
        frontend_sessions=0
        live_writer=0
        obligation='pending'
        planner_relay=0
        synthesized_rows=0
        synthesized_blobs=0
        preflight_mutation=0
        baseline_manifest='PASSED'
        baseline_manifest_sha256=$manifestHash
        cluster_system_identifier=[string]$binding.cluster_system_identifier
        database_name=[string]$binding.database_name
        database_oid=[string]$binding.database_oid
        runtime_database_binding_fingerprint=[string]$binding.fingerprint
        database_access_strategy=$DatabaseStrategy
        frontend_business='NOT_STARTED'
        allowance=0
        Worker=0
        Medium=0
        High=0
        provider_egress=0
        historical_evidence_modified=$false
    }
    Write-JsonValue (Join-Path $Evidence 'result.json') $finalResult
} finally {
    & bash -lc "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu'; '$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_ctl' -D '$PgDataWsl' -m fast -w stop >/dev/null 2>&1 || true; rmdir -- '$Socket' 2>/dev/null || true"
}

Get-Content -Raw -LiteralPath (Join-Path $Evidence 'result.json')
