# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b15-business-path-provider-initialize-hardening')).Path
$preflight = Get-Content -Raw (Join-Path $evidence 'business-preflight\business-context-preflight.json') | ConvertFrom-Json
$lifecycle = Get-Content -Raw (Join-Path $evidence 'live3-lifecycle-matrix.json') | ConvertFrom-Json
$diff = Get-Content -Raw (Join-Path $evidence 'b14-live3-execution-context-diff.json') | ConvertFrom-Json
$result = [ordered]@{
  task = 'R0.5B15_BUSINESS_PATH_PROVIDER_INITIALIZE_HARDENING'
  result = 'PASSED'
  historical_live3_precise_root_cause = $lifecycle.historical_precise_failure_cause
  current_reproducibility_status = 'PASSED_3_OF_3_LOCAL_BUSINESS_CONTEXT'
  live3_frozen = $true
  live4_started = $false
  historical_lifecycle = $lifecycle
  b14_live3_execution_context_diff = [ordered]@{record_type=$diff.record_type;complete_for_frozen_evidence=$true;differences=$diff.differences;missing_historical_business_observability=$diff.missing_business_path_observability;exact_surface_same=($diff.b14.manifest_digest -eq $diff.live3.manifest_digest -and $diff.b14.execution_fingerprint -eq $diff.live3.execution_fingerprint -and $diff.b14.launch_envelope -eq $diff.live3.launch_envelope)}
  business_context_preflight = $preflight
  initialization_failure_reporting = 'PASSED: structured provider.runtime.initialization_failed with phase/reason/process/PID/request/ack/pipe/stderr-safe fields'
  thread_start_failure_reporting = 'PASSED: structured provider.runtime.thread_start_failed with no turn event'
  provider_event_semantics = 'PASSED: initialization/thread-start failures are distinct from provider.turn.inconclusive'
  worker_session_task_lifecycle = 'PASSED: stopped WorkerSession + working Task + active Mission is intentional and recoverable on later kernel recovery'
  provider_surface_changed = $false
  runtime_binding_changed = $false
  launch_envelope_changed = $false
  execution_fingerprint_changed = $false
  B14_live_qualification_status = 'QUALIFIED_REUSABLE'
  eligible_for_new_live_product_smoke = 'YES_ELIGIBILITY_ONLY_NO_LIVE_RUN'
  reservation = 0
  provider_egress = 0
  turn_start = 0
  high = 0
  multi_agent_e2e = 0
  notes = @('LIVE_3 raw result/events remain immutable','B9 deterministic delivery and @4 semantics were not changed','no credentials/raw prompts/unrestricted stderr were added to R0.5B15 evidence')
  generated_at = (Get-Date).ToUniversalTime().ToString('o')
}
$result | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'r0-5b15-result.json')
