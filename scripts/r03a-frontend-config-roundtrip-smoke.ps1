$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$repo = 'D:\Programs\Polis'
$runtime = Join-Path $repo '.runtime\frontend-config-roundtrip'
$configPath = Join-Path $runtime 'frontend-execution-config.windows.json'
$wslConfigPath = Join-Path $runtime 'frontend-execution-config.wsl.json'
$reportPath = Join-Path $runtime 'frontend-execution-config.wsl-report.json'
$windowsConfig = [ordered]@{
    dsn = 'host=127.0.0.1 port=55432 dbname=restore user=polis_runtime'
    binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
    code_mode_host = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
    auth_file = 'C:\Users\chyinan\.codex\auth.json'
    selected_config_path = Join-Path $repo '.runtime\linux\r03a-t14c\home\config.toml'
    execution_manifest_path = Join-Path $repo 'evidence\development\r0.3a-current-binary-frontend-l2\execution-manifest.json'
    qualification_path = Join-Path $repo 'evidence\development\r0.3a-current-binary-frontend-l2\offline-result.json'
    blob_durability_qualification_path = Join-Path $repo 'evidence\development\r0.3a-blob-durability\qualification.json'
    behavioral_contract_path = Join-Path $repo 'summary\r0-3a-behavioral-contract-hardening.md'
    checker_feedback_qualification_path = Join-Path $repo 'evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json'
    postgres_dump_path = Join-Path $repo '.tools\pg\usr\lib\postgresql\18\bin\pg_dump'
    postgres_snapshot_dsn = 'host=127.0.0.1 port=55432 dbname=restore user=postgres'
    runtime_root = 'D:\Polis-recovery\r0.3a-frontend-config-roundtrip-runtime'
    evidence_root = Join-Path $repo 'evidence\development\frontend-config-roundtrip'
    recovery_package_root = Join-Path $repo 'evidence\development'
    source_cas_root = Join-Path $repo 'evidence\development'
    runtime_artifact_manifest_path = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'
    authorization_binding_path = Join-Path $repo 'evidence\development\r0.3a-sample-6-frontend-initial-v9-authorization-binding.json'
    current_l1_evidence_path = Join-Path $repo 'evidence\development\r0.3a-current-binary-l1'
    execution_fingerprint = '708790c760a905f8eeced8ba426b024341eec789bdb79ad1d89f87cb31982091'
    current_l1_fingerprint = 'c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'
    handover_boundary_evidence_path = Join-Path $repo 'evidence\development\frontend-config-roundtrip\handover'
    handover_boundary_snapshot_path = 'D:\Polis-recovery\r0.3a-frontend-config-roundtrip-runtime\handover.dump'
    problem_key = 'r03a-real-peer-collaboration-v1'
    purpose = 'real_frontend_handover_from_recovered_state'
    employee_id = 'emp-frontend'
    model = 'gpt-5.6-luna'
    effort = 'medium'
    tool_call_limit = 48
    medium_limit = 1
    high_limit = 0
    concurrency = 1
    retry = $false
    reset = $false
}

try {
    $repoUnix = Convert-WindowsPathToWsl $repo
    $oldErrorAction = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & bash -lc "cd '$repoUnix' && bash scripts/go.sh run ./cmd/polis-r03a-real-frontend -initial-only"
        $legacyExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorAction
    }
    if ($legacyExitCode -eq 0) { throw 'legacy Frontend launch without explicit config unexpectedly succeeded' }

    Write-FrontendConfigPair $windowsConfig $configPath $wslConfigPath
    $report = Invoke-FrontendWslConfigProbe $repo $wslConfigPath $reportPath
    $reportRaw = Get-Content -Raw -LiteralPath $reportPath
    if ($report.status -ne 'FRONTEND_EXECUTION_CONFIG_READY' -or $report.employee_id -ne 'emp-frontend' -or $report.medium_limit -ne 1 -or $report.path_encoding -ne 'wsl_absolute') { throw 'round-trip probe result was invalid' }
    if ($reportRaw.Contains('host=127.0.0.1') -or $reportRaw.Contains('polis_runtime')) { throw 'raw DSN leaked into WSL probe evidence' }
    Write-Output 'frontend-config-roundtrip=PASS'
} finally {
    if (Test-Path -LiteralPath $runtime) { Remove-Item -LiteralPath $runtime -Recurse -Force }
}
