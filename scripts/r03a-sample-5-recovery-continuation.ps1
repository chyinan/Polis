# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-backend-sample-5-recovery-continuation-v1',
    [string]$RecoveryRoot = 'D:\Polis-recovery\r0.3a-pagination-v3-backend-sample-5-recovery-continuation-v1',
    [string]$SourceDatabase = 'polis_r0_3a_pagination_v3_backend_20260914162229_40936',
    [string]$SourceBlobRoot = 'D:\Polis-recovery\r0.3a-pagination-v3-backend-sample-5-runtime\backend\blobs'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
$pgStarted = $false
$sourceDatabasePresent = $false
$restoreDatabase = ''
$restoreDatabaseCreated = $false
$recoveryPackageComplete = $false

function Write-Utf8Json([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    [IO.File]::WriteAllText($Path, (($Value | ConvertTo-Json -Depth 100) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}
function Hash-File([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}
function To-WslPath([string]$Path) {
    if ($Path -notmatch '^([A-Za-z]):\\(.*)$') { throw "expected absolute Windows path: $Path" }
    '/mnt/' + $Matches[1].ToLowerInvariant() + '/' + ($Matches[2] -replace '\\', '/')
}
function Invoke-Checked([string]$File, [string[]]$Arguments) {
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$File exited with code $LASTEXITCODE" }
}
function Invoke-BashChecked([string]$Command) {
    & bash -lc $Command
    if ($LASTEXITCODE -ne 0) { throw "bash command exited with code $LASTEXITCODE" }
}
function Invoke-PsqlFile([string]$Database, [string]$SqlPath, [string]$OutputPath) {
    $repoUnix = To-WslPath $Repo
    $sqlUnix = To-WslPath $SqlPath
    $outputUnix = To-WslPath $OutputPath
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $psql = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin/psql"
    Invoke-BashChecked "$ld '$psql' -h 127.0.0.1 -p 55432 -d '$Database' -v ON_ERROR_STOP=1 -Atf '$sqlUnix' > '$outputUnix'"
}
function Export-State([string]$Database, [string]$CompanyID, [string]$OutputPath) {
    $sqlPath = Join-Path $RecoveryRoot ('state-' + [guid]::NewGuid().ToString('N') + '.sql')
    $sql = Get-Content -LiteralPath (Join-Path $Repo 'scripts\r03a-backend-state-query.sql') -Raw
    $sql = $sql -replace '(?m)^\\set ON_ERROR_STOP on\s*', ''
    $sql = $sql.Replace('r03a-t2-company-%', $CompanyID.Replace("'", "''"))
    [IO.File]::WriteAllText($sqlPath, $sql, [Text.UTF8Encoding]::new($false))
    try { Invoke-PsqlFile $Database $sqlPath $OutputPath }
    finally { if (Test-Path -LiteralPath $sqlPath) { Remove-Item -LiteralPath $sqlPath -Force } }
}
function Invoke-PgDump([string]$Database, [string]$DumpPath) {
    $repoUnix = To-WslPath $Repo
    $dumpUnix = To-WslPath $DumpPath
    $pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    Invoke-BashChecked "$ld '$pg/pg_dump' -h 127.0.0.1 -p 55432 -d '$Database' --format=custom --no-owner --no-acl --file='$dumpUnix'"
}
function Invoke-PgRestore([string]$Database, [string]$DumpPath) {
    $repoUnix = To-WslPath $Repo
    $dumpUnix = To-WslPath $DumpPath
    $pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    Invoke-BashChecked "$ld '$pg/pg_restore' -h 127.0.0.1 -p 55432 -d '$Database' --no-owner --no-acl --exit-on-error '$dumpUnix'"
}

try {
    Set-Location -LiteralPath $Repo
    if (Test-Path -LiteralPath $Evidence) {
        if (@(Get-ChildItem -LiteralPath $Evidence -Force).Count -ne 0) { throw 'recovery continuation evidence path is not fresh' }
    }
    if (Test-Path -LiteralPath $RecoveryRoot) {
        if (@(Get-ChildItem -LiteralPath $RecoveryRoot -Force).Count -ne 0) { throw 'recovery continuation root is not fresh' }
    }
    if (-not (Test-Path -LiteralPath (Join-Path $SourceBlobRoot '.pagination-runtime'))) { throw 'preserved sample-5 blob root is unavailable' }
    New-Item -ItemType Directory -Force -Path $Evidence, $RecoveryRoot | Out-Null
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    $repoUnix = To-WslPath $Repo
    $pgBin = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $sourceCheck = (& bash -lc "$ld '$pgBin/psql' -h 127.0.0.1 -p 55432 -d '$SourceDatabase' -U polis_runtime -Atc 'SELECT 1'").Trim()
    if ($LASTEXITCODE -ne 0 -or $sourceCheck -ne '1') { throw 'preserved sample-5 source database is unavailable' }
    $sourceDatabasePresent = $true

    $dsn = "postgres://polis_runtime@127.0.0.1:55432/$SourceDatabase"
    $sourceRootUnix = To-WslPath $SourceBlobRoot
    $evidenceUnix = To-WslPath $Evidence
    $cutCmd = "env POLIS_TEST_DSN='$dsn' bash scripts/go.sh run ./cmd/polis-r03a-recovery-cut -dsn '$dsn' -root '$sourceRootUnix' -output '$evidenceUnix'"
    Invoke-BashChecked $cutCmd

    $snapshot = Get-Content -LiteralPath (Join-Path $Evidence 'authoritative-snapshot.json') -Raw | ConvertFrom-Json
    $anchors = $snapshot.recovery_anchors
    if ($anchors.final_contract_revision_id -ne '9667c267be7fabf4a36dafea6b535060' -or $anchors.message_id -ne '420e11bfe5690853af96f0a60709c707' -or $anchors.backend_checkpoint_id -ne '5d50fce75f4fbb3ff2ef283c337e025d' -or $anchors.backend_artifact_id -ne 'fdb47635d9454e3103c0123820d7f1d8' -or $anchors.backend_workspace_revision -ne 5 -or $anchors.obligation_state -ne 'pending') { throw 'preserved sample-5 anchors do not match frozen terminal evidence' }
    if ($snapshot.handover.message_id -ne $anchors.message_id -or $snapshot.handover.workspace_revision -ne 5) { throw 'Backend snapshot did not refresh authoritative handover facts' }

    $dumpPath = Join-Path $RecoveryRoot 'sample-5-authoritative-state.dump'
    Invoke-PgDump $SourceDatabase $dumpPath
    $dumpHash = Hash-File $dumpPath
    $casManifest = Get-Content -LiteralPath (Join-Path $Evidence 'cas-manifest.json') -Raw | ConvertFrom-Json
    $casRoot = Join-Path $RecoveryRoot 'cas'
    New-Item -ItemType Directory -Force -Path $casRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $SourceBlobRoot $anchors.company_id) -Destination (Join-Path $casRoot $anchors.company_id) -Recurse
    $casDigest = Hash-File (Join-Path $Evidence 'cas-manifest.json')
    $sourceState = Join-Path $RecoveryRoot 'source-state.json'
    Export-State $SourceDatabase $anchors.company_id $sourceState
    $manifest = [ordered]@{ schema_version='r0.3a-sample-5-recovery-continuation-v1'; source_database=$SourceDatabase; snapshot_path=$dumpPath; snapshot_sha256=$dumpHash; cas_manifest_path=(Join-Path $Evidence 'cas-manifest.json'); cas_manifest_sha256=$casDigest; source_state_path=$sourceState; source_company_id=$anchors.company_id; recovery_package_complete=$true; historical_evidence_modified=$false; generated_at=[DateTime]::UtcNow.ToString('o') }
    Write-Utf8Json (Join-Path $Evidence 'recovery-manifest.json') $manifest
    $recoveryPackageComplete = $true

    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'drop', $SourceDatabase)
    $sourceDatabasePresent = $false
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'stop')
    $pgStarted = $false
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'start')
    $pgStarted = $true
    $restoreDatabase = 'polis_r0_3a_pagination_v3_sample5_restore_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + $PID
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'create', $restoreDatabase)
    $restoreDatabaseCreated = $true
    Invoke-PgRestore $restoreDatabase $dumpPath
    Invoke-Checked 'bash' @('scripts/r03a-real-backend-pg.sh', 'grant', $restoreDatabase)
    $restoredState = Join-Path $RecoveryRoot 'restored-state.json'
    Export-State $restoreDatabase $anchors.company_id $restoredState
    if ((Get-Content -LiteralPath $sourceState -Raw).Trim() -ne (Get-Content -LiteralPath $restoredState -Raw).Trim()) { throw 'restored sample-5 state differs from source state' }

    $env:POLIS_DSN = "postgres://polis_runtime@127.0.0.1:55432/$restoreDatabase"
    $env:POLIS_V3_RUNTIME_ROOT = (Join-Path $RecoveryRoot 'restored-runtime')
    $env:POLIS_V3_EVIDENCE = $Evidence
    $env:POLIS_V3_BACKEND_EXECUTION_MANIFEST = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-backend-l2-v6\execution-manifest.json'
    $env:POLIS_V3_BACKEND_QUALIFICATION = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-backend-l2-target-live-v1\result.json'
    $env:POLIS_V3_BACKEND_L2_LIVE = $env:POLIS_V3_BACKEND_QUALIFICATION
    $env:POLIS_V3_ACCEPTANCE_QUALIFICATION = 'D:\Programs\Polis\evidence\development\r0.3a-public-response-encoding-contract-hardening\qualification.json'
    $env:POLIS_BLOB_DURABILITY_QUALIFICATION = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json'
    $env:POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION = 'D:\Programs\Polis\evidence\development\r0.3a-frontend-checker-feedback-l2-v2\offline-result.json'
    $env:POLIS_CODEX_BINARY = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
    $env:POLIS_CODEX_CODE_MODE_HOST = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
    $env:POLIS_CODEX_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
    $env:POLIS_SELECTED_CODEX_CONFIG = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml'
    $env:POLIS_CURRENT_L1_EVIDENCE = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1'
    $env:POLIS_BLOB_DURABILITY_QUALIFICATION = 'D:\Programs\Polis\evidence\development\r0.3a-blob-durability\qualification.json'
    $env:POLIS_V3_BACKEND_BINDING = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-backend-sample-5\authorization-binding.json'
    $env:POLIS_V3_ALLOWANCE = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-backend-sample-5\allowance.json'
    $env:POLIS_V3_BACKEND_RESULT = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-v3-backend-sample-5\backend-result.json'
    $env:POLIS_V3_POSTGRES_SNAPSHOT = $dumpPath
    & bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -backend-restore-verify
    if ($LASTEXITCODE -ne 0) { throw 'restored sample-5 continuity verification failed' }

    Write-Utf8Json (Join-Path $Evidence 'result.json') ([ordered]@{ qualification='R0.3A-SAMPLE-5-RECOVERY-CONTINUATION'; status='PASSED'; sample_5_original_recovery_attempt='INCONCLUSIVE'; sample_5_recovery_continuation='PASSED'; authoritative_snapshot='PASSED'; fresh_restore='PASSED'; authoritative_state_continuity='PASSED'; backend_real_execution='PASSED'; frontend_started=$false; frontend='NOT_STARTED'; obligation_state='pending'; synthesized_rows=0; source_database_destroyed_before_restore=$true; snapshot_path=$dumpPath; snapshot_sha256=$dumpHash; cas_manifest_sha256=$casDigest; restored_state_path=$restoredState; historical_evidence_modified=$false; provider_egress=0; medium=0; high=0; generated_at=[DateTime]::UtcNow.ToString('o') })
} catch {
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    Write-Utf8Json (Join-Path $Evidence 'result.json') ([ordered]@{ qualification='R0.3A-SAMPLE-5-RECOVERY-CONTINUATION'; status='INCONCLUSIVE'; sample_5_original_recovery_attempt='INCONCLUSIVE'; sample_5_recovery_continuation='INCONCLUSIVE'; source_database=$SourceDatabase; source_database_preserved=[bool]$sourceDatabasePresent; recovery_package_complete=[bool]$recoveryPackageComplete; error=$_.Exception.Message; provider_egress=0; medium=0; high=0; historical_evidence_modified=$false; generated_at=[DateTime]::UtcNow.ToString('o') })
    throw
} finally {
    if ($restoreDatabaseCreated -and $restoreDatabase -ne '') { try { & bash scripts/r03a-real-backend-pg.sh drop $restoreDatabase } catch {} }
    if ($sourceDatabasePresent -and -not $recoveryPackageComplete) { Write-Warning "preserving source DB: $SourceDatabase" }
    if ($pgStarted) { try { & bash scripts/r03a-real-backend-pg.sh stop } catch {} }
    Set-Location -LiteralPath $oldLocation
}
