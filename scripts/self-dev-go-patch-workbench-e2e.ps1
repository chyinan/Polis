$ErrorActionPreference = 'Stop'

$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$runID = 'selfdev-e2e-' + (Get-Date -Format 'yyyyMMddTHHmmss')
$evidence = Join-Path (Join-Path $repo 'evidence/development/polis-self-dev-v1-go-patch-delivery-20261009') $runID
$state = Join-Path $evidence 'fixture-state.json'
$complete = Join-Path $evidence 'browser-complete.json'
$runtime = Join-Path $evidence 'runtime'
$frontend = Join-Path $repo 'frontend'
$viteMode = 'selfdev-e2e-' + $runID
$viteEnv = Join-Path $frontend ".env.$viteMode.local"
$viteStdout = Join-Path $evidence 'vite.stdout.log'
$viteStderr = Join-Path $evidence 'vite.stderr.log'
$runtime = Join-Path (Join-Path $env:LOCALAPPDATA 'Temp/opencode') $runID
$goStdout = Join-Path $runtime 'go-e2e.stdout.log'
$goStderr = Join-Path $runtime 'go-e2e.stderr.log'
$browserLog = Join-Path $evidence 'browser.log'
$wslRoot = '/mnt/d/Programs/Polis-cloud-main'
$wslEvidence = "/mnt/d/Programs/Polis-cloud-main/evidence/development/polis-self-dev-v1-go-patch-delivery-20261009/$runID"
$wslScript = "cd $wslRoot && export GOPROXY=off POLIS_SELF_DEV_REAL_GO_PATCH_E2E=1 POLIS_SELF_DEV_E2E_RUN_ID=$runID POLIS_SELF_DEV_E2E_EVIDENCE=$wslEvidence POLIS_GO_TOOLCHAIN_ROOT=/run/polis-self-dev-go-toolchain SELFDEV_GO_TEST_PATTERN='^TestRealGoPatchWorkbenchE2E$' POLIS_SELF_DEV_EVIDENCE_SLICE=polis-self-dev-v1-go-patch-delivery-20261009-real-e2e-20261026 && bash scripts/self-dev-go-job-pg-test.sh"
$wslProcess = $null
$viteProcess = $null
$browserEnvWasSet = Test-Path Env:POLIS_SELF_DEV_E2E_STATE
$previousBrowserEnv = $env:POLIS_SELF_DEV_E2E_STATE

New-Item -ItemType Directory -Force -Path $runtime | Out-Null
if (Test-Path -LiteralPath $state) { throw "Refusing to overwrite existing E2E fixture state: $state" }
if (Test-Path -LiteralPath $complete) { throw "Refusing to overwrite existing E2E completion evidence: $complete" }
if (Test-Path -LiteralPath $viteEnv) { throw "Vite E2E env file already exists; preserving it: $viteEnv" }

try {
  $listener = [System.Net.Sockets.TcpClient]::new()
  try {
    $listener.Connect('127.0.0.1', 4173)
    throw 'Port 4173 is already in use; refusing to stop or reuse an unrelated process.'
  } catch [System.Net.Sockets.SocketException] {
    # No listener owns the fixed local Vite port.
  } finally {
    $listener.Dispose()
  }

  $wslArguments = "-d Ubuntu-22.04 -u chyinan --exec /bin/bash -lc `"$wslScript`""
  $wslProcess = Start-Process -FilePath (Get-Command wsl.exe).Source `
    -ArgumentList $wslArguments `
    -RedirectStandardOutput $goStdout -RedirectStandardError $goStderr -PassThru -WindowStyle Hidden

  $fixtureDeadline = [DateTime]::UtcNow.AddMinutes(10)
  while (-not (Test-Path -LiteralPath $state)) {
    $wslProcess.Refresh()
    if ($wslProcess.HasExited) { throw "Go fixture harness exited before publishing readiness (exit=$($wslProcess.ExitCode))." }
    if ([DateTime]::UtcNow -ge $fixtureDeadline) { throw 'Timed out waiting for the disposable PostgreSQL/API fixture.' }
    Start-Sleep -Milliseconds 500
  }

  $fixture = Get-Content -LiteralPath $state -Raw -Encoding UTF8 | ConvertFrom-Json
  if (-not $fixture.companyId -or -not $fixture.taskId -or -not $fixture.baseUrl -or -not $fixture.ownerPassword -or $fixture.viteMode -ne $viteMode) {
    throw 'Go fixture state is incomplete.'
  }

  $node = (Get-Command node.exe).Source
  $vite = Join-Path $frontend 'node_modules/vite/bin/vite.js'
  if (-not (Test-Path -LiteralPath $vite)) { throw "Vite entry is missing: $vite" }
  $viteProcess = Start-Process -FilePath $node `
    -ArgumentList @($vite, '--host', '127.0.0.1', '--strictPort', '--mode', $viteMode) `
    -WorkingDirectory $frontend -RedirectStandardOutput $viteStdout -RedirectStandardError $viteStderr -PassThru -WindowStyle Hidden

  $uiDeadline = [DateTime]::UtcNow.AddMinutes(2)
  $ready = $false
  while (-not $ready) {
    $viteProcess.Refresh()
    if ($viteProcess.HasExited) { throw "Vite exited before becoming ready (exit=$($viteProcess.ExitCode)); see $viteStdout and $viteStderr." }
    try {
      $response = Invoke-WebRequest -Uri 'http://127.0.0.1:4173/group/installation' -TimeoutSec 3
      $ready = $response.StatusCode -eq 200
    } catch {
      if ([DateTime]::UtcNow -ge $uiDeadline) { throw "Timed out waiting for Vite: $($_.Exception.Message)" }
      Start-Sleep -Milliseconds 400
    }
  }

  $env:POLIS_SELF_DEV_E2E_STATE = $state
  $python = (Get-Command python.exe).Source
  & $python (Join-Path $repo 'scripts/self-dev-go-patch-workbench-e2e.py') 2>&1 | Tee-Object -FilePath $browserLog
  $browserExit = $LASTEXITCODE
  if ($browserExit -ne 0) { throw "Workbench browser E2E failed with exit $browserExit; see $browserLog." }

  $testDeadline = [DateTime]::UtcNow.AddMinutes(5)
  while ($true) {
    $wslProcess.Refresh()
    if ($wslProcess.HasExited) { break }
    if ([DateTime]::UtcNow -ge $testDeadline) { throw 'Timed out waiting for Go E2E assertions and PostgreSQL cleanup.' }
    Start-Sleep -Milliseconds 500
  }
  if ($wslProcess.ExitCode -ne 0) { throw "Go E2E harness failed with exit $($wslProcess.ExitCode); see $goStdout and $goStderr." }
  if (-not (Test-Path -LiteralPath $complete)) { throw 'Browser did not write its completion evidence.' }
  $result = Get-Content -LiteralPath $complete -Raw -Encoding UTF8 | ConvertFrom-Json
  if ($result.result -ne 'passed') { throw "Browser completion evidence is not passed: $($result.error)" }
  Write-Output "REAL_GO_PATCH_WORKBENCH_E2E=PASS"
  Write-Output "company_id=$($result.companyId)"
  Write-Output "task_id=$($result.taskId)"
  Write-Output "job_run_id=$($result.jobRunId)"
  Write-Output "artifact_id=$($result.artifactId)"
  Write-Output "disposition=$($result.disposition)"
  Write-Output "provider_egress=0 real_model_worker_started=0 automatic_merge=NO automatic_deploy=NO"
} catch {
  if ((Test-Path -LiteralPath $state) -and -not (Test-Path -LiteralPath $complete)) {
    $failure = @{result='failed'; error=$_.Exception.Message; providerEgress=0; realModelWorkerStarted=0; automaticMerge='NO'; automaticDeploy='NO'}
    $failure | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $complete -Encoding UTF8
  }
  throw
} finally {
  if ($viteProcess) {
    $viteProcess.Refresh()
    if (-not $viteProcess.HasExited) {
      Stop-Process -Id $viteProcess.Id -Force -ErrorAction SilentlyContinue
      Wait-Process -Id $viteProcess.Id -Timeout 10 -ErrorAction SilentlyContinue
    }
  }
  if ($browserEnvWasSet) { $env:POLIS_SELF_DEV_E2E_STATE = $previousBrowserEnv }
  else { Remove-Item Env:POLIS_SELF_DEV_E2E_STATE -ErrorAction SilentlyContinue }
  if (Test-Path -LiteralPath $evidence) {
    foreach ($log in @($goStdout, $goStderr)) {
      if (Test-Path -LiteralPath $log) {
        Copy-Item -LiteralPath $log -Destination $evidence -Force
      }
    }
  }
}
