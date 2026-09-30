# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-3')).Path
$db = Get-Content -Raw (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$result = Get-Content -Raw (Join-Path $evidence 'live-3-result.json') | ConvertFrom-Json
$allowance = Get-Content -Raw (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$final = Get-Content -Raw (Join-Path $evidence 'authoritative-final-state.json') | ConvertFrom-Json
$protocolPath = Join-Path $evidence 'provider-evidence\e3aab47b7169ebfa3b8072a82575a9ec\protocol.jsonl'
$workspaceDigest = $result.workspace.digest
$casPath = Join-Path $evidence ("cas\r05b-live-3\" + $workspaceDigest)
$providerPid = [int64]$result.provider.process_pid
$checks = [ordered]@{
  company_count = [int](@($db.company).Count)
  mission_count = [int](@($db.missions).Count)
  task_count = [int](@($db.tasks).Count)
  provider_task_count = [int](@($db.tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' }).Count)
  binding_count = [int](@($db.task_validation_bindings).Count)
  worker_session_count = [int](@($db.worker_sessions).Count)
  stopped_worker_session_count = [int](@($db.worker_sessions | Where-Object { $_.state -eq 'stopped' }).Count)
  provider_terminal_observation_count = [int](@($db.worker_observations | Where-Object { $_.reason -eq 'provider_terminal' }).Count)
  worker_check_count = [int](@($db.worker_checks).Count)
  checkpoint_count = [int](@($db.worker_checkpoints).Count)
  artifact_count = [int](@($db.artifacts).Count)
  artifact_qualification_count = [int](@($db.task_validation_artifact_qualifications).Count)
  mission_state = $final.mission.state
  task_state = (@($db.tasks | Where-Object { $_.kind -eq 'compat' })[0]).state
  allowance_medium_turns = [int]$allowance.medium_turns
  allowance_high_turns = [int]$allowance.high_turns
  allowance_tool_calls_used = [int]$allowance.tool_calls_used
  protocol_bytes = (Get-Item -LiteralPath $protocolPath).Length
  cas_workspace_blob_present = (Test-Path -LiteralPath $casPath)
  provider_process_alive = [bool](Get-Process -Id $providerPid -ErrorAction SilentlyContinue)
  no_credentials_recorded = (@(Get-ChildItem -LiteralPath $evidence -Recurse -Force -File -ErrorAction SilentlyContinue | Where-Object { $_.Name -match '(^|[-_.])(auth|credential|secret|token)([-_.]|$)' }).Count -eq 0)
}
$pass = $checks.company_count -eq 1 -and $checks.mission_count -eq 1 -and $checks.task_count -eq 2 -and $checks.provider_task_count -eq 1 -and $checks.binding_count -eq 1 -and $checks.worker_session_count -eq 1 -and $checks.stopped_worker_session_count -eq 1 -and $checks.provider_terminal_observation_count -eq 1 -and $checks.worker_check_count -eq 0 -and $checks.checkpoint_count -eq 0 -and $checks.artifact_count -eq 0 -and $checks.artifact_qualification_count -eq 0 -and $checks.mission_state -eq 'active' -and $checks.task_state -eq 'working' -and $checks.allowance_medium_turns -eq 1 -and $checks.allowance_high_turns -eq 0 -and $checks.allowance_tool_calls_used -eq 0 -and $checks.protocol_bytes -eq 0 -and $checks.cas_workspace_blob_present -and -not $checks.provider_process_alive -and $checks.no_credentials_recorded
[ordered]@{status=if($pass){'PASS'}else{'FAIL'}; checks=$checks} | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidence 'post-attempt-integrity.json')
if (-not $pass) { throw 'LIVE_3_POST_ATTEMPT_INTEGRITY_FAILED' }
