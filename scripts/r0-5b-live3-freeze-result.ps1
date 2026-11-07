# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'

$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-3')).Path
$manifest = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-execution-manifest.json') | ConvertFrom-Json
$l2 = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-live-l2-qualification.json') | ConvertFrom-Json
$b13 = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b13-provider-process-launch-reproducibility-hardening-v3\qualification.json') | ConvertFrom-Json
$preflight = Get-Content -Raw (Join-Path $evidence 'server-preflight.json') | ConvertFrom-Json
$allowance = Get-Content -Raw (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$final = Get-Content -Raw (Join-Path $evidence 'authoritative-final-state.json') | ConvertFrom-Json
$activity = Get-Content -Raw (Join-Path $evidence 'activity-snapshot.json') | ConvertFrom-Json
$db = Get-Content -Raw (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$browser = Get-Content -Raw (Join-Path $evidence 'browser-e2e-result.json') | ConvertFrom-Json
$session = @($db.worker_sessions)[0]
$task = @($db.tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })[0]
$binding = @($db.task_validation_bindings | Where-Object { $_.task_id -eq $task.id })[0]
$workspaceRow = @($db.worker_workspaces | Where-Object { $_.task_id -eq $task.id })[0]
$terminal = @($db.worker_observations | Where-Object { $_.reason -eq 'provider_terminal' })[0]
$providerPid = [int64]$session.process_pid
$providerProcessAlive = [bool](Get-Process -Id $providerPid -ErrorAction SilentlyContinue)
$protocolPath = Join-Path $evidence 'provider-evidence\e3aab47b7169ebfa3b8072a82575a9ec\protocol.jsonl'
$protocolBytes = (Get-Item -LiteralPath $protocolPath).Length
$providerTaskCount = @($db.tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' }).Count
$providerEmployeeCount = @($db.worker_sessions | Where-Object { $_.employee_id -eq 'emp-backend' }).Count
$providerSessionCount = @($db.worker_sessions).Count
$publicAcceptance = $final.mission.acceptanceContract

$result = [ordered]@{
  task = 'R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_3'
  result = 'INCONCLUSIVE'
  classification = 'provider/runtime failure before initialize/thread/turn; no retry authorized'
  company = [ordered]@{ id = 'r05b-live-3'; bootstrap = 'controlled Company/roster bootstrap only; no Mission/Task SQL creation' }
  mission = [ordered]@{ id = $final.mission.missionId; title = $final.mission.title; state = $final.mission.state; created_through = 'Workbench Real mode'; started_through = 'Workbench Real mode'; cancel_post_count = 0; acceptance_contract = $publicAcceptance }
  task_roles = [ordered]@{
    total_task_rows = @($db.tasks).Count
    tasks = @($db.tasks | ForEach-Object { [ordered]@{id=$_.id; kind=$_.kind; owner=$_.owner; state=$_.state; generation=$_.generation; mission_id=$_.mission_id} })
    provider_executable_task_count = $providerTaskCount
    provider_executable_employee_count = $providerEmployeeCount
    provider_bearing_worker_session_count = $providerSessionCount
    planning_provider_usage = 0
  }
  task_validation_binding = [ordered]@{ present = ($null -ne $binding); task_id = $binding.task_id; mission_id = $binding.mission_id; configuration_digest = $binding.configuration_digest; acceptance_revision = $binding.acceptance_revision; runner_kind = $binding.runner_kind; runner_revision = $binding.runner_revision; public_contract = $publicAcceptance }
  workspace = [ordered]@{ task_id = $workspaceRow.task_id; digest = $workspaceRow.digest; revision = $workspaceRow.revision; initial_content_written = ($workspaceRow.revision -eq 1); deliverable_produced = $false }
  provider_binding = [ordered]@{
    authorization_count = 1
    reservation_count = [int]$allowance.medium_turns
    purpose = $preflight.purpose
    model = 'gpt-5.6-luna'
    effort = 'medium'
    profile = 'gpt-5.6-luna/medium'
    provider_mode = 'real'
    surface_id = $manifest.execution_identity.surface_id
    tool_count = $manifest.execution_identity.tool_count
    manifest_digest = $manifest.execution_identity.manifest_digest
    aggregate_schema_bytes = $manifest.execution_identity.aggregate_schema_bytes
    aggregate_schema_digest = $manifest.execution_identity.aggregate_schema_digest
    exact_surface_execution_fingerprint = $manifest.product_exact_surface_execution_fingerprint
    product_provider_v4_L2_fingerprint = $l2.product_provider_v4_L2_fingerprint
    execution_envelope = $manifest.execution_identity.launch_runtime_envelope.fingerprint
    runtime_version = $manifest.execution_identity.provider_runtime.native_version
    binary_sha256 = $manifest.execution_identity.provider_runtime.binary_sha256
    helper_sha256 = $manifest.execution_identity.provider_runtime.helper_sha256
    task_id = $task.id
    task_kind = $task.kind
    task_owner = $task.owner
    employee_id = $session.employee_id
    employee_role = 'backend'
    session_id = $session.id
    epoch = $session.epoch
    incarnation = $session.incarnation
    task_validation_binding_digest = $binding.configuration_digest
    workspace_digest = $workspaceRow.digest
    workspace_revision = $workspaceRow.revision
    persistence = 'authorization object was validated and accepted by Reserve; terminal usage was null because Initialize failed, so no credential/authorization payload was persisted in terminal evidence'
  }
  provider = [ordered]@{
    provider_reservation = [int]$allowance.medium_turns
    provider_egress = 0
    turn_start = 0
    real_provider_turn = 0
    first_output_latency_ms = $null
    elapsed_ms = $null
    input_tokens = $null
    output_tokens = $null
    tool_calls = [int]$session.tool_calls_used
    reconnects = 0
    process_pid = $providerPid
    process_created_and_attached = $true
    initialize = 'FAILED_OR_UNOBSERVED; provider protocol file is empty'
    thread_start = 'NOT_SENT/NOT_OBSERVED'
    terminal_state = $terminal.data.state
    terminal_outcome = $terminal.data.outcome
    terminal_usage = $terminal.data.usage
    protocol_path = 'provider-evidence/e3aab47b7169ebfa3b8072a82575a9ec/protocol.jsonl'
    protocol_bytes = $protocolBytes
    provider_process_alive_after_capture = $providerProcessAlive
    orphan_provider_process = [int]$providerProcessAlive
  }
  delivery = [ordered]@{ workspace_check = 'NOT_RUN'; task_submit = 'NOT_RUN'; qualified_checkpoint_count = @($db.worker_checkpoints).Count; artifact_count = @($db.artifacts).Count; artifact_digest = $null; artifact_cas_resolution = 'NOT_REACHED_NO_ARTIFACT'; task_final_state = $task.state; mission_final_state = $final.mission.state }
  browser_observation = $browser
  activity = [ordered]@{ snapshot_cursor = $activity.meta.snapshotCursor; event_count = @($activity.items).Count; provider_turn_inconclusive = $true; worker_terminal = $true; sse_requests = 0 }
  counters = [ordered]@{ mission_business_instances = 1; second_missions = 0; provider_executable_tasks = $providerTaskCount; provider_executable_employees = $providerEmployeeCount; provider_bearing_sessions = $providerSessionCount; business_authorizations = 1; provider_reservations = [int]$allowance.medium_turns; provider_egress = 0; real_provider_turns = 0; retries = 0; successors = 0; high = 0; duplicate_reservations = 0; orphan_sessions = 0; orphan_provider_processes = [int]$providerProcessAlive; orphan_provider_turns = 0 }
  surface = [ordered]@{ surface_id = $manifest.execution_identity.surface_id; manifest_digest = $manifest.execution_identity.manifest_digest; schema_bytes = $manifest.execution_identity.aggregate_schema_bytes; schema_digest = $manifest.execution_identity.aggregate_schema_digest; exact_surface_execution_fingerprint = $manifest.product_exact_surface_execution_fingerprint; provider_L2_fingerprint = $l2.product_provider_v4_L2_fingerprint; launch_envelope_fingerprint = $manifest.execution_identity.launch_runtime_envelope.fingerprint; b13_launch_envelope_fingerprint = $b13.current_launch_envelope_fingerprint }
  evidence = [ordered]@{ live_3_result = 'live-3-result.json'; authoritative_final_state = 'authoritative-final-state.json'; postgres_final_state = 'postgres-final-state.json'; activity_snapshot = 'activity-snapshot.json'; browser_e2e = 'browser-e2e-result.json'; provider_terminal = 'postgres-final-state.json#worker_observations'; provider_protocol = 'provider-evidence/e3aab47b7169ebfa3b8072a82575a9ec/protocol.jsonl'; post_provider_query = 'post-provider-state-query.txt'; no_credentials_recorded = $true }
  post_attempt = [ordered]@{ no_provider_traffic_after_terminal = $true; no_second_start = $true; no_cancel_command = $true; live2_historical_evidence_modified = $false; sse_started = $false; multi_agent_e2e_started = $false }
}
$result | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'live-3-result.json')
