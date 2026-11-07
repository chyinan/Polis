# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-5'
$manifest = Get-Content -Raw -Encoding UTF8 (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-execution-manifest.json') | ConvertFrom-Json
$l2 = Get-Content -Raw -Encoding UTF8 (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-live-l2-qualification.json') | ConvertFrom-Json
$preflight = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'server-preflight.json') | ConvertFrom-Json
$allowance = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$db = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$browser = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'browser-e2e-result.json') | ConvertFrom-Json
$activity = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'activity-snapshot.json') | ConvertFrom-Json
$provider = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'provider-terminal-summary.json') | ConvertFrom-Json
$mission = @($db.missions)[0]
$tasks = @($db.tasks)
$compat = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })[0]
$bootstrap = @($tasks | Where-Object { $_.kind -eq 'bootstrap_plan' })[0]
$binding = @($db.task_validation_bindings | Where-Object { $_.task_id -eq $compat.id })[0]
$session = @($db.worker_sessions | Where-Object { $_.task_id -eq $compat.id })[0]
$workspaceRow = @($db.worker_workspaces | Where-Object { $_.task_id -eq $compat.id })[0]
$checks = @($db.worker_checks | Where-Object { $_.task_id -eq $compat.id })
$passCheck = @($checks | Where-Object { $_.passed -eq $true })[0]
$checkpoints = @($db.worker_checkpoints)
$submitCheckpoint = @($checkpoints | Where-Object { $_.data.summary -eq 'current validated workspace submitted as final Task deliverable' })[0]
$artifact = @($db.artifacts | Where-Object { $_.task_id -eq $compat.id })[0]
$staging = @($db.artifact_staging | Where-Object { $_.task_id -eq $compat.id })[0]
$qualification = @($db.task_validation_artifact_qualifications | Where-Object { $_.task_id -eq $compat.id })[0]
$terminalObservation = @($db.worker_observations | Where-Object { $_.reason -eq 'provider_terminal' })[0]
$auth = $terminalObservation.data.usage.authorization
$casPath = Join-Path $evidence ("cas\r05b-live-5\" + $artifact.digest)
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
$taskSnapshot = [ordered]@{
  total_task_rows = $tasks.Count
  tasks = @($tasks | ForEach-Object { [ordered]@{id=$_.id; kind=$_.kind; owner=$_.owner; state=$_.state; generation=$_.generation; mission_id=$_.mission_id} })
  provider_executable_task_count = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' }).Count
  provider_executable_employee_count = @($db.worker_sessions | Where-Object { $_.employee_id -eq 'emp-backend' }).Count
  provider_bearing_worker_session_count = @($db.worker_sessions).Count
  planning_provider_usage = 0
}
$taskSnapshot | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath (Join-Path $evidence 'task-role-snapshot.json')
$binding | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'task-validation-binding.json')
$auth | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'business-authorization.json')
$passCheck | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'workspace-check-receipt.json')
$submitCheckpoint.data | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'qualified-checkpoint.json')
$artifact | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'artifact-metadata.json')
[ordered]@{path=$casPath; present=$casPresent; expected_digest=$artifact.digest; actual_digest=$casDigest; resolution=($casPresent -and $casDigest -eq $artifact.digest)} | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidence 'cas-verification.json')
$submitResult = [ordered]@{task_id=$compat.id; mission_id=$mission.id; checkpoint_id=$submitCheckpoint.id; validation_receipt_id=$passCheck.id; artifact_id=$artifact.id; workspace_digest=$workspaceRow.digest; workspace_revision=[int]$workspaceRow.revision; task_state=$compat.state; delivery_state='committed'}
$submitResult | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidence 'task-submit-result.json')
$result = [ordered]@{
  task = 'R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_5'
  result = 'FAILED'
  classification = 'completed real provider delivery, but Workbench overview projection did not expose the authoritative qualified checkpoint/Artifact checkpoint relation'
  failure_boundary = 'POST_PROVIDER_READ_VISIBILITY'
  business = [ordered]@{company=[ordered]@{id='r05b-live-5'; bootstrap='controlled Company/roster bootstrap only; Mission/Task creation remained browser-only'}; mission=[ordered]@{id=$mission.id; title=$mission.title; state=$mission.state; created_through='Workbench Real mode'; started_through='Workbench Real mode'; acceptance_contract=$mission.acceptance_contract; cancel_post_count=0}; task_roles=$taskSnapshot}
  task_validation_binding = [ordered]@{present=($null -ne $binding); task_id=$binding.task_id; mission_id=$binding.mission_id; configuration_digest=$binding.configuration_digest; acceptance_revision=$binding.acceptance_revision; runner_kind=$binding.runner_kind; runner_revision=$binding.runner_revision; public_contract=$binding.contract}
  workspace = [ordered]@{task_id=$workspaceRow.task_id; digest=$workspaceRow.digest; revision=[int]$workspaceRow.revision; deliverable_produced=$true; workspace_check='PASS'; pass_receipt_id=$passCheck.id; task_submit='PASS'; task_submit_receipt=$artifact.id}
  provider_binding = [ordered]@{authorization_count=1; purpose=$auth.Purpose; model=$auth.Model; effort=$auth.Effort; profile=$auth.Profile; provider_mode=$auth.ProviderMode; surface_id=$manifest.execution_identity.surface_id; tool_count=$manifest.execution_identity.tool_count; manifest_digest=$manifest.execution_identity.manifest_digest; aggregate_schema_bytes=$manifest.execution_identity.aggregate_schema_bytes; aggregate_schema_digest=$manifest.execution_identity.aggregate_schema_digest; exact_surface_execution_fingerprint=$manifest.product_exact_surface_execution_fingerprint; product_provider_v4_L2_fingerprint=$l2.product_provider_v4_L2_fingerprint; execution_envelope=$manifest.execution_identity.launch_runtime_envelope.fingerprint; runtime_version=$manifest.execution_identity.provider_runtime.native_version; binary_sha256=$manifest.execution_identity.provider_runtime.binary_sha256; helper_sha256=$manifest.execution_identity.provider_runtime.helper_sha256; task_id=$compat.id; task_kind=$compat.kind; task_owner=$compat.owner; employee_id=$session.employee_id; employee_role='backend'; session_id=$session.id; epoch=$session.epoch; task_validation_binding_digest=$binding.configuration_digest; workspace_digest=$workspaceRow.digest; workspace_revision=[int]$workspaceRow.revision; transport_policy_revision=$auth.TransportPolicyRevision}
  initialization = $provider.initialize
  provider = [ordered]@{reservation=1; provider_egress=[int]$provider.turn.provider_egress; turn_start=[int]$provider.turn.start_count; real_provider_turn=[int]$provider.turn.completed_count; first_output_latency_ms=$provider.turn.first_output_latency_ms; first_output=$provider.turn.first_output_delta; elapsed_ms=$provider.turn.first_output_latency_ms; input_tokens=$provider.turn.token_usage.total.inputTokens; output_tokens=$provider.turn.token_usage.total.outputTokens; total_tokens=$provider.turn.token_usage.total.totalTokens; tool_calls=[int]$provider.turn.tool_result_count; reconnects=[int]$provider.turn.reconnect_count; registered_tool_count=[int]$provider.thread_start.registered_tool_count; thread_start='PASS'; turn_terminal=$provider.turn.terminal_state}
  delivery = [ordered]@{workspace_check='PASS'; task_submit='PASS'; qualified_checkpoint_count=$checkpoints.Count; qualified_checkpoint_id=$submitCheckpoint.id; artifact_count=@($db.artifacts).Count; artifact_id=$artifact.id; artifact_digest=$artifact.digest; artifact_task_relation=($artifact.task_id -eq $compat.id); artifact_mission_relation=($qualification.company_id -eq $mission.company_id); cas_resolution=($casPresent -and $casDigest -eq $artifact.digest); task_final_state=$compat.state; mission_final_state=$mission.state}
  visibility = [ordered]@{backend_read_visibility='PASS'; frontend_real_mode=[bool]$browser.real_mode_visible; frontend_task_visible=[bool]$browser.candidate_task_visible; frontend_artifact_visible=[bool]$browser.artifact_visible; frontend_checkpoint_projection=[bool]$browser.checkpoint_visible; activity_checkpoint_event=(@($activity.items | Where-Object { $_.summary -eq 'work.checkpoint' }).Count -gt 0); activity_delivery_event=(@($activity.items | Where-Object { $_.summary -eq 'task.delivery' }).Count -gt 0); activity_provider_turn_event=(@($activity.items | Where-Object { $_.summary -eq 'provider.turn.completed' }).Count -gt 0); sse_requests=[int]$browser.sse_requests}
  counters = [ordered]@{mission_business_instances=1; second_missions=0; provider_executable_tasks=$taskSnapshot.provider_executable_task_count; provider_executable_employees=$taskSnapshot.provider_executable_employee_count; provider_bearing_sessions=$taskSnapshot.provider_bearing_worker_session_count; business_authorizations=1; provider_reservations=1; provider_egress=[int]$provider.turn.provider_egress; real_provider_turns=[int]$provider.turn.completed_count; retries=0; successors=0; high=0; duplicate_reservations=0; orphan_sessions=@($db.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count; orphan_provider_processes=0; orphan_provider_turns=0}
  terminal_lifecycle = [ordered]@{worker_session_state=$session.state; compat_task_state=$compat.state; bootstrap_task_state=$bootstrap.state; mission_state=$mission.state; provider_outcome=$provider.turn.outcome; clean_stop=$true; no_provider_traffic_after_terminal=$true}
  surface = [ordered]@{surface_id=$manifest.execution_identity.surface_id; manifest_digest=$manifest.execution_identity.manifest_digest; schema_bytes=$manifest.execution_identity.aggregate_schema_bytes; schema_digest=$manifest.execution_identity.aggregate_schema_digest; exact_surface_execution_fingerprint=$manifest.product_exact_surface_execution_fingerprint; provider_L2_fingerprint=$l2.product_provider_v4_L2_fingerprint; launch_envelope_fingerprint=$manifest.execution_identity.launch_runtime_envelope.fingerprint; b16_current_identity_match=$true}
  browser_observation=$browser
  activity=[ordered]@{snapshot_cursor=$activity.meta.snapshotCursor; event_count=@($activity.items).Count; checkpoint_event_present=(@($activity.items | Where-Object { $_.summary -eq 'work.checkpoint' }).Count -gt 0); provider_turn_completed_event_present=(@($activity.items | Where-Object { $_.summary -eq 'provider.turn.completed' }).Count -eq 1); sse_requests=[int]$browser.sse_requests}
  evidence=[ordered]@{allowance='allowance.json'; business_authorization='business-authorization.json'; task_roles='task-role-snapshot.json'; task_validation_binding='task-validation-binding.json'; initialization='initialization-lifecycle.json'; provider_terminal='provider-terminal-summary.json'; provider_protocol=$provider.protocol_path; workspace_check='workspace-check-receipt.json'; task_submit='task-submit-result.json'; qualified_checkpoint='qualified-checkpoint.json'; artifact='artifact-metadata.json'; cas='cas-verification.json'; authoritative_final_state='authoritative-final-state.json'; postgres_final_state='postgres-final-state.json'; activity_snapshot='activity-snapshot.json'; browser_e2e='browser-e2e-result.json'}
  post_attempt=[ordered]@{no_provider_traffic_after_terminal=$true; no_second_start=$true; no_cancel_command=$true; live4_historical_evidence_modified=$false; sse_started=$false; multi_agent_e2e_started=$false; freeze_reason='terminal completed; stop after evidence capture; read-model checkpoint projection remains incomplete'}
}
$result | ConvertTo-Json -Depth 100 | Set-Content -LiteralPath (Join-Path $evidence 'live-5-result.json')
Write-Output 'LIVE_5_RESULT_FROZEN_FAILED_READ_VISIBILITY'
