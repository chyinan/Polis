$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$evidence = Join-Path $Repo 'evidence\development\r0.3a-frontend-runtime-initialize-forensics-v1\windows-runner-integration-v4'
$runtime = Join-Path $evidence 'runtime'
$configWindows = Join-Path $evidence 'frontend-execution-config.windows.json'
$configWsl = Join-Path $evidence 'frontend-execution-config.wsl.json'
$configReport = Join-Path $evidence 'frontend-execution-config.wsl-report.json'
$runnerExe = Join-Path $Repo '.runtime\windows\r0.3a-pagination-v3\polis-r03a-pagination-v3.exe'
$baselinePath = Join-Path $Repo 'evidence\development\r03a-frontend-clean-baseline-v2-requalification-2\FrontendExecutionBaselineManifest.json'
$casRootWsl = '/home/chyinan/.local/state/polis-recovery/r03a-frontend-clean-baseline-v2-requalification-2/blobs'
$casRootWindows = Convert-WslPathToWindows $casRootWsl
$casManifest = Join-Path $Repo 'evidence\development\r0.3a-sample-6-final-recovery-continuation-v5\cas-manifest.json'
$baseline = Get-Content -Raw -LiteralPath $baselinePath | ConvertFrom-Json
$baselineHash = (Get-Content -Raw -LiteralPath ($baselinePath + '.sha256')).Trim()
$cas = Get-Content -Raw -LiteralPath (Join-Path $Repo 'evidence\development\r03a-frontend-clean-baseline-v2-requalification-2\cas-validation.json') | ConvertFrom-Json

if (-not (Test-Path -LiteralPath $runnerExe -PathType Leaf)) { throw 'Windows business runner executable is missing' }
New-Item -ItemType Directory -Force -Path $evidence,$runtime | Out-Null

$windowsConfig = [ordered]@{
    path_encoding='windows-native'
    dsn='host=127.0.0.1 port=55432 dbname=polis_r0_3a_frontend_clean_baseline_v2_requalification_2 user=polis_runtime'
    binary='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
    code_mode_host='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
    auth_file='C:\Users\chyinan\.codex\auth.json'
    selected_config_path=(Join-Path $Repo '.runtime\linux\r03a-t14c\home\config.toml')
    execution_config_path=$configWindows
    execution_manifest_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-frontend-l2\execution-manifest.json')
    qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-frontend-l2\offline-result.json')
    blob_durability_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-blob-durability\qualification.json')
    behavioral_contract_path=(Join-Path $Repo 'evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json')
    checker_feedback_qualification_path=(Join-Path $Repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json')
    postgres_dump_path=(Join-Path $Repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump')
    postgres_snapshot_dsn='host=127.0.0.1 port=55432 dbname=polis_r0_3a_frontend_clean_baseline_v2_requalification_2 user=chyinan'
    runtime_root=$runtime
    evidence_root=$evidence
    recovery_package_root=(Join-Path $Repo 'evidence\development\r03a-frontend-clean-baseline-v2-requalification-2')
    source_cas_root=$casRootWindows
    runtime_artifact_manifest_path='C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'
    authorization_binding_path=(Join-Path $Repo 'evidence\development\r0.3a-pagination-v3-real-frontend-clean-single-session-v4\frontend-binding.json')
    current_l1_evidence_path=(Join-Path $Repo 'evidence\development\r0.3a-current-binary-l1')
    execution_fingerprint='708790c760a905f8eeced8ba426b024341eec789bdb79ad1d89f87cb31982091'
    current_l1_fingerprint='c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'
    handover_boundary_evidence_path=$evidence
    handover_boundary_snapshot_path=(Join-Path $runtime 'unused.dump')
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
    runtime_cas_binding=[ordered]@{canonical_root=$casRootWindows; layout_revision='r03a-cas-layout@1'; company_namespace=$cas.company_namespaces[0]; required_blob_inventory_digest=$cas.inventory_digest; required_blob_count=4}
}
Write-FrontendJson $configWindows $windowsConfig
$wslConfig = Convert-FrontendConfigToWsl $windowsConfig
$wslConfig.execution_config_path = Convert-WindowsPathToWsl $configWsl
Write-FrontendJson $configWsl $wslConfig
$wslProbe = Invoke-FrontendWslConfigProbe $Repo $configWsl $configReport
if ($wslProbe.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'WSL config bridge did not pass' }

$env:POLIS_CODEX_BINARY = $windowsConfig.binary
$env:POLIS_CODEX_CODE_MODE_HOST = $windowsConfig.code_mode_host
$env:POLIS_CODEX_AUTH_FILE = $windowsConfig.auth_file
$env:POLIS_V3_RUNTIME_ROOT = $runtime
$env:POLIS_V3_EVIDENCE = $evidence
$env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT = Join-Path $evidence 'frontend-runtime-preflight.json'
$env:POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT = '05333c83d07fe649a7308325414daf4a4826263292546452a1a696986967a8fb'
& $runnerExe -frontend-runtime-preflight
if ($LASTEXITCODE -ne 0) { throw "Windows-native runner runtime preflight failed with exit code $LASTEXITCODE" }
$runtimeReport = Get-Content -Raw -LiteralPath $env:POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT | ConvertFrom-Json
if ($runtimeReport.status -ne 'FRONTEND_RUNTIME_ACTIVATION_PREFLIGHT_PASSED' -or $runtimeReport.turn_started -or $runtimeReport.provider_egress -or $runtimeReport.business_mutation -or -not $runtimeReport.stop_confirmed -or $runtimeReport.registered_tool_count -ne 12) { throw 'Windows-native runner integration report is invalid' }
$result = [ordered]@{ qualification='R0.3A-FRONTEND-WINDOWS-NATIVE-RUNNER-INTEGRATION'; status='PASSED'; config_bridge='PASSED'; launch_mode=$runtimeReport.launch_envelope.launch_mode; execution_envelope_fingerprint=$runtimeReport.execution_envelope_fingerprint; initialize='PASSED'; thread_registration='PASSED'; registered_tools=12; turn_started=0; provider_egress=0; business_mutation=0; allowance_created=0; clean_stop='PASSED'; provider_surface_changed=$false; historical_evidence_modified=$false }
Write-FrontendJson (Join-Path $evidence 'result.json') $result
