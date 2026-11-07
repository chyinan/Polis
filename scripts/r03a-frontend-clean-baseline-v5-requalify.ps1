param(
    [string]$EvidenceId = 'r03a-frontend-clean-baseline-v5-requalification',
    [string]$BaselineId = 'r03a-frontend-clean-baseline-v5',
    [string]$DbName = 'polis_r0_3a_frontend_clean_baseline_v5',
    [int]$Port = 55432,
    [string]$Socket = '/tmp/polis-pg-r03a-frontend-requal-v5'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$evidence = Join-Path $Repo "evidence\development\$EvidenceId"
$baselineEvidence = Join-Path $Repo "evidence\development\$BaselineId"
$baseline = Join-Path $baselineEvidence 'FrontendExecutionBaselineManifest.json'
$baselineHash = (Get-Content -Raw -LiteralPath "$baseline.sha256").Trim()
$runner = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$runtime = Join-Path $Repo '.runtime\windows\r03a-frontend-business-v5'
$casRootWsl = "/home/chyinan/.local/state/polis-recovery/$BaselineId/blobs"
$casRootWindows = Convert-WslPathToWindows $casRootWsl
$casManifest = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$casInventory = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'
$company = 'r03a-pagination-v3-company-1789379324974307400'
$dsn = "host=127.0.0.1 port=$Port dbname=$DbName user=polis_runtime"
$policy = 'r03a-transport-policy@1'
$envelope = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51'
$evidenceWsl = Convert-WindowsPathToWsl $evidence
$configWsl = Join-Path $baselineEvidence 'frontend-execution-config.wsl.json'
$configReport = Join-Path $evidence 'execution-config.json'
$databaseBinding = Join-Path $Repo 'evidence\development\r0.3a-windows-wsl-database-access-view-hardening-v1\runtime-database-binding.json'
$databaseView = Join-Path $Repo 'evidence\development\r0.3a-windows-wsl-database-access-view-hardening-v1\windows-database-access-view.json'

if (Test-Path $evidence) { throw "requalification evidence already exists: $evidence" }
New-Item -ItemType Directory -Force -Path $evidence | Out-Null
if (-not (Test-Path $runner -PathType Leaf)) { throw 'runner missing' }

$repoConfig = Convert-WindowsPathToWsl (Join-Path $baselineEvidence 'frontend-execution-config.wsl.json')
& bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-config-probe -config '$repoConfig' -output '$evidenceWsl/execution-config.json'"
if ($LASTEXITCODE -ne 0) { throw 'execution config requalification failed' }

$env:POLIS_DSN = $dsn
$env:POLIS_CODEX_BINARY = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$env:POLIS_CODEX_CODE_MODE_HOST = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
$env:POLIS_CODEX_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
$env:POLIS_V3_RUNTIME_ROOT = $runtime
$env:POLIS_V3_EVIDENCE = Join-Path $evidence 'runtime-evidence'
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST = $baseline
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $baselineHash
$env:POLIS_V3_FRONTEND_CAS_MANIFEST = $casManifest
$env:POLIS_V3_FRONTEND_CAS_ROOT = $casRootWindows
$env:POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'
$env:POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = $company
$env:POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = $casInventory
$env:POLIS_V3_TRANSPORT_POLICY_REVISION = $policy
$env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $envelope
$env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT = Join-Path $evidence 'frontend-activation-preflight.json'
$env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT = Join-Path $evidence 'frontend-runtime-preflight.json'
$env:POLIS_RUNTIME_DATABASE_BINDING = $databaseBinding
$env:POLIS_WINDOWS_DATABASE_ACCESS_VIEW = $databaseView
$env:POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT = Join-Path $evidence 'windows-database-preflight.json'
New-Item -ItemType Directory -Force -Path $env:POLIS_V3_EVIDENCE | Out-Null

$pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
$pgLib = "$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu"
$started = $false
try {
    & bash -lc "export LD_LIBRARY_PATH='$pgLib'; mkdir -m 700 -p '$Socket'; '$pg/pg_ctl' -D '/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/pgdata' -l '/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/requalification.log' -o '-k $Socket -h 127.0.0.1 -p $Port -c synchronous_commit=on' -w start"
    if ($LASTEXITCODE -ne 0) { throw 'requalification PostgreSQL startup failed' }
    $started = $true
    & bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn '$dsn' -output '$evidenceWsl/starting-state-1.json'"
    & bash -lc "bash scripts/go.sh run ./cmd/polis-r03a-frontend-starting-state-probe -dsn '$dsn' -output '$evidenceWsl/starting-state-2.json'"
    & $runner -frontend-database-preflight
    if ($LASTEXITCODE -ne 0) { throw 'requalification Windows database preflight failed' }
    & $runner -frontend-activation-preflight
    if ($LASTEXITCODE -ne 0) { throw 'requalification activation preflight failed' }
    & $runner -frontend-runtime-preflight
    if ($LASTEXITCODE -ne 0) { throw 'requalification runtime preflight failed' }
} finally {
    if ($started) { & bash -lc "export LD_LIBRARY_PATH='$pgLib'; '$pg/pg_ctl' -D '/home/chyinan/.local/state/polis-recovery/$BaselineId/postgres/pgdata' -m fast -w stop" }
    & bash -lc "rmdir -- '$Socket' 2>/dev/null || true"
}

$activation = Get-Content $env:POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT -Raw | ConvertFrom-Json
$runtimeReport = Get-Content $env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT -Raw | ConvertFrom-Json
$databaseReport = Get-Content $env:POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT -Raw | ConvertFrom-Json
if ($databaseReport.status -ne 'WINDOWS_DATABASE_ACCESS_PREFLIGHT_PASSED' -or $activation.status -ne 'FRONTEND_ACTIVATION_PREFLIGHT_PASSED' -or $activation.anchor_probe -ne 'PASSED' -or $activation.mutation -or $runtimeReport.status -ne 'FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED' -or $runtimeReport.execution_envelope_fingerprint -ne $envelope -or $runtimeReport.turn_started -or $runtimeReport.provider_egress -or $runtimeReport.allowance_created -or $runtimeReport.business_mutation -or $runtimeReport.pagination_runtime_preflight -ne 'PASSED') { throw 'requalification report failed contract' }
$state1 = Get-Content (Join-Path $evidence 'starting-state-1.json') -Raw | ConvertFrom-Json
$state2 = Get-Content (Join-Path $evidence 'starting-state-2.json') -Raw | ConvertFrom-Json
$stateJson1 = $state1.state | ConvertTo-Json -Depth 50 -Compress
$stateJson2 = $state2.state | ConvertTo-Json -Depth 50 -Compress
if ($stateJson1 -ne $stateJson2) { throw 'starting state drifted during requalification' }
$out = [ordered]@{ status='PASSED'; baseline_manifest_hash=(Get-Content "$baseline.sha256" -Raw).Trim(); execution_config='PASSED'; RuntimeDatabaseBinding='PASSED'; WindowsDatabaseAccessPreflight='PASSED'; FrontendStartingStateProbe='PASSED'; FrontendActivationPreflight='PASSED'; FrontendRuntimeActivationPreflight='PASSED'; PaginationRuntimePreflight='PASSED'; mutation=0; allowance=0; Worker=0; Medium=0; provider_egress=0; historical_evidence_modified=$false }
$out | ConvertTo-Json -Depth 20 | Set-Content (Join-Path $evidence 'result.json') -Encoding utf8
