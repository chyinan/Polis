$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'r03a-frontend-config-bridge.ps1')

$Repo = 'D:\Programs\Polis'
$repoUnix = Convert-WindowsPathToWsl $Repo
$v9Root = '/home/chyinan/.local/state/polis-recovery/r03a-sample-6-restore-continuation-v9'
$pgData = "$v9Root/postgres/pgdata"
$socket = '/tmp/polis-pg-r03a-v9-frontend-single'
$expectedDB = 'polis_r0_3a_sample6_v6_restore_20260914164615_305'
$evidenceUnix = "$repoUnix/evidence/development/r0.3a-pagination-v3-real-frontend-single-session-v7"
$evidenceWindows = Join-Path $Repo 'evidence\development\r0.3a-pagination-v3-real-frontend-single-session-v7'
$runtimeUnix = "$v9Root/frontend-single-runtime"
$allowanceUnix = "$evidenceUnix/allowance.json"
$frontendBindingUnix = "$evidenceUnix/frontend-binding.json"
$frontendResultUnix = "$evidenceUnix/result.json"
$snapshotUnix = "$v9Root/unused-frontend-snapshot.dump"
$pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
$pgLib = "$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu"
$pgDataWindows = 'C:\Users\chyinan\AppData\Local\Packages\CanonicalGroupLimited.Ubuntu22.04LTS_79rhkp1fndgsc\LocalState\rootfs\home\chyinan\.local\state\polis-recovery\r03a-sample-6-restore-continuation-v9\postgres\pgdata'

if (Test-Path -LiteralPath $evidenceWindows) {
    $existing = @(Get-ChildItem -LiteralPath $evidenceWindows -Force)
    if ($existing.Count -gt 0) { throw 'single-session Frontend evidence path is not fresh' }
}
New-Item -ItemType Directory -Force -Path $evidenceWindows | Out-Null

$envMap = [ordered]@{
    POLIS_DSN = "host=127.0.0.1 port=55432 dbname=$expectedDB user=polis_runtime"
    POLIS_CODEX_BINARY = '/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/codex.exe'
    POLIS_CODEX_CODE_MODE_HOST = '/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/codex-code-mode-host.exe'
    POLIS_CODEX_AUTH_FILE = '/mnt/c/Users/chyinan/.codex/auth.json'
    POLIS_SELECTED_CODEX_CONFIG = "$repoUnix/.runtime/linux/r03a-t14c/home/config.toml"
    POLIS_V3_RUNTIME_ROOT = $v9Root
    POLIS_V3_EVIDENCE = $evidenceUnix
    POLIS_V3_ACCEPTANCE_QUALIFICATION = "$repoUnix/evidence/development/r0.3a-public-response-encoding-contract-hardening/qualification.json"
    POLIS_BLOB_DURABILITY_QUALIFICATION = "$repoUnix/evidence/development/r0.3a-blob-durability/qualification.json"
    POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION = "$repoUnix/evidence/development/r0.3a-frontend-checker-feedback-l2-v2/offline-result.json"
    POLIS_CURRENT_L1_EVIDENCE = "$repoUnix/evidence/development/r0.3a-current-binary-l1"
    POLIS_V3_BACKEND_EXECUTION_MANIFEST = "$repoUnix/evidence/development/r0.3a-current-binary-backend-l2-v6/execution-manifest.json"
    POLIS_V3_BACKEND_QUALIFICATION = "$repoUnix/evidence/development/r0.3a-current-binary-backend-l2-target-live-v1/result.json"
    POLIS_V3_BACKEND_L2_LIVE = "$repoUnix/evidence/development/r0.3a-current-binary-backend-l2-target-live-v1/result.json"
    POLIS_V3_FRONTEND_EXECUTION_MANIFEST = "$repoUnix/evidence/development/r0.3a-current-binary-frontend-l2/execution-manifest.json"
    POLIS_V3_FRONTEND_QUALIFICATION = "$repoUnix/evidence/development/r0.3a-current-binary-frontend-l2-live/result.json"
    POLIS_V3_FRONTEND_L2_LIVE = "$repoUnix/evidence/development/r0.3a-current-binary-frontend-l2-live/result.json"
    POLIS_V3_BACKEND_BINDING = "$repoUnix/evidence/development/r0.3a-pagination-v3-backend-sample-6/authorization-binding.json"
    POLIS_V3_FRONTEND_BINDING = $frontendBindingUnix
    POLIS_V3_ALLOWANCE = $allowanceUnix
    POLIS_V3_BACKEND_RESULT = "$repoUnix/evidence/development/r0.3a-sample-6-v9-continuity-adjudication-v3/backend-restored-runtime-verification.json"
    POLIS_V3_FRONTEND_RESULT = $frontendResultUnix
    POLIS_V3_POSTGRES_SNAPSHOT = $snapshotUnix
    POLIS_V3_POSTGRES_SNAPSHOT_DSN = "host=127.0.0.1 port=55432 dbname=$expectedDB user=chyinan"
    POLIS_V3_RESTORE_PROOF = "$repoUnix/evidence/development/r0.3a-sample-6-v9-continuity-adjudication-v3/result.json"
}

function Invoke-ExplicitWsl([string]$Command) {
    & bash -lc $Command
    if ($LASTEXITCODE -ne 0) { throw "WSL command failed with exit code $LASTEXITCODE" }
}

function New-ShellEnvPrefix($Values) {
    $parts = @()
    foreach ($entry in $Values.GetEnumerator()) {
        $value = ([string]$entry.Value).Replace("'", "'\\''")
        $parts += ("{0}='{1}'" -f $entry.Key, $value)
    }
    return ($parts -join ' ')
}

$pgStarted = $false
try {
    # Create the formal offline preflight/freshness records before PostgreSQL or allowance creation.
    $prefix = New-ShellEnvPrefix $envMap
    Invoke-ExplicitWsl "cd '$repoUnix' && env $prefix bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -frontend-single-preflight"
    Invoke-ExplicitWsl "cd '$repoUnix' && env $prefix bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -frontend-single-freshness"

    $wslConfigPath = Join-Path $evidenceWindows 'frontend-execution-config.wsl.json'
    $wslReportPath = Join-Path $evidenceWindows 'frontend-execution-config.wsl-report.json'
    $wslConfig = [ordered]@{
        path_encoding='wsl'; dsn=$envMap.POLIS_DSN; binary=$envMap.POLIS_CODEX_BINARY; code_mode_host=$envMap.POLIS_CODEX_CODE_MODE_HOST; auth_file=$envMap.POLIS_CODEX_AUTH_FILE; selected_config_path=$envMap.POLIS_SELECTED_CODEX_CONFIG; execution_config_path=(Convert-WindowsPathToWsl $wslConfigPath); execution_manifest_path=$envMap.POLIS_V3_FRONTEND_EXECUTION_MANIFEST; qualification_path=$envMap.POLIS_V3_FRONTEND_QUALIFICATION; blob_durability_qualification_path=$envMap.POLIS_BLOB_DURABILITY_QUALIFICATION; behavioral_contract_path=$envMap.POLIS_V3_ACCEPTANCE_QUALIFICATION; checker_feedback_qualification_path=$envMap.POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION; postgres_dump_path="$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/pg_dump"; postgres_snapshot_dsn=$envMap.POLIS_V3_POSTGRES_SNAPSHOT_DSN; runtime_root=$runtimeUnix; evidence_root=$evidenceUnix; recovery_package_root=$v9Root; source_cas_root="$v9Root/blobs"; runtime_artifact_manifest_path='/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/runtime-manifest.json'; authorization_binding_path=$frontendBindingUnix; current_l1_evidence_path=$envMap.POLIS_CURRENT_L1_EVIDENCE; execution_fingerprint='708790c760a905f8eeced8ba426b024341eec789bdb79ad1d89f87cb31982091'; current_l1_fingerprint='c3428c5ad1d1d0f9ae9ced8c79a43d62d33a2b396a374ce3da7367c5972ff2b6'; handover_boundary_evidence_path=$evidenceUnix; handover_boundary_snapshot_path=$snapshotUnix; problem_key='r03a-real-peer-collaboration-v1'; purpose='real_backend_peer_collaboration'; employee_id='emp-frontend'; model='gpt-5.6-luna'; effort='medium'; tool_call_limit=48; medium_limit=1; high_limit=0; concurrency=1; retry=$false; reset=$false
    }
    Write-FrontendJson $wslConfigPath $wslConfig
    $probe = Invoke-FrontendWslConfigProbe $Repo $wslConfigPath $wslReportPath
    if ($probe.status -ne 'FRONTEND_EXECUTION_CONFIG_READY') { throw 'FRONTEND_EXECUTION_CONFIG_INVALID' }

    & bash -lc "test -f '$pgData/PG_VERSION'"
    if ($LASTEXITCODE -ne 0) { throw 'v9 PGDATA missing' }
    Invoke-ExplicitWsl "cd '$repoUnix' && export LD_LIBRARY_PATH='$pgLib' && mkdir -m 700 -p '$socket' && bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory '$socket' -port 55432 -output '$evidenceUnix/socket-readiness.json' && '$pg/pg_ctl' -D '$pgData' -l '$v9Root/postgres/frontend-single.log' -o '-k $socket -h 127.0.0.1 -p 55432 -c synchronous_commit=on' -w start"
    $pgStarted = $true
    Invoke-ExplicitWsl "export LD_LIBRARY_PATH='$pgLib'; '$pg/psql' -X -h 127.0.0.1 -p 55432 -U chyinan -d postgres -Atc `"select datname from pg_database where datname like 'polis_r0_3a_sample6_v6_restore_%' order by datname desc limit 1`" | grep -Fx '$expectedDB' >/dev/null"

    Invoke-ExplicitWsl "cd '$repoUnix' && env $prefix bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -frontend-single"
    $result = Get-Content -Raw -LiteralPath (Join-Path $evidenceWindows 'result.json') | ConvertFrom-Json
    if ($result.status -ne 'PASSED' -or $result.frontend_real_execution -ne 'PASSED' -or $result.real_peer_collaboration_v3 -ne 'PASSED' -or [int]$result.medium_started -ne 1 -or [int]$result.provider_egress -ne 1) { throw 'single-session Frontend business result did not pass' }
} finally {
    if ($pgStarted) { try { & bash -lc "export LD_LIBRARY_PATH='$pgLib'; '$pg/pg_ctl' -D '$pgData' -m fast -w stop" } catch { Write-Warning $_.Exception.Message } }
    try { & bash -lc "rmdir -- '$socket' 2>/dev/null || true" } catch { }
}
