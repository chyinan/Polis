# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-4')).Path
$db = Get-Content -Raw (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$result = Get-Content -Raw (Join-Path $evidence 'live-4-result.json') | ConvertFrom-Json
$allowance = Get-Content -Raw (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$browser = Get-Content -Raw (Join-Path $evidence 'browser-e2e-result.json') | ConvertFrom-Json
$preflight = Get-Content -Raw (Join-Path $evidence 'server-preflight.json') | ConvertFrom-Json
$manifest = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-execution-manifest.json') | ConvertFrom-Json
$l2 = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-live-l2-qualification.json') | ConvertFrom-Json
$tasks = @($db.tasks)
$compat = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })[0]
$session = @($db.worker_sessions)[0]
$binding = @($db.task_validation_bindings)[0]
$workspaceRow = @($db.worker_workspaces)[0]
$protocol = Get-ChildItem -LiteralPath (Join-Path $evidence 'provider-evidence') -Recurse -Filter protocol.jsonl -File | Select-Object -First 1
$protocolBytes = if ($null -eq $protocol) { 0 } else { [int64]$protocol.Length }
$casPath = Join-Path $evidence ("cas\r05b-live-4\" + $workspaceRow.digest)
$casPresent = Test-Path -LiteralPath $casPath
function Get-Sha256([string]$path) {
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $stream = [System.IO.File]::OpenRead($path)
    try { return ([System.BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-', '').ToLowerInvariant() }
    finally { $stream.Dispose() }
  } finally { $sha.Dispose() }
}
$casDigest = if ($casPresent) { Get-Sha256 $casPath } else { '' }
$providerProcessAlive = [bool](Get-Process -Id ([int]$session.process_pid) -ErrorAction SilentlyContinue)
$events = @($db.events)
$credentials = @(Get-ChildItem -LiteralPath $evidence -Recurse -Force -File -ErrorAction SilentlyContinue | Where-Object { $_.Name -match '(^|[-_.])(auth|credential|secret|token)([-_.]|$)' })
$checks = [ordered]@{
  company_count = @($db.company).Count
  mission_count = @($db.missions).Count
  task_count = $tasks.Count
  provider_task_count = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' }).Count
  binding_count = @($db.task_validation_bindings).Count
  worker_session_count = @($db.worker_sessions).Count
  stopped_worker_session_count = @($db.worker_sessions | Where-Object { $_.state -eq 'stopped' }).Count
  orphan_session_count = @($db.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count
  worker_workspace_count = @($db.worker_workspaces).Count
  worker_check_count = @($db.worker_checks).Count
  checkpoint_count = @($db.worker_checkpoints).Count
  artifact_count = @($db.artifacts).Count
  artifact_staging_count = @($db.artifact_staging).Count
  artifact_qualification_count = @($db.task_validation_artifact_qualifications).Count
  mission_state = @($db.missions)[0].state
  bootstrap_state = @($tasks | Where-Object { $_.kind -eq 'bootstrap_plan' })[0].state
  compat_task_state = $compat.state
  binding_matches_task = ($binding.task_id -eq $compat.id -and $binding.mission_id -eq $compat.mission_id -and $binding.configuration_digest -eq $result.task_validation_binding.configuration_digest)
  workspace_digest_matches_cas = ($casPresent -and $casDigest -eq $workspaceRow.digest)
  workspace_revision = [int]$workspaceRow.revision
  allowance_medium_turns = [int]$allowance.medium_turns
  allowance_high_turns = [int]$allowance.high_turns
  allowance_tool_calls_used = [int]$allowance.tool_calls_used
  protocol_bytes = $protocolBytes
  provider_process_alive = $providerProcessAlive
  provider_egress = [int]$result.provider.provider_egress
  real_provider_turn = [int]$result.provider.real_provider_turn
  provider_initialization_event_count = @($events | Where-Object { $_.kind -eq 'provider.runtime.initialization_failed' }).Count
  provider_turn_inconclusive_event_count = @($events | Where-Object { $_.kind -eq 'provider.turn.inconclusive' }).Count
  browser_real_mode = [bool]$browser.real_mode_visible
  browser_mission_create_post_count = [int]$browser.mission_create_post_count
  browser_mission_start_post_count = [int]$browser.mission_start_post_count
  browser_provider_task_visible = [bool]$browser.provider_task_visible
  browser_worker_stopped_visible = [bool]$browser.provider_worker_stopped_visible
  browser_sse_requests = [int]$browser.sse_requests
  surface_matches = ($preflight.surface -eq $manifest.execution_identity.surface_id -and $preflight.execution_fingerprint -eq $manifest.product_exact_surface_execution_fingerprint -and $preflight.provider_l2 -eq $l2.product_provider_v4_L2_fingerprint -and $preflight.binary_sha256 -eq $manifest.execution_identity.provider_runtime.binary_sha256 -and $preflight.helper_sha256 -eq $manifest.execution_identity.provider_runtime.helper_sha256)
  no_credentials_recorded = ($credentials.Count -eq 0)
}
$pass = $checks.company_count -eq 1 -and $checks.mission_count -eq 1 -and $checks.task_count -eq 2 -and $checks.provider_task_count -eq 1 -and $checks.binding_count -eq 1 -and $checks.worker_session_count -eq 1 -and $checks.stopped_worker_session_count -eq 1 -and $checks.orphan_session_count -eq 0 -and $checks.worker_workspace_count -eq 1 -and $checks.worker_check_count -eq 0 -and $checks.checkpoint_count -eq 0 -and $checks.artifact_count -eq 0 -and $checks.artifact_staging_count -eq 0 -and $checks.artifact_qualification_count -eq 0 -and $checks.mission_state -eq 'active' -and $checks.bootstrap_state -eq 'completed' -and $checks.compat_task_state -eq 'working' -and $checks.binding_matches_task -and $checks.workspace_digest_matches_cas -and $checks.workspace_revision -eq 1 -and $checks.allowance_medium_turns -eq 1 -and $checks.allowance_high_turns -eq 0 -and $checks.allowance_tool_calls_used -eq 0 -and $checks.protocol_bytes -eq 0 -and -not $checks.provider_process_alive -and $checks.provider_egress -eq 0 -and $checks.real_provider_turn -eq 0 -and $checks.provider_initialization_event_count -eq 1 -and $checks.provider_turn_inconclusive_event_count -eq 0 -and $checks.browser_real_mode -and $checks.browser_mission_create_post_count -eq 1 -and $checks.browser_mission_start_post_count -eq 1 -and $checks.browser_provider_task_visible -and $checks.browser_worker_stopped_visible -and $checks.browser_sse_requests -eq 0 -and $checks.surface_matches -and $checks.no_credentials_recorded
[ordered]@{status=if($pass){'PASS'}else{'FAIL'}; checks=$checks; cas_path=$casPath; cas_digest=$casDigest; provider_protocol_path=if($null -eq $protocol){$null}else{$protocol.FullName}} | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath (Join-Path $evidence 'post-attempt-integrity.json')
if (-not $pass) { throw 'LIVE_4_POST_ATTEMPT_INTEGRITY_FAILED' }
Write-Output 'LIVE_4_POST_ATTEMPT_INTEGRITY_PASS'
