# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-postgres-preservation-contract-hardening-v1',
    [string]$RecoveryRoot = 'D:\Polis-recovery\r0.3a-postgres-preservation-qualification-v1'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
$clusterStarted = $false
$databaseCreated = $false
$roleCreated = $false
$runRoot = Join-Path $RecoveryRoot 'fixture'
$pgData = Join-Path $runRoot 'pgdata'
$newPgData = Join-Path $runRoot 'new-pgdata'
$port = 55433
$database = 'polis_r0_3a_preservation_fixture'
$role = 'polis_runtime'

function To-WslPath([string]$Path) {
    if ($Path -notmatch '^([A-Za-z]):\\(.*)$') { throw "expected absolute Windows path: $Path" }
    '/mnt/' + $Matches[1].ToLowerInvariant() + '/' + ($Matches[2] -replace '\\', '/')
}
function Invoke-BashChecked([string]$Command) {
    & bash -lc $Command
    if ($LASTEXITCODE -ne 0) { throw "bash command failed: $Command" }
}
function Scalar([string]$Database, [string]$Sql) {
    $repoUnix = To-WslPath $Repo
    $pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    return (& bash -lc "$ld '$pg/psql' -h 127.0.0.1 -p $port -d '$Database' -Atc '$Sql'").Trim()
}
function Hash-Text([string]$Text) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash([Text.UTF8Encoding]::new($false).GetBytes($Text)))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}
function Hash-File([string]$Path) { (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }
function Write-Utf8Json([string]$Path, $Value) { [IO.File]::WriteAllText($Path, (($Value | ConvertTo-Json -Depth 100) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false)) }

try {
    Set-Location -LiteralPath $Repo
    if (Test-Path -LiteralPath $Evidence -PathType Container) {
        if (@(Get-ChildItem -LiteralPath $Evidence -Force).Count -ne 0) { throw 'preservation qualification evidence path is not fresh' }
    }
    if (Test-Path -LiteralPath $RecoveryRoot) { Remove-Item -LiteralPath $RecoveryRoot -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $Evidence, $runRoot, $pgData, $newPgData | Out-Null
    $repoUnix = To-WslPath $Repo
    $pg = "$repoUnix/.tools/pg/usr/lib/postgresql/18/bin"
    $ld = "export LD_LIBRARY_PATH='$repoUnix/.tools/pg/usr/lib/x86_64-linux-gnu';"
    $dataUnix = To-WslPath $pgData
    $newDataUnix = To-WslPath $newPgData

    Invoke-BashChecked "$ld '$pg/initdb' -D '$dataUnix' -L '$repoUnix/.tools/pg/usr/share/postgresql/18' --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust"
    Invoke-BashChecked "$ld '$pg/pg_ctl' -D '$dataUnix' -l '$dataUnix/server.log' -o '-h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off' -w start"
    $clusterStarted = $true
    Invoke-BashChecked "$ld '$pg/createdb' -h 127.0.0.1 -p $port '$database'"
    $databaseCreated = $true
    Invoke-BashChecked "$ld '$pg/psql' -h 127.0.0.1 -p $port -d postgres -v ON_ERROR_STOP=1 -c 'CREATE ROLE $role LOGIN'"
    $roleCreated = $true
    $owner = (& bash -lc 'id -un').Trim()
    $dsn = "host=127.0.0.1 port=$port dbname=$database user=$owner"
    $env:POLIS_DSN = $dsn
    Invoke-BashChecked "POLIS_DSN='$dsn' bash scripts/go.sh run ./cmd/polis migrate"
    Invoke-BashChecked "$ld '$pg/psql' -h 127.0.0.1 -p $port -d '$database' -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO $role; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO $role;'"

    $serverVersion = Scalar 'postgres' 'SHOW server_version'
    $dbOID = Scalar 'postgres' "SELECT oid::text FROM pg_database WHERE datname='$database'"
    $dbOwner = Scalar 'postgres' "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$database'"
    $systemIdentifier = ((& bash -lc "$ld '$pg/pg_controldata' '$dataUnix'" | Select-String 'Database system identifier').ToString() -split ':', 2)[1].Trim()
    $pgVersionPath = Join-Path $pgData 'PG_VERSION'
    $pgControlPath = Join-Path $pgData 'global\pg_control'
    $pgDataDigest = Hash-Text ((Get-Content -LiteralPath $pgVersionPath -Raw).Trim() + '|' + (Hash-File $pgControlPath))
    $roleGrantSpecDigest = Hash-Text "role-provisioning@1|role=$role|public:USAGE|ALL TABLES:SELECT,INSERT,UPDATE"
    $migrationDigest = Hash-Text ((Get-ChildItem -LiteralPath (Join-Path $Repo 'db\migrations') -File | Sort-Object Name | ForEach-Object { $_.Name + ':' + (Hash-File $_.FullName) }) -join '|')
    $manifest = [ordered]@{
        manifest_revision = 'r0.3a-preserved-postgres-access@1'; postgres_version = $serverVersion; pgdata_canonical_path = $pgData; pgdata_digest = $pgDataDigest; cluster_system_identifier = $systemIdentifier; database_name = $database; database_oid = $dbOID; database_owner = $dbOwner; runtime_role_name = $role; role_provisioning_revision = 'role-provisioning@1'; role_grant_spec_digest = $roleGrantSpecDigest; migration_schema_digest = $migrationDigest; company_id = 'preservation-company'; mission_id = 'preservation-mission'; backend_task_id = 'preservation-backend-task'; frontend_task_id = 'preservation-frontend-task'; created_at = [DateTime]::UtcNow.ToString('o'); preservation_reason = 'deterministic physical cluster preservation qualification'; credentials_included = $false
    }
    Write-Utf8Json (Join-Path $Evidence 'PreservedPostgresAccessManifest.json') $manifest
    [IO.File]::WriteAllText((Join-Path $Evidence 'PreservedPostgresAccessManifest.sha256'), (Hash-File (Join-Path $Evidence 'PreservedPostgresAccessManifest.json')) + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))

    Invoke-BashChecked "$ld '$pg/pg_ctl' -D '$dataUnix' -m fast -w stop"
    $clusterStarted = $false
    Invoke-BashChecked "$ld '$pg/pg_ctl' -D '$dataUnix' -l '$dataUnix/server.log' -o '-h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off' -w start"
    $clusterStarted = $true
    $reopenSystemIdentifier = ((& bash -lc "$ld '$pg/pg_controldata' '$dataUnix'" | Select-String 'Database system identifier').ToString() -split ':', 2)[1].Trim()
    $reopenOID = Scalar 'postgres' "SELECT oid::text FROM pg_database WHERE datname='$database'"
    $reopenRole = Scalar 'postgres' "SELECT count(*)::text FROM pg_roles WHERE rolname='$role'"
    $exactReopen = $reopenSystemIdentifier -eq $systemIdentifier -and $reopenOID -eq $dbOID -and $reopenRole -eq '1'

    Invoke-BashChecked "$ld '$pg/psql' -h 127.0.0.1 -p $port -d postgres -v ON_ERROR_STOP=1 -c 'DROP ROLE $role'"
    $roleMissing = (Scalar 'postgres' "SELECT count(*)::text FROM pg_roles WHERE rolname='$role'") -eq '0'
    Invoke-BashChecked "$ld '$pg/psql' -h 127.0.0.1 -p $port -d postgres -v ON_ERROR_STOP=1 -c 'CREATE ROLE $role LOGIN'"
    Invoke-BashChecked "$ld '$pg/psql' -h 127.0.0.1 -p $port -d '$database' -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO $role; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO $role;'"
    $roleReprovisioned = (Scalar 'postgres' "SELECT count(*)::text FROM pg_roles WHERE rolname='$role'") -eq '1'
    $newRoleGrants = Scalar $database "SELECT count(*)::text FROM information_schema.role_table_grants WHERE grantee='$role' AND privilege_type IN ('SELECT','INSERT','UPDATE')"

    Invoke-BashChecked "$ld '$pg/pg_ctl' -D '$dataUnix' -m fast -w stop"
    $clusterStarted = $false
    Invoke-BashChecked "$ld '$pg/initdb' -D '$newDataUnix' -L '$repoUnix/.tools/pg/usr/share/postgresql/18' --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust"
    $newSystemIdentifier = ((& bash -lc "$ld '$pg/pg_controldata' '$newDataUnix'" | Select-String 'Database system identifier').ToString() -split ':', 2)[1].Trim()

    $qualification = [ordered]@{ qualification='R0.3A-POSTGRES-PRESERVATION-CONTRACT'; status=if($exactReopen -and $roleMissing -and $roleReprovisioned -and [int]$newRoleGrants -ge 3 -and $newSystemIdentifier -ne $systemIdentifier){'PASSED'}else{'FAILED'}; postgres_preservation_contract_hardening=if($exactReopen -and $roleMissing -and $roleReprovisioned){'PASSED'}else{'FAILED'}; durable_cluster_identity_qualification=if($exactReopen -and $newSystemIdentifier -ne $systemIdentifier){'PASSED'}else{'FAILED'}; preserved_state_reopen_qualification=if($exactReopen){'PASSED'}else{'FAILED'}; eligible_for_new_backend_sample=$false; provider_visible_surface_changed=$false; identity=[ordered]@{server_version=$serverVersion; data_directory=$pgData; cluster_system_identifier=$systemIdentifier; database_name=$database; database_oid=$dbOID; database_owner=$dbOwner; runtime_role=$role; role_grant_spec_digest=$roleGrantSpecDigest; migration_schema_digest=$migrationDigest}; cases=[ordered]@{exact_reopen=$exactReopen; missing_role_detected=$roleMissing; access_plane_reprovisioned=$roleReprovisioned; exact_grants_after_reprovision=([int]$newRoleGrants -ge 3); new_initdb_identity_mismatch=($newSystemIdentifier -ne $systemIdentifier); no_plaintext_credentials=$true}; medium=0; high=0; provider_egress=0; historical_evidence_modified=$false; generated_at=[DateTime]::UtcNow.ToString('o') }
    Write-Utf8Json (Join-Path $Evidence 'qualification.json') $qualification
    if ($qualification.status -ne 'PASSED') { throw 'PostgreSQL preservation qualification failed' }
} catch {
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    Write-Utf8Json (Join-Path $Evidence 'result.json') ([ordered]@{qualification='R0.3A-POSTGRES-PRESERVATION-CONTRACT';status='FAILED';error=$_.Exception.Message;medium=0;high=0;provider_egress=0;historical_evidence_modified=$false})
    throw
} finally {
    if ($clusterStarted) { try { Invoke-BashChecked "$ld '$pg/pg_ctl' -D '$dataUnix' -m fast -w stop" } catch {} }
    if ($databaseCreated) { try { Invoke-BashChecked "$ld '$pg/dropdb' -h 127.0.0.1 -p $port '$database'" } catch {} }
    if ($roleCreated) { try { Invoke-BashChecked "$ld '$pg/psql' -h 127.0.0.1 -p $port -d postgres -c 'DROP ROLE IF EXISTS $role'" } catch {} }
    if (Test-Path -LiteralPath $RecoveryRoot) { Remove-Item -LiteralPath $RecoveryRoot -Recurse -Force }
    Set-Location -LiteralPath $oldLocation
}
