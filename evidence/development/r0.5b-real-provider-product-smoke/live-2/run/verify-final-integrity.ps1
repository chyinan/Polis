$ErrorActionPreference = 'Stop'
$evidence = $PSScriptRoot
$state = Get-Content -Raw -LiteralPath (Join-Path $evidence 'current-state.json') | ConvertFrom-Json
$counts = Get-Content -Raw -LiteralPath (Join-Path $evidence 'current-counts.json') | ConvertFrom-Json
$allowance = Get-Content -Raw -LiteralPath (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$result = Get-Content -Raw -LiteralPath (Join-Path $evidence 'live-2-result.json') | ConvertFrom-Json
$browser = Get-Content -Raw -LiteralPath (Join-Path $evidence 'browser-final.json') | ConvertFrom-Json
$server = Get-Content -Raw -LiteralPath (Join-Path $evidence 'server-preflight.json') | ConvertFrom-Json
$postgresReady = (Get-Content -Raw -LiteralPath (Join-Path $evidence 'postgres-ready.txt')).Trim()
$expectedCriteria = @(
  "Mission ID: $($state.mission.id)",
  "Task ID: $(($state.tasks | Where-Object { $_.kind -eq 'compat' }).id)",
  'Acknowledgement: This artifact was produced through the Polis real-provider product path.',
  'Task summary: This artifact records one small product integration smoke.'
)
$compat = @($state.tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })
$bootstrap = @($state.tasks | Where-Object { $_.kind -eq 'bootstrap_plan' -and $_.owner -eq 'emp-planning' })
$providerSessions = @($state.worker_sessions | Where-Object { $_.employee_id -eq 'emp-backend' -and $_.task_id -eq $compat[0].id })
$binding = @($state.task_validation_bindings | Where-Object { $_.task_id -eq $compat[0].id })
$passingChecks = @($state.checks | Where-Object { $_.phase -eq 'product' -and $_.passed })
$terminalRows = @($state.provider_terminal_observations)
$runtimePath = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421'
$providerExecutables = @((Join-Path $runtimePath 'codex.exe'),(Join-Path $runtimePath 'codex-code-mode-host.exe'))
$liveProviderProcesses = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.ExecutablePath -and $providerExecutables -contains $_.ExecutablePath })
$credentialSnapshot = Join-Path (Join-Path (Join-Path (Join-Path $evidence 'provider-root') $providerSessions[0].id) 'home') 'auth.json'
$listeners = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in @(8081,4174) })

if ($counts.company_count -ne 1 -or $counts.employee_count -ne 4 -or $counts.mission_count -ne 1 -or $counts.task_count -ne 2) { throw 'LIVE_2 Company/Mission/Task cardinalities failed' }
if ($counts.bootstrap_plan_count -ne 1 -or $counts.compat_count -ne 1 -or $counts.provider_executable_task_count -ne 1 -or $counts.task_validation_binding_count -ne 1) { throw 'LIVE_2 Task-role or TaskValidationBinding cardinality failed' }
if ($bootstrap.Count -ne 1 -or $bootstrap[0].state -ne 'completed' -or $compat.Count -ne 1 -or $compat[0].state -ne 'cancelled') { throw 'LIVE_2 final Task roles/states are inconsistent with the cleanup receipt' }
if ($providerSessions.Count -ne 1 -or $providerSessions[0].state -ne 'stopped' -or $providerSessions[0].tool_calls_used -ne 13) { throw 'LIVE_2 provider WorkerSession terminal evidence is inconsistent' }
if ($binding.Count -ne 1 -or $binding[0].mission_id -ne $state.mission.id -or (Compare-Object @($binding[0].contract.required_text) $expectedCriteria).Count -ne 0) { throw 'LIVE_2 immutable TaskValidationBinding differs from the public Mission contract' }
if ($passingChecks.Count -ne 1 -or $passingChecks[0].report.status -ne 'PASS' -or $passingChecks[0].report.configuration_digest -ne $binding[0].configuration_digest) { throw 'LIVE_2 product check receipt does not bind the immutable validator' }
if ($counts.qualified_checkpoints -ne 0 -or $counts.artifact_count -ne 0 -or $counts.artifact_qualification_count -ne 0) { throw 'LIVE_2 unexpectedly has a qualified checkpoint or Artifact' }
if ($terminalRows.Count -ne 1 -or $terminalRows[0].data.state -ne 'completed' -or $terminalRows[0].data.usage.provider_egress -ne 1) { throw 'LIVE_2 provider terminal/egress evidence is inconsistent' }
if ($allowance.medium_turns -ne 1 -or $allowance.high_turns -ne 0 -or $allowance.medium_limit -ne 1 -or $allowance.high_limit -ne 0) { throw 'LIVE_2 provider allowance exceeded its exact limit' }
if ($browser.missionCreatePostCount -ne 1 -or $browser.missionStartPostCount -ne 1 -or $browser.missionCancelPostCount -ne 1 -or $browser.sseRequests -ne 0 -or @($browser.pageErrors).Count -ne 0) { throw 'LIVE_2 browser E2E command or page-health evidence is inconsistent' }
if ($liveProviderProcesses.Count -ne 0 -or (Test-Path -LiteralPath $credentialSnapshot) -or @($state.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count -ne 0) { throw 'LIVE_2 provider process, credential snapshot, or live session remains' }
if ($listeners.Count -ne 0) { throw 'LIVE_2 backend/frontend service listener remains after cleanup' }
if ($state.mission.state -ne 'cancelled' -or $counts.cancel_event_count -ne 1 -or $result.result -ne 'FAILED') { throw 'LIVE_2 final cancellation or terminal business result is inconsistent' }
if ($postgresReady -notmatch '^180006\|on\|on\|7$' -or $server.allowance_file_created -ne $false) { throw 'LIVE_2 fresh PostgreSQL/runtime preflight evidence is inconsistent' }

$integrity = [pscustomobject]@{
  post_attempt_evidence_integrity = 'PASS'
  product_smoke_result = $result.result
  company_count = $counts.company_count
  mission_count = $counts.mission_count
  task_count = $counts.task_count
  provider_executable_task_count = $counts.provider_executable_task_count
  provider_bearing_worker_session_count = $counts.provider_bearing_session_count
  business_authorization_count = $result.provider.authorization_count
  reservation_count = $result.provider.reservation_count
  provider_egress = $result.provider.provider_egress
  provider_tool_calls = $result.provider.tool_calls
  qualified_checkpoint_count = $counts.qualified_checkpoints
  artifact_count = $counts.artifact_count
  retry_count = $result.retries
  successor_count = $result.successors
  high_turns = $allowance.high_turns
  duplicate_reservations = $result.duplicate_reservations
  live_provider_sessions = $result.live_provider_sessions
  live_provider_processes = $liveProviderProcesses.Count
  credential_snapshot_present = (Test-Path -LiteralPath $credentialSnapshot)
  backend_frontend_listeners = $listeners.Count
  final_mission_state = $state.mission.state
}
$json = ConvertTo-Json -InputObject $integrity -Depth 10
[IO.File]::WriteAllText((Join-Path $evidence 'post-attempt-integrity.json'), $json + "`n", [Text.UTF8Encoding]::new($false))
$integrity | ConvertTo-Json -Compress
