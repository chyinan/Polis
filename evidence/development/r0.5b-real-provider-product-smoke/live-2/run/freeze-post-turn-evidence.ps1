$ErrorActionPreference = 'Stop'
$evidence = $PSScriptRoot
$statePath = Join-Path $evidence 'current-state.json'
$state = Get-Content -Raw -LiteralPath $statePath | ConvertFrom-Json
$mission = $state.mission
$tasks = @($state.tasks)
$bindings = @($state.task_validation_bindings)
$sessions = @($state.worker_sessions)
$checks = @($state.checks)
$terminalRows = @($state.provider_terminal_observations)
$artifacts = @($state.artifacts)
$checkpoints = @($state.checkpoints)
$qualifications = @($state.artifact_qualifications)
$providerTaskRows = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.owner -eq 'emp-backend' })
$providerSessions = @($sessions | Where-Object { $_.employee_id -eq 'emp-backend' -and $_.task_id -eq $providerTaskRows[0].id })
$passingChecks = @($checks | Where-Object { $_.phase -eq 'product' -and $_.passed })
$runtimePath = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421'
$providerExecutables = @((Join-Path $runtimePath 'codex.exe'),(Join-Path $runtimePath 'codex-code-mode-host.exe'))
$liveProviderProcesses = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.ExecutablePath -and $providerExecutables -contains $_.ExecutablePath })

if ($tasks.Count -ne 2 -or $providerTaskRows.Count -ne 1 -or $bindings.Count -ne 1 -or $sessions.Count -ne 1 -or $providerSessions.Count -ne 1) { throw 'LIVE_2 terminal Task/session cardinality is inconsistent' }
if ($terminalRows.Count -ne 1 -or $terminalRows[0].data.state -ne 'completed' -or $terminalRows[0].data.outcome -ne 'COMPLETED') { throw 'LIVE_2 did not persist exactly one normally completed provider terminal record' }
if ($terminalRows[0].data.usage.provider_egress -ne 1 -or $providerSessions[0].state -ne 'stopped') { throw 'LIVE_2 provider egress or WorkerSession terminal state is inconsistent' }
if ($passingChecks.Count -ne 1 -or $checkpoints.Count -ne 0 -or $artifacts.Count -ne 0 -or $qualifications.Count -ne 0) { throw 'LIVE_2 public checker/checkpoint/Artifact evidence does not match the observed terminal state' }
if ($liveProviderProcesses.Count -ne 0) { throw 'A LIVE_2 Codex/provider process remains after terminal cleanup' }

$terminal = $terminalRows[0].data
$usage = $terminal.usage
$authorization = $usage.authorization
$allowancePath = Join-Path $evidence 'allowance.json'
if (-not (Test-Path -LiteralPath $allowancePath)) { throw 'LIVE_2 allowance record is missing after a provider egress' }
$allowance = Get-Content -Raw -LiteralPath $allowancePath | ConvertFrom-Json
if ($allowance.medium_turns -ne 1 -or $allowance.high_turns -ne 0 -or $allowance.medium_limit -ne 1 -or $allowance.high_limit -ne 0) { throw 'LIVE_2 allowance counters exceed the authorized single-medium attempt' }

$protocolPath = Join-Path $evidence (Join-Path (Join-Path 'provider-evidence' $providerSessions[0].id) 'protocol.jsonl')
$protocol = @(Get-Content -LiteralPath $protocolPath | ForEach-Object { $_ | ConvertFrom-Json })
$turnStart = $protocol | Where-Object { $_.direction -eq 'send' -and $_.data.method -eq 'turn/start' } | Select-Object -First 1
$firstOutput = $protocol | Where-Object { $_.direction -eq 'receive' -and $_.data.method -eq 'item/agentMessage/delta' } | Select-Object -First 1
$turnCompleted = $protocol | Where-Object { $_.direction -eq 'receive' -and $_.data.method -eq 'turn/completed' } | Select-Object -First 1
$turnStartCount = @($protocol | Where-Object { $_.direction -eq 'send' -and $_.data.method -eq 'turn/start' }).Count
$toolCallCount = @($protocol | Where-Object { $_.direction -eq 'receive' -and $_.data.method -eq 'item/tool/call' }).Count
$reconnectMarkers = @($protocol | Where-Object { $_.data.method -match 'reconnect|resume' }).Count
if ($turnStartCount -ne 1 -or $toolCallCount -ne $providerSessions[0].tool_calls_used) { throw 'LIVE_2 protocol log and WorkerSession tool-call counts disagree' }
$culture = [Globalization.CultureInfo]::InvariantCulture
$dateStyle = [Globalization.DateTimeStyles]::AssumeUniversal
$timestampFormat = "yyyy-MM-dd'T'HH:mm:ss.fffffff'Z'"
$startedText = if ($usage.started_at -is [DateTime]) { $usage.started_at.ToUniversalTime().ToString($timestampFormat, $culture) } else { [string]$usage.started_at }
$finishedText = if ($usage.finished_at -is [DateTime]) { $usage.finished_at.ToUniversalTime().ToString($timestampFormat, $culture) } else { [string]$usage.finished_at }
$turnStartText = if ($turnStart.time -is [DateTime]) { $turnStart.time.ToUniversalTime().ToString($timestampFormat, $culture) } else { [string]$turnStart.time }
$firstOutputText = if ($firstOutput.time -is [DateTime]) { $firstOutput.time.ToUniversalTime().ToString($timestampFormat, $culture) } else { [string]$firstOutput.time }
$startedAt = [DateTimeOffset]::ParseExact($startedText, $timestampFormat, $culture, $dateStyle)
$finishedAt = [DateTimeOffset]::ParseExact($finishedText, $timestampFormat, $culture, $dateStyle)
$protocolTurnStart = [DateTimeOffset]::ParseExact($turnStartText, $timestampFormat, $culture, $dateStyle)
$protocolFirstOutput = [DateTimeOffset]::ParseExact($firstOutputText, $timestampFormat, $culture, $dateStyle)

$companyMission = [pscustomobject]@{
  company_id = $state.company.id
  company_seq = $state.company.company_seq
  employee_count = @($state.company.employees).Count
  mission = $mission
}
$taskRoleEvidence = [pscustomobject]@{
  task_count = $tasks.Count
  tasks = $tasks
  provider_executable_task_count = $providerTaskRows.Count
  provider_executable_employee_count = 1
  provider_bearing_worker_session_count = $providerSessions.Count
  planning_worker_session_count = @($sessions | Where-Object { $_.employee_id -eq 'emp-planning' }).Count
}
$providerSession = $providerSessions[0]
$check = $passingChecks[0]
$classification = 'FAILED'
$reason = 'The provider turn completed normally and the public workspace check passed, but the employee stopped without a qualified checkpoint or Artifact.'
$providerSummary = [pscustomobject]@{
  authorization_count = 1
  reservation_count = $allowance.medium_turns
  medium_turns = $allowance.medium_turns
  high_turns = $allowance.high_turns
  provider_egress = $usage.provider_egress
  turn_terminal = $terminal.state
  turn_outcome = $terminal.outcome
  model = $authorization.Model
  effort = $authorization.Effort
  purpose = $authorization.Purpose
  surface_id = $authorization.ToolSurfaceQualification
  tool_count = $authorization.ToolCount
  manifest_digest = $authorization.ToolSurfaceDigest
  aggregate_schema_bytes = $authorization.AggregateSchemaBytes
  aggregate_schema_digest = $authorization.AggregateSchemaDigest
  exact_surface_execution_fingerprint = $authorization.ExactSurfaceExecutionFingerprint
  product_provider_l2_fingerprint = $authorization.ProductProviderL2Fingerprint
  task_validation_binding_digest = $authorization.TaskValidationBindingDigest
  initial_workspace_digest = $authorization.WorkspaceDigest
  initial_workspace_revision = $authorization.WorkspaceRevision
  elapsed_ms = [Math]::Round(($finishedAt - $startedAt).TotalMilliseconds, 3)
  time_to_first_output_ms = [Math]::Round(($protocolFirstOutput - $protocolTurnStart).TotalMilliseconds, 3)
  protocol_turn_start_count = $turnStartCount
  protocol_turn_completed_present = ($null -ne $turnCompleted)
  tool_calls = $providerSessions[0].tool_calls_used
  reconnects_observed = $reconnectMarkers
  reconnect_evidence = 'The terminal usage object did not persist the runtime reconnect counter; protocol.jsonl contains one turn/start and no reconnect/resume method markers.'
  token_usage = $usage.token_usage
  terminal_observation = $terminal
}
$result = [pscustomobject]@{
  task = 'R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_2'
  result = $classification
  reason = $reason
  company_id = $state.company.id
  mission_id = $mission.id
  mission_state_before_cleanup = $mission.state
  task_roles = $taskRoleEvidence
  task_validation_binding = $bindings[0]
  provider_session = $providerSession
  provider = $providerSummary
  workspace_check = $check
  workspace_after_check = $state.workspaces
  qualified_checkpoint_count = $checkpoints.Count
  artifact_count = $artifacts.Count
  artifact_qualification_count = $qualifications.Count
  artifact_digest = $null
  artifact_cas_resolution = 'NOT_REACHED_NO_ARTIFACT'
  frontend_observation = @('overview','tasks','activity')
  activity_event_count = @($state.activity).Count
  retries = 0
  successors = 0
  high_turns = $allowance.high_turns
  second_missions = 0
  duplicate_reservations = 0
  live_provider_sessions = @($sessions | Where-Object { $_.state -ne 'stopped' }).Count
  provider_process_count = $liveProviderProcesses.Count
  browser_start_click_count = 1
  provider_terminal_record_count = $terminalRows.Count
  evidence_source = 'current-state.json plus provider-evidence session protocol.jsonl'
}

function Write-JsonEvidence([string]$Name, $Value) {
  $json = ConvertTo-Json -InputObject $Value -Depth 30
  [IO.File]::WriteAllText((Join-Path $evidence $Name), $json + "`n", [Text.UTF8Encoding]::new($false))
}

Copy-Item -LiteralPath (Join-Path $evidence 'current-state.json') -Destination (Join-Path $evidence 'post-turn-pre-cancel-state.json') -Force
Copy-Item -LiteralPath (Join-Path $evidence 'current-counts.json') -Destination (Join-Path $evidence 'post-turn-pre-cancel-counts.json') -Force
Write-JsonEvidence 'company-mission-snapshot.json' $companyMission
Write-JsonEvidence 'task-role-snapshot.json' $taskRoleEvidence
Write-JsonEvidence 'task-validation-binding.json' $bindings[0]
Write-JsonEvidence 'business-authorization-binding.json' $authorization
Write-JsonEvidence 'provider-bearing-worker-session.json' $providerSession
Write-JsonEvidence 'provider-terminal-summary.json' $providerSummary
Write-JsonEvidence 'workspace-check-receipt.json' $check
Write-JsonEvidence 'checkpoint-metadata.json' $checkpoints
Write-JsonEvidence 'artifact-metadata.json' $artifacts
Write-JsonEvidence 'activity-snapshot.json' $state.activity
Write-JsonEvidence 'live-2-result.json' $result
[pscustomobject]@{result=$classification;reason=$reason;provider_egress=$usage.provider_egress;tool_calls=$providerSessions[0].tool_calls_used;terminal=$terminal.state;artifact_count=$artifacts.Count;checkpoint_count=$checkpoints.Count;live_sessions=$result.live_provider_sessions;provider_process_count=0} | ConvertTo-Json -Compress
