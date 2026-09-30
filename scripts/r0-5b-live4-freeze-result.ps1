# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'

$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-4')).Path
$manifest = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-execution-manifest.json') | ConvertFrom-Json
$l2 = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-live-l2-qualification.json') | ConvertFrom-Json
$preflight = Get-Content -Raw (Join-Path $evidence 'server-preflight.json') | ConvertFrom-Json
$allowance = Get-Content -Raw (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$final = Get-Content -Raw (Join-Path $evidence 'authoritative-final-state.json') | ConvertFrom-Json
$activity = Get-Content -Raw (Join-Path $evidence 'activity-snapshot.json') | ConvertFrom-Json
$browser = Get-Content -Raw (Join-Path $evidence 'browser-e2e-result.json') | ConvertFrom-Json
$db = Get-Content -Raw (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$mission = @($db.missions)[0]
$tasks = @($db.tasks)
$task = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })[0]
$bootstrap = @($tasks | Where-Object { $_.kind -eq 'bootstrap_plan' })[0]
$binding = @($db.task_validation_bindings | Where-Object { $_.task_id -eq $task.id })[0]
$session = @($db.worker_sessions | Where-Object { $_.task_id -eq $task.id })[0]
$workspaceRow = @($db.worker_workspaces | Where-Object { $_.task_id -eq $task.id })[0]
$initialization = @($db.worker_observations | Where-Object { $_.reason -eq 'provider_initialization' })[0]
$initializationEvent = @($db.events | Where-Object { $_.kind -eq 'provider.runtime.initialization_failed' })[0]
$protocol = Get-ChildItem -LiteralPath (Join-Path $evidence 'provider-evidence') -Recurse -Filter protocol.jsonl -File | Select-Object -First 1
$protocolBytes = if ($null -eq $protocol) { 0 } else { [int64]$protocol.Length }
$providerProcessAlive = [bool](Get-Process -Id ([int]$session.process_pid) -ErrorAction SilentlyContinue)
$startedAt = [DateTime]::Parse($allowance.started).ToUniversalTime()
$initializationAt = [DateTime]::Parse($initializationEvent.payload.occurred_at).ToUniversalTime()
$elapsedMs = [int][Math]::Round(($initializationAt - $startedAt).TotalMilliseconds)
$taskCount = $tasks.Count
$providerTaskCount = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' }).Count
$providerEmployeeCount = @($db.worker_sessions | Where-Object { $_.employee_id -eq 'emp-backend' }).Count
$providerSessionCount = @($db.worker_sessions).Count
$orphanSessionCount = @($db.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count
$providerProtocolPath = if ($null -eq $protocol) { $null } else { $protocol.FullName.Substring($evidence.Length + 1) }

$result = [ordered]@{
  task = 'R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_4'
  result = 'INCONCLUSIVE'
  classification = 'runtime/init failure before initialize request/ack, thread/start and real provider turn; no retry authorized'
  failure_boundary = 'INCONCLUSIVE_PREPROVIDER'
  business = [ordered]@{
    company = [ordered]@{id='r05b-live-4'; bootstrap='controlled Company/roster bootstrap only; Mission/Task creation remained browser-only'}
    mission = [ordered]@{id=$mission.id; title=$mission.title; state=$mission.state; created_through='Workbench Real mode'; started_through='Workbench Real mode'; acceptance_contract=$mission.acceptance_contract; cancel_post_count=0}
    task_roles = [ordered]@{
      total_task_rows=$taskCount
      tasks=@($tasks | ForEach-Object { [ordered]@{id=$_.id; kind=$_.kind; owner=$_.owner; state=$_.state; generation=$_.generation; mission_id=$_.mission_id} })
      provider_executable_task_count=$providerTaskCount
      provider_executable_employee_count=$providerEmployeeCount
      provider_bearing_worker_session_count=$providerSessionCount
      planning_provider_usage=0
    }
  }
  task_validation_binding = [ordered]@{present=($null -ne $binding); task_id=$binding.task_id; mission_id=$binding.mission_id; configuration_digest=$binding.configuration_digest; acceptance_revision=$binding.acceptance_revision; runner_kind=$binding.runner_kind; runner_revision=$binding.runner_revision; public_contract=$binding.contract}
  workspace = [ordered]@{task_id=$workspaceRow.task_id; digest=$workspaceRow.digest; revision=$workspaceRow.revision; initial_content_written=($workspaceRow.revision -eq 1); deliverable_produced=$false; workspace_check='NOT_RUN'; task_submit='NOT_RUN'}
  provider_binding = [ordered]@{
    authorization_count=1
    purpose=$preflight.purpose
    model='gpt-5.6-luna'
    effort='medium'
    profile=$session.profile
    provider_mode='real'
    surface_id=$manifest.execution_identity.surface_id
    tool_count=$manifest.execution_identity.tool_count
    manifest_digest=$manifest.execution_identity.manifest_digest
    aggregate_schema_bytes=$manifest.execution_identity.aggregate_schema_bytes
    aggregate_schema_digest=$manifest.execution_identity.aggregate_schema_digest
    exact_surface_execution_fingerprint=$manifest.product_exact_surface_execution_fingerprint
    product_provider_v4_L2_fingerprint=$l2.product_provider_v4_L2_fingerprint
    execution_envelope=$manifest.execution_identity.launch_runtime_envelope.fingerprint
    runtime_version=$manifest.execution_identity.provider_runtime.native_version
    binary_sha256=$manifest.execution_identity.provider_runtime.binary_sha256
    helper_sha256=$manifest.execution_identity.provider_runtime.helper_sha256
    task_id=$task.id
    task_kind=$task.kind
    task_owner=$task.owner
    employee_id=$session.employee_id
    employee_role='backend'
    session_id=$session.id
    epoch=$session.epoch
    incarnation=$session.incarnation
    task_validation_binding_digest=$binding.configuration_digest
    workspace_digest=$workspaceRow.digest
    workspace_revision=$workspaceRow.revision
    persistence='authorization was validated and reservation succeeded; terminal usage was not persisted because initialization failed before a turn'
  }
  initialization = [ordered]@{
    process_created=$initialization.data.process_created
    pid=$session.process_pid
    pid_present=$initialization.data.pid_present
    child_alive_before_initialize=$initialization.data.child_exit_observed -eq $false
    initialize_request_sent=$initialization.data.initialize_request_sent
    initialize_ack_received=$initialization.data.initialize_ack_received
    initialized_notification_sent=$initialization.data.initialized_notification_sent
    phase=$initialization.data.phase
    reason_code=$initialization.data.reason_code
    safe_message=$initialization.data.safe_message
    stdout_pipe_state=$initialization.data.stdout_pipe_state
    stderr_category=$initialization.data.stderr_category
    child_exit_observed=$initialization.data.child_exit_observed
  }
  provider = [ordered]@{
    reservation=1
    provider_egress=0
    turn_start=0
    real_provider_turn=0
    first_output_latency_ms=$null
    elapsed_ms=$elapsedMs
    input_tokens=$null
    output_tokens=$null
    tool_calls=[int]$session.tool_calls_used
    reconnects=0
    registered_tool_count=0
    thread_start='NOT_SENT/NOT_OBSERVED'
    terminal_phase='provider.runtime.initialization_failed'
    protocol_path=$providerProtocolPath
    protocol_bytes=$protocolBytes
    provider_process_alive_after_capture=$providerProcessAlive
    orphan_provider_process=[int]$providerProcessAlive
  }
  delivery = [ordered]@{workspace_check='NOT_RUN'; task_submit='NOT_RUN'; qualified_checkpoint_count=@($db.worker_checkpoints).Count; artifact_count=@($db.artifacts).Count; artifact_digest=$null; artifact_cas_resolution='NOT_REACHED_NO_ARTIFACT'; task_final_state=$task.state; mission_final_state=$mission.state}
  browser_observation=$browser
  activity=[ordered]@{snapshot_cursor=$activity.meta.snapshotCursor; event_count=@($activity.items).Count; provider_initialization_failed=(@($activity.items | Where-Object {$_.kind -eq 'provider_runtime_initialization_failed'}).Count -eq 1); provider_turn_inconclusive_event_present=$false; worker_terminal_visible=($session.state -eq 'stopped'); sse_requests=0}
  counters=[ordered]@{mission_business_instances=1; second_missions=0; provider_executable_tasks=$providerTaskCount; provider_executable_employees=$providerEmployeeCount; provider_bearing_sessions=$providerSessionCount; business_authorizations=1; provider_reservations=1; provider_egress=0; real_provider_turns=0; retries=0; successors=0; high=0; duplicate_reservations=0; orphan_sessions=$orphanSessionCount; orphan_provider_processes=[int]$providerProcessAlive; orphan_provider_turns=0}
  surface=[ordered]@{surface_id=$manifest.execution_identity.surface_id; manifest_digest=$manifest.execution_identity.manifest_digest; schema_bytes=$manifest.execution_identity.aggregate_schema_bytes; schema_digest=$manifest.execution_identity.aggregate_schema_digest; exact_surface_execution_fingerprint=$manifest.product_exact_surface_execution_fingerprint; provider_L2_fingerprint=$l2.product_provider_v4_L2_fingerprint; launch_envelope_fingerprint=$manifest.execution_identity.launch_runtime_envelope.fingerprint; b15_current_identity_match=$true}
  lifecycle=[ordered]@{worker_session_state=$session.state; compat_task_state=$task.state; mission_state=$mission.state; lifecycle_contract='B15 current lifecycle: initialization failure stops WorkerSession without synthesizing business failure, candidate or Artifact'}
  evidence=[ordered]@{allowance='allowance.json'; server_preflight='server-preflight.json'; authoritative_final_state='authoritative-final-state.json'; postgres_final_state='postgres-final-state.json'; activity_snapshot='activity-snapshot.json'; browser_e2e='browser-e2e-result.json'; provider_initialization='postgres-final-state.json#worker_observations[provider_initialization]'; provider_protocol=$providerProtocolPath; post_provider_query='post-provider-state-query.txt'; no_credentials_recorded=$true}
  post_attempt=[ordered]@{no_provider_traffic_after_terminal=$true; no_second_start=$true; no_cancel_command=$true; live3_historical_evidence_modified=$false; sse_started=$false; multi_agent_e2e_started=$false; freeze_reason='terminal before initialize/turn; stop after evidence capture'}
}
$result | ConvertTo-Json -Depth 60 | Set-Content -LiteralPath (Join-Path $evidence 'live-4-result.json')
Write-Output 'LIVE_4_RESULT_FROZEN_INCONCLUSIVE'
