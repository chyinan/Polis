param(
    [string]$BaselineId = 'r03a-frontend-clean-baseline-v5',
    [string]$DbName = 'polis_r0_3a_frontend_clean_baseline_v5',
    [int]$Port = 55432,
    [string]$Socket = '/tmp/polis-pg-r03a-frontend-baseline-v5',
    [switch]$ContinueExisting,
    [string]$FinalizerPath = (Join-Path $PSScriptRoot 'r03a-frontend-clean-baseline-v5-finalize.py'),
    [string]$BaselineQualification = 'V5',
    [string]$ManifestRevision = 'r03a-frontend-execution-baseline-manifest@5'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')
$Repo = 'D:\Programs\Polis'
$Evidence = Join-Path $Repo "evidence\development\$BaselineId"
$Runner = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$PolicyRevision = 'r03a-transport-policy@1'
$EnvelopeFingerprint = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51'
$RunNonce = [DateTime]::UtcNow.ToString('yyyyMMddHHmmss')
$DbSuffix = ($DbName -split '_')[-1]
$SocketSuffix = $BaselineId -replace '^.*-', ''

if ((Test-Path -LiteralPath $Evidence) -and -not $ContinueExisting) { throw "baseline evidence already exists: $Evidence" }
if (-not (Test-Path -LiteralPath $Runner -PathType Leaf)) { throw 'Windows runner executable is missing' }

$env:R03A_FRONTEND_BASELINE_ID = $BaselineId
$env:R03A_FRONTEND_BASELINE_DB_SUFFIX = $DbSuffix
$env:R03A_FRONTEND_BASELINE_SOCKET_SUFFIX = $SocketSuffix
$env:POLIS_V3_TRANSPORT_POLICY_REVISION = $PolicyRevision
$env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $EnvelopeFingerprint
$casManifest = Join-Path $Evidence 'cas-manifest.json'
$casManifestWsl = Convert-WindowsPathToWsl $casManifest
if (-not $ContinueExisting) {
    & bash -lc "R03A_FRONTEND_BASELINE_ID='$BaselineId' R03A_FRONTEND_BASELINE_DB_SUFFIX='$DbSuffix' R03A_FRONTEND_BASELINE_SOCKET_SUFFIX='$SocketSuffix' POLIS_V3_TRANSPORT_POLICY_REVISION='$PolicyRevision' POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT='$EnvelopeFingerprint' bash scripts/r03a-frontend-clean-baseline-materialize.sh"
    if ($LASTEXITCODE -ne 0) { throw 'immutable-package materialization failed' }
}
 $authoritativeCasManifest = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
if (Test-Path -LiteralPath $authoritativeCasManifest) {
    Copy-Item -LiteralPath $authoritativeCasManifest -Destination $casManifest -Force
} else {
    & bash -lc "cp -- /home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/work/anchor-observation/cas-manifest.json '$casManifestWsl'"
    if ($LASTEXITCODE -ne 0) { throw 'authoritative CAS manifest export failed' }
}

& (Join-Path $PSScriptRoot 'r03a-frontend-clean-baseline-v3-qualify.ps1') -BaselineId $BaselineId -DbName $DbName -Port $Port -Socket $Socket -TransportPolicyRevision $PolicyRevision -ExecutionEnvelopeFingerprint $EnvelopeFingerprint -CASManifestPath $casManifest -RuntimeEvidenceName "runtime-evidence-v5-$RunNonce"
if ($LASTEXITCODE -ne 0) { throw 'Windows native baseline qualification failed' }

$runtimeRoot = Join-Path $Evidence 'windows-runtime'
$binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$helper = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
$auth = 'C:\Users\chyinan\.codex\auth.json'
$baseline = Join-Path $Evidence 'FrontendExecutionBaselineManifest.json'
$baselineHash = (Get-Content -Raw -LiteralPath "$baseline.sha256").Trim()
$casRoot = Convert-WslPathToWindows "/home/chyinan/.local/state/polis-recovery/$BaselineId/blobs"
$company = 'r03a-pagination-v3-company-1789379324974307400'
$inventory = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'

function Invoke-LocalRuntimePreflight([string]$EvidenceName, [string]$OutputName) {
    $localEvidence = Join-Path $Evidence $EvidenceName
    New-Item -ItemType Directory -Force -Path $localEvidence | Out-Null
    $env:POLIS_CODEX_BINARY = $binary
    $env:POLIS_CODEX_CODE_MODE_HOST = $helper
    $env:POLIS_CODEX_AUTH_FILE = $auth
    $env:POLIS_V3_RUNTIME_ROOT = $runtimeRoot
    $env:POLIS_V3_EVIDENCE = $localEvidence
    $env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT = Join-Path $Evidence $OutputName
    $env:POLIS_V3_FRONTEND_BASELINE_MANIFEST = $baseline
    $env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $baselineHash
    $env:POLIS_V3_FRONTEND_CAS_MANIFEST = $casManifest
    $env:POLIS_V3_FRONTEND_CAS_ROOT = $casRoot
    $env:POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'
    $env:POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = $company
    $env:POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = $inventory
    $env:POLIS_V3_TRANSPORT_POLICY_REVISION = $PolicyRevision
    $env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = $EnvelopeFingerprint
    $env:POLIS_DSN = "host=127.0.0.1 port=$Port dbname=$DbName user=polis_runtime"
    & $Runner -frontend-runtime-preflight
    if ($LASTEXITCODE -ne 0) { throw "local runtime preflight failed: $EvidenceName" }
    $report = Get-Content -Raw -LiteralPath (Join-Path $Evidence $OutputName) | ConvertFrom-Json
    if ($report.status -ne 'FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED' -or $report.execution_envelope_fingerprint -ne $EnvelopeFingerprint -or $report.transport_policy.transport_policy_revision -ne $PolicyRevision -or $report.turn_started -or $report.provider_egress -or $report.allowance_created -or $report.business_mutation) { throw "runtime preflight contract failed: $EvidenceName" }
}

Invoke-LocalRuntimePreflight "runtime-evidence-v5-repeat-$RunNonce" 'frontend-runtime-preflight-2.json'

function Invoke-PaginationPreflight([string]$EvidenceName, [string]$OutputName, [string]$ResultName) {
    Invoke-LocalRuntimePreflight $EvidenceName $OutputName
    $report = Get-Content -Raw -LiteralPath (Join-Path $Evidence $OutputName) | ConvertFrom-Json
    $result = [ordered]@{ qualification='R0.3A-FRONTEND-PAGINATION-RUNTIME-PREFLIGHT'; status='PASSED'; runtime_os='windows'; launch_mode=$report.launch_envelope.launch_mode; execution_envelope_fingerprint=$report.execution_envelope_fingerprint; pagination_runtime_preflight=$report.pagination_runtime_preflight; required_blob_count=4; provider_egress=0; allowance_created=0; business_mutation=0; clean_stop=$report.stop_confirmed; historical_evidence_modified=$false }
    $result | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $Evidence $ResultName) -Encoding utf8
}

Invoke-PaginationPreflight "pagination-runtime-evidence-v5-$RunNonce-1" 'pagination-runtime-preflight-1.json' 'pagination-runtime-result-1.json'
Invoke-PaginationPreflight "pagination-runtime-evidence-v5-$RunNonce-2" 'pagination-runtime-preflight-2.json' 'pagination-runtime-result-2.json'

& python3 $FinalizerPath $Evidence (Join-Path $Evidence 'pagination-runtime-result-1.json') (Join-Path $Evidence 'pagination-runtime-result-2.json') $BaselineQualification $ManifestRevision
if ($LASTEXITCODE -ne 0) { throw 'v5 baseline finalization failed' }

Get-Content -Raw -LiteralPath (Join-Path $Evidence 'result.json')
