$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$evidence = Join-Path $Repo 'evidence\development\r0.3a-frontend-runtime-initialize-forensics-v1\pagination-runtime-preflight-v4'
$runtime = Join-Path $Repo '.runtime\windows\r03a-frontend-business-v3'
$runnerExe = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$casRootWsl = '/home/chyinan/.local/state/polis-recovery/r03a-frontend-clean-baseline-v3/blobs'
$casRootWindows = Convert-WslPathToWindows $casRootWsl
$casManifest = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$baseline = Join-Path $Repo 'evidence\development\r03a-frontend-clean-baseline-v3\FrontendExecutionBaselineManifest.json'
$baselineHash = (Get-Content -Raw -LiteralPath "$baseline.sha256").Trim()
New-Item -ItemType Directory -Force -Path $evidence,$runtime | Out-Null
$env:POLIS_CODEX_BINARY = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$env:POLIS_CODEX_CODE_MODE_HOST = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
$env:POLIS_CODEX_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
$env:POLIS_V3_RUNTIME_ROOT = $runtime
$env:POLIS_V3_EVIDENCE = $evidence
$env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT = Join-Path $evidence 'frontend-runtime-preflight.json'
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST = $baseline
$env:POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256 = $baselineHash
$env:POLIS_V3_FRONTEND_CAS_MANIFEST = $casManifest
$env:POLIS_V3_FRONTEND_CAS_ROOT = $casRootWindows
$env:POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION = 'r03a-cas-layout@1'
$env:POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE = 'r03a-pagination-v3-company-1789379324974307400'
$env:POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST = 'e0a76541c569d0f4671daab81ae8b62c4bb9632621ab180f1bfe5e272427b5bf'
$env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = 'e3dc30e477216733eceb54b01b15a545627d650f4414ebf6beb46a16ce8c4475'
& $runnerExe -frontend-runtime-preflight
if ($LASTEXITCODE -ne 0) { throw 'pagination runtime preflight failed' }
$report = Get-Content -Raw -LiteralPath $env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT | ConvertFrom-Json
if ($report.status -ne 'FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED' -or $report.pagination_runtime_preflight -ne 'PASSED' -or $report.turn_started -or $report.provider_egress -or $report.business_mutation -or $report.allowance_created) { throw 'pagination runtime preflight report did not satisfy the no-provider contract' }
$result = [ordered]@{qualification='R0.3A-FRONTEND-PAGINATION-RUNTIME-PREFLIGHT'; status='PASSED'; runtime_os='windows'; launch_mode=$report.launch_envelope.launch_mode; execution_envelope_fingerprint=$report.execution_envelope_fingerprint; pagination_runtime_preflight=$report.pagination_runtime_preflight; required_blob_count=4; provider_egress=0; allowance_created=0; business_mutation=0; clean_stop=$report.stop_confirmed; historical_evidence_modified=$false}
[IO.File]::WriteAllText((Join-Path $evidence 'result.json'), (($result | ConvertTo-Json -Depth 20) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
