# pattern: Imperative Shell
$smokeEnvironment = [ordered]@{
  POLIS_DESKTOP_SESSION_TOKEN = [Environment]::GetEnvironmentVariable('POLIS_DESKTOP_SESSION_TOKEN', 'Process')
  POLIS_WORKBENCH_FIXTURE_ADDR = [Environment]::GetEnvironmentVariable('POLIS_WORKBENCH_FIXTURE_ADDR', 'Process')
  POLIS_WORKBENCH_API_URL = [Environment]::GetEnvironmentVariable('POLIS_WORKBENCH_API_URL', 'Process')
  POLIS_WORKBENCH_FRONTEND_URL = [Environment]::GetEnvironmentVariable('POLIS_WORKBENCH_FRONTEND_URL', 'Process')
  VITE_WORKBENCH_MODE = [Environment]::GetEnvironmentVariable('VITE_WORKBENCH_MODE', 'Process')
  VITE_WORKBENCH_COMPANY_ID = [Environment]::GetEnvironmentVariable('VITE_WORKBENCH_COMPANY_ID', 'Process')
  VITE_WORKBENCH_LIVE_UPDATES = [Environment]::GetEnvironmentVariable('VITE_WORKBENCH_LIVE_UPDATES', 'Process')
  VITE_WORKBENCH_API_BASE_URL = [Environment]::GetEnvironmentVariable('VITE_WORKBENCH_API_BASE_URL', 'Process')
}

$portInUse = Get-NetTCPConnection -State Listen -LocalPort 4173 -ErrorAction SilentlyContinue
if ($null -ne $portInUse) { throw 'Refusing to use occupied Vite port 4173' }
$fixturePortInUse = Get-NetTCPConnection -State Listen -LocalPort 18084 -ErrorAction SilentlyContinue
if ($null -ne $fixturePortInUse) { throw 'Refusing to use occupied artifact fixture port 18084' }
$repoRoot = (Get-Location).Path
$viteMarker = Join-Path $repoRoot 'frontend\node_modules'
try {
  [Environment]::SetEnvironmentVariable('POLIS_DESKTOP_SESSION_TOKEN', 'slice26-browser-session', 'Process')
  [Environment]::SetEnvironmentVariable('POLIS_WORKBENCH_FIXTURE_ADDR', '127.0.0.1:18084', 'Process')
  [Environment]::SetEnvironmentVariable('POLIS_WORKBENCH_API_URL', 'http://127.0.0.1:18084/api/workbench', 'Process')
  [Environment]::SetEnvironmentVariable('POLIS_WORKBENCH_FRONTEND_URL', 'http://127.0.0.1:4173', 'Process')
  [Environment]::SetEnvironmentVariable('VITE_WORKBENCH_MODE', 'real', 'Process')
  [Environment]::SetEnvironmentVariable('VITE_WORKBENCH_COMPANY_ID', 'browser-company-01', 'Process')
  [Environment]::SetEnvironmentVariable('VITE_WORKBENCH_LIVE_UPDATES', 'deferred', 'Process')
  [Environment]::SetEnvironmentVariable('VITE_WORKBENCH_API_BASE_URL', 'http://127.0.0.1:18084/api/workbench', 'Process')
  rtk proxy python C:\Users\chyinan\.agents\skills\webapp-testing\scripts\with_server.py --server "rtk proxy bash -lc 'POLIS_DESKTOP_SESSION_TOKEN=slice26-browser-session POLIS_WORKBENCH_FIXTURE_ADDR=127.0.0.1:18084 bash scripts/go.sh run scripts/artifact_delivery_browser_fixture.go'" --port 18084 --server 'rtk proxy npm --prefix frontend run dev -- --host 127.0.0.1 --port 4173 --strictPort' --port 4173 -- rtk proxy python scripts/r1-artifact-delivery-browser-smoke.py
  if ($LASTEXITCODE -ne 0) { throw "artifact browser smoke failed with exit code $LASTEXITCODE" }
} finally {
  try {
    try { Invoke-WebRequest -Uri 'http://127.0.0.1:18084/__fixture/shutdown' -Method Post -Headers @{ 'X-Polis-Desktop-Token' = 'slice26-browser-session' } -TimeoutSec 2 | Out-Null } catch { }
    $viteProcesses = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.CommandLine -and $_.CommandLine.Contains($viteMarker) -and $_.CommandLine.Contains('--port 4173') }
    foreach ($process in $viteProcesses) { Stop-Process -Id $process.ProcessId -ErrorAction SilentlyContinue }
  } finally {
    foreach ($name in $smokeEnvironment.Keys) {
      [Environment]::SetEnvironmentVariable($name, $smokeEnvironment[$name], 'Process')
    }
  }
}
