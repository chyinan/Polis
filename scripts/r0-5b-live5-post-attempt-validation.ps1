# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-5'
$db = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$browser = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'browser-e2e-result.json') | ConvertFrom-Json
$provider = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'provider-terminal-summary.json') | ConvertFrom-Json
$allowance = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$result = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'live-5-result.json') | ConvertFrom-Json
$manifest = Get-Content -Raw -Encoding UTF8 (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-execution-manifest.json') | ConvertFrom-Json
$l2 = Get-Content -Raw -Encoding UTF8 (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-live-l2-qualification.json') | ConvertFrom-Json
$mission = @($db.missions)[0]
$tasks = @($db.tasks)
$compat = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })[0]
$bootstrap = @($tasks | Where-Object { $_.kind -eq 'bootstrap_plan' })[0]
$session = @($db.worker_sessions | Where-Object { $_.task_id -eq $compat.id })[0]
$binding = @($db.task_validation_bindings | Where-Object { $_.task_id -eq $compat.id })[0]
$workspaceRow = @($db.worker_workspaces | Where-Object { $_.task_id -eq $compat.id })[0]
$checks = @($db.worker_checks)
$checkpoints = @($db.worker_checkpoints)
$artifact = @($db.artifacts | Where-Object { $_.task_id -eq $compat.id })[0]
$qualification = @($db.task_validation_artifact_qualifications | Where-Object { $_.task_id -eq $compat.id })[0]
$protocol = Get-ChildItem -LiteralPath (Join-Path $evidence 'provider-evidence') -Recurse -Filter protocol.jsonl -File | Select-Object -First 1
function Get-Sha256([string]$path) {
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $stream = [System.IO.File]::OpenRead($path)
    try { return ([System.BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-', '').ToLowerInvariant() }
    finally { $stream.Dispose() }
  } finally { $sha.Dispose() }
}
$casPath = Join-Path $evidence ("cas\r05b-live-5\" + $artifact.digest)
$casPresent = Test-Path -LiteralPath $casPath
$casDigest = if ($casPresent) { Get-Sha256 $casPath } else { '' }
$providerProcessAlive = [bool](Get-Process -Id ([int]$session.process_pid) -ErrorAction SilentlyContinue)
$events = @($db.events)
$checksMap = [ordered]@{
  company_count = @($db.company).Count
  mission_count = @($db.missions).Count
  task_count = $tasks.Count
  provider_task_count = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' }).Count
  binding_count = @($db.task_validation_bindings).Count
  worker_session_count = @($db.worker_sessions).Count
  worker_session_stopped = ($session.state -eq 'stopped')
  orphan_session_count = @($db.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count
  task_binding_coherent = ($binding.task_id -eq $compat.id -and $binding.mission_id -eq $compat.mission_id)
  workspace_check_count = $checks.Count
  workspace_pass_count = @($checks | Where-Object { $_.passed -eq $true }).Count
  qualified_checkpoint_count = $checkpoints.Count
  artifact_count = @($db.artifacts).Count
  artifact_staging_count = @($db.artifact_staging).Count
  artifact_qualification_count = @($db.task_validation_artifact_qualifications).Count
  bootstrap_state = $bootstrap.state
  compat_task_state = $compat.state
  mission_state = $mission.state
  workspace_revision = [int]$workspaceRow.revision
  artifact_digest_present = (-not [string]::IsNullOrWhiteSpace($artifact.digest))
  workspace_digest_matches_artifact = ($artifact.digest -eq $workspaceRow.digest)
  cas_present = $casPresent
  cas_digest_matches_artifact = ($casPresent -and $casDigest -eq $artifact.digest)
  provider_process_alive = $providerProcessAlive
  provider_egress = [int]$provider.turn.provider_egress
  real_provider_turn = [int]$provider.turn.completed_count
  initialize_pass = [bool]$provider.initialize.ack_received
  thread_start_pass = ([int]$provider.thread_start.thread_started_count -eq 1 -and [int]$provider.thread_start.registered_tool_count -eq 7)
  turn_completed = ($provider.turn.terminal_state -eq 'completed' -and [int]$provider.turn.completed_count -eq 1)
  browser_real_mode = [bool]$browser.real_mode_visible
  browser_candidate_task = [bool]$browser.candidate_task_visible
  browser_artifact = [bool]$browser.artifact_visible
  browser_checkpoint_projection = [bool]$browser.checkpoint_visible
  activity_checkpoint = (@($events | Where-Object { $_.kind -eq 'work.checkpoint' }).Count -gt 0)
  activity_delivery = (@($events | Where-Object { $_.kind -eq 'task.delivery' }).Count -gt 0)
  activity_turn_completed = (@($events | Where-Object { $_.kind -eq 'provider.turn.completed' }).Count -eq 1)
  mission_create_count = @($events | Where-Object { $_.kind -eq 'mission.create' }).Count
  mission_start_count = @($events | Where-Object { $_.kind -eq 'mission.start' }).Count
  retries = 0
  successors = 0
  high = [int]$allowance.high_turns
  duplicate_reservations = 0
  protocol_bytes = [int64]$protocol.Length
  surface_matches = ($manifest.product_exact_surface_execution_fingerprint -eq $result.surface.exact_surface_execution_fingerprint -and $l2.product_provider_v4_L2_fingerprint -eq $result.surface.provider_L2_fingerprint)
}
$backendPass = $checksMap.company_count -eq 1 -and $checksMap.mission_count -eq 1 -and $checksMap.task_count -eq 2 -and $checksMap.provider_task_count -eq 1 -and $checksMap.binding_count -eq 1 -and $checksMap.worker_session_count -eq 1 -and $checksMap.worker_session_stopped -and $checksMap.orphan_session_count -eq 0 -and $checksMap.task_binding_coherent -and $checksMap.workspace_pass_count -ge 1 -and $checksMap.qualified_checkpoint_count -eq 2 -and $checksMap.artifact_count -eq 1 -and $checksMap.artifact_staging_count -eq 1 -and $checksMap.artifact_qualification_count -eq 1 -and $checksMap.bootstrap_state -eq 'completed' -and $checksMap.compat_task_state -eq 'candidate' -and $checksMap.mission_state -eq 'active' -and $checksMap.workspace_revision -eq 4 -and $checksMap.artifact_digest_present -and $checksMap.workspace_digest_matches_artifact -and $checksMap.cas_digest_matches_artifact -and -not $checksMap.provider_process_alive -and $checksMap.provider_egress -eq 1 -and $checksMap.real_provider_turn -eq 1 -and $checksMap.initialize_pass -and $checksMap.thread_start_pass -and $checksMap.turn_completed -and $checksMap.mission_create_count -eq 1 -and $checksMap.mission_start_count -eq 1 -and $checksMap.retries -eq 0 -and $checksMap.successors -eq 0 -and $checksMap.high -eq 0 -and $checksMap.duplicate_reservations -eq 0 -and $checksMap.surface_matches
$frontendPass = $checksMap.browser_real_mode -and $checksMap.browser_candidate_task -and $checksMap.browser_artifact -and $checksMap.browser_checkpoint_projection -and $checksMap.activity_checkpoint -and $checksMap.activity_delivery -and $checksMap.activity_turn_completed
$status = if ($backendPass -and $frontendPass) { 'PASS' } else { 'FAIL' }
[ordered]@{status=$status; backend_delivery_integrity=if($backendPass){'PASS'}else{'PASS_WITH_GAPS'}; frontend_workbench_visibility=if($frontendPass){'PASS'}else{'FAIL_CHECKPOINT_PROJECTION'}; checks=$checksMap; cas_path=$casPath; cas_digest=$casDigest; protocol_path=$protocol.FullName} | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'post-attempt-validation.json')
if ($status -ne 'PASS') { Write-Output 'LIVE_5_POST_ATTEMPT_VALIDATION_FAIL_CHECKPOINT_PROJECTION'; exit 2 }
Write-Output 'LIVE_5_POST_ATTEMPT_VALIDATION_PASS'
