$ErrorActionPreference = 'Stop'

$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$runtimeRoot = Join-Path $repo '.runtime'
$runID = [guid]::NewGuid().ToString('N')
$runRoot = Join-Path $runtimeRoot "r0-7-windows-runtime-smoke-$runID"
$evidence = Join-Path $repo "evidence\development\r0.7-windows-runtime-smoke-$runID"
$pgRoot = Join-Path $repo '.tools\pg-windows-runtime'
$pgBin = Join-Path $pgRoot 'bin'
$pgData = Join-Path $runRoot 'postgres'
$blobRoot = Join-Path $runRoot 'cas'
$logs = Join-Path $evidence 'logs'
$polis = Join-Path $repo 'desktop\src-tauri\binaries\polis-x86_64-pc-windows-msvc.exe'

New-Item -ItemType Directory -Force $runRoot, $evidence, $logs, $blobRoot | Out-Null

function Get-FreePort {
  $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
  $listener.Start()
  $port = $listener.LocalEndpoint.Port
  $listener.Stop()
  return $port
}

function Wait-Tcp([int]$Port) {
  $deadline = [DateTime]::UtcNow.AddSeconds(20)
  do {
    try {
      $client = [Net.Sockets.TcpClient]::new()
      $client.Connect('127.0.0.1', $Port)
      $client.Dispose()
      return
    } catch { Start-Sleep -Milliseconds 100 }
  } while ([DateTime]::UtcNow -lt $deadline)
  throw "TCP readiness timeout on $Port"
}

function Wait-Health([int]$Port, [string]$Token) {
  $deadline = [DateTime]::UtcNow.AddSeconds(20)
  do {
    try {
      $headers = @{'X-Polis-Desktop-Token' = $Token}
      $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/healthz" -Headers $headers -TimeoutSec 2
      if ($response.StatusCode -eq 200) { return }
    } catch { Start-Sleep -Milliseconds 100 }
  } while ([DateTime]::UtcNow -lt $deadline)
  throw "backend readiness timeout on $Port"
}

$pgPort = Get-FreePort
$backendPort = Get-FreePort
$adminPassword = [guid]::NewGuid().ToString('N')
$appPassword = [guid]::NewGuid().ToString('N')
$sessionToken = [guid]::NewGuid().ToString('N')
$database = 'polis_r0_smoke_' + $runID.Substring(0, 12)
$passwordFile = Join-Path $runRoot 'initdb.password'
$pgProcess = $null
$backendProcess = $null
$result = [ordered]@{
  record_type = 'r0.7-windows-runtime-smoke@1'
  status = 'started'
  provider_business_egress = 0
  external_notifications = 0
  postgres_port = $pgPort
  backend_port = $backendPort
  database = $database
}

try {
  if (-not (Test-Path (Join-Path $pgBin 'initdb.exe'))) { throw 'PostgreSQL 18 Windows runtime is missing' }
  if (-not (Test-Path $polis)) { throw 'Windows Polis sidecar is missing' }

  Set-Content -LiteralPath $passwordFile -Value $adminPassword -NoNewline
  & (Join-Path $pgBin 'initdb.exe') -D $pgData -U postgres --pwfile=$passwordFile --auth-host=scram-sha-256 --auth-local=scram-sha-256 --encoding=UTF8 --no-locale *> (Join-Path $logs 'initdb.log')
  if ($LASTEXITCODE -ne 0) { throw "initdb failed: $LASTEXITCODE" }
  Remove-Item -LiteralPath $passwordFile -Force

  $pgProcess = Start-Process -FilePath (Join-Path $pgBin 'postgres.exe') -ArgumentList @('-D', $pgData, '-h', '127.0.0.1', '-p', $pgPort) -RedirectStandardOutput (Join-Path $logs 'postgres.log') -RedirectStandardError (Join-Path $logs 'postgres-error.log') -PassThru
  Wait-Tcp $pgPort
  $env:PGPASSWORD = $adminPassword
  $createRoleSql = "CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '$appPassword';"
  & (Join-Path $pgBin 'psql.exe') -h 127.0.0.1 -p $pgPort -U postgres -d postgres -v ON_ERROR_STOP=1 -c $createRoleSql *> (Join-Path $logs 'create-role.log')
  if ($LASTEXITCODE -ne 0) { throw "role setup failed: $LASTEXITCODE" }
  & (Join-Path $pgBin 'psql.exe') -h 127.0.0.1 -p $pgPort -U postgres -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $database OWNER polis_runtime;" *> (Join-Path $logs 'create-database.log')
  if ($LASTEXITCODE -ne 0) { throw "database setup failed: $LASTEXITCODE" }
  Remove-Item Env:PGPASSWORD

  $dsn = "postgres://polis_runtime:$appPassword@127.0.0.1:$pgPort/${database}"
  Set-Content -LiteralPath (Join-Path $logs 'migrate-input.txt') -Value "database=$database port=$pgPort"
  $env:POLIS_DSN = $dsn
  $env:POLIS_BLOB_ROOT = $blobRoot
  $errorPreferenceBeforeMigration = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  & $polis migrate *> (Join-Path $logs 'migrate.log')
  $migrationExitCode = $LASTEXITCODE
  $ErrorActionPreference = $errorPreferenceBeforeMigration
  if ($migrationExitCode -ne 0) { throw "migration failed: $migrationExitCode" }
  $env:POLIS_WORKBENCH_ADDR = "127.0.0.1:$backendPort"
  $env:POLIS_WORKER_MODE = 'deterministic'
  $env:POLIS_DESKTOP_SESSION_TOKEN = $sessionToken
  $backendProcess = Start-Process -FilePath $polis -ArgumentList 'serve' -RedirectStandardOutput (Join-Path $logs 'backend.log') -RedirectStandardError (Join-Path $logs 'backend-error.log') -PassThru
  Wait-Health $backendPort $sessionToken

  try {
    Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$backendPort/healthz" -TimeoutSec 2 | Out-Null
    throw 'unauthenticated health request unexpectedly succeeded'
  } catch {
    if ($_.Exception.Response.StatusCode.value__ -ne 401) { throw }
  }
  $headers = @{'X-Polis-Desktop-Token' = $sessionToken}
  $health = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$backendPort/healthz" -Headers $headers -TimeoutSec 2
  $active = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$backendPort/api/desktop/active-work" -Headers $headers -TimeoutSec 2
  $shutdown = Invoke-WebRequest -UseBasicParsing -Method Post -Uri "http://127.0.0.1:$backendPort/api/desktop/shutdown" -Headers $headers -TimeoutSec 2
  $backendProcess.WaitForExit(5000) | Out-Null
  if (-not $backendProcess.HasExited) { throw 'backend graceful shutdown timeout' }
  $result.health_status = $health.StatusCode
  $result.active_work = ($active.Content | ConvertFrom-Json).active
  $result.shutdown_status = $shutdown.StatusCode
  $result.backend_exit_code = $backendProcess.ExitCode
  $result.status = 'passed'
} catch {
  $result.status = 'failed'
  $result.failure = $_.Exception.Message
  throw
} finally {
  Remove-Item Env:POLIS_DSN, Env:POLIS_BLOB_ROOT, Env:POLIS_WORKBENCH_ADDR, Env:POLIS_WORKER_MODE, Env:POLIS_DESKTOP_SESSION_TOKEN, Env:PGPASSWORD -ErrorAction SilentlyContinue
  if ($backendProcess -and -not $backendProcess.HasExited) { Stop-Process -Id $backendProcess.Id -Force -ErrorAction SilentlyContinue }
  if ($pgProcess -and -not $pgProcess.HasExited) { & (Join-Path $pgBin 'pg_ctl.exe') -D $pgData -m fast -w stop *> (Join-Path $logs 'postgres-stop.log'); if (-not $pgProcess.HasExited) { Stop-Process -Id $pgProcess.Id -Force -ErrorAction SilentlyContinue } }
  $result | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $evidence 'result.json')
  if (Test-Path $passwordFile) { Remove-Item -LiteralPath $passwordFile -Force -ErrorAction SilentlyContinue }
  if (Test-Path $runRoot) { Remove-Item -LiteralPath $runRoot -Recurse -Force -ErrorAction SilentlyContinue }
}

Write-Output (Join-Path $evidence 'result.json')
