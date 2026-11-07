$ErrorActionPreference = 'Stop'
$evidence = $PSScriptRoot
$resultPath = Join-Path $evidence 'live-2-result.json'
$result = Get-Content -Raw -LiteralPath $resultPath | ConvertFrom-Json
$state = Get-Content -Raw -LiteralPath (Join-Path $evidence 'current-state.json') | ConvertFrom-Json
$browser = Get-Content -Raw -LiteralPath (Join-Path $evidence 'browser-final.json') | ConvertFrom-Json
$cancel = Get-Content -Raw -LiteralPath (Join-Path $evidence 'browser-cancel.json') | ConvertFrom-Json
$allowance = Get-Content -Raw -LiteralPath (Join-Path $evidence 'allowance.json') | ConvertFrom-Json
$protocolPath = Join-Path $evidence (Join-Path (Join-Path 'provider-evidence' $result.provider_session.id) 'protocol.jsonl')
$protocol = @(Get-Content -LiteralPath $protocolPath | ForEach-Object { $_ | ConvertFrom-Json })
$protocolTurnStart = $protocol | Where-Object { $_.direction -eq 'send' -and $_.data.method -eq 'turn/start' } | Select-Object -First 1
$protocolFirstOutput = $protocol | Where-Object { $_.direction -eq 'receive' -and $_.data.method -eq 'item/agentMessage/delta' } | Select-Object -First 1
$protocolTurnCompleted = $protocol | Where-Object { $_.direction -eq 'receive' -and $_.data.method -eq 'turn/completed' } | Select-Object -First 1
$protocolToolCalls = @($protocol | Where-Object { $_.direction -eq 'receive' -and $_.data.method -eq 'item/tool/call' }).Count
$reconnectMarkers = @($protocol | Where-Object { $_.data.method -match 'reconnect|resume' }).Count
$finalTasks = @($state.tasks | Select-Object id,kind,owner,state,generation)
$finalSessions = @($state.worker_sessions | Select-Object id,task_id,employee_id,state,epoch,incarnation,process_pid,stop_receipt,tool_call_limit,tool_calls_used)
$runtimePath = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421'
$providerExecutables = @((Join-Path $runtimePath 'codex.exe'),(Join-Path $runtimePath 'codex-code-mode-host.exe'))
$liveProviderProcesses = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.ExecutablePath -and $providerExecutables -contains $_.ExecutablePath })

if ($result.result -ne 'FAILED' -or $state.mission.state -ne 'cancelled' -or $allowance.medium_turns -ne 1 -or $allowance.high_turns -ne 0) { throw 'LIVE_2 terminal result or final lifecycle state is inconsistent' }
if ($browser.missionCreatePostCount -ne 1 -or $browser.missionStartPostCount -ne 1 -or $browser.missionCancelPostCount -ne 1) { throw 'LIVE_2 Workbench command cardinality is inconsistent' }
if ($state.provider_terminal_observations.Count -ne 1 -or $state.provider_terminal_observations[0].data.usage.provider_egress -ne 1 -or $protocolToolCalls -ne 13 -or $reconnectMarkers -ne 0) { throw 'LIVE_2 provider protocol and persisted terminal evidence disagree' }
if (@($state.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count -ne 0 -or $liveProviderProcesses.Count -ne 0) { throw 'LIVE_2 has a live WorkerSession or provider process after cleanup' }

$result | Add-Member -Force -NotePropertyName mission_state_final -NotePropertyValue $state.mission.state
$result | Add-Member -Force -NotePropertyName task_final_states -NotePropertyValue $finalTasks
$result | Add-Member -Force -NotePropertyName worker_session_final_states -NotePropertyValue $finalSessions
$result | Add-Member -Force -NotePropertyName post_attempt_cleanup -NotePropertyValue ([pscustomobject]@{
  command = 'mission.cancel'
  browser_response_status = $cancel.responseStatus
  browser_command_count = $browser.missionCancelPostCount
  mission_state = $state.mission.state
  provider_egress_after_terminal = 0
  live_provider_sessions = @($state.worker_sessions | Where-Object { $_.state -ne 'stopped' }).Count
  live_provider_processes = $liveProviderProcesses.Count
})
$result | Add-Member -Force -NotePropertyName browser_e2e -NotePropertyValue $browser
$result.provider | Add-Member -Force -NotePropertyName protocol_turn_start_at -NotePropertyValue $protocolTurnStart.time
$result.provider | Add-Member -Force -NotePropertyName protocol_first_output_at -NotePropertyValue $protocolFirstOutput.time
$result.provider | Add-Member -Force -NotePropertyName protocol_turn_completed_at -NotePropertyValue $protocolTurnCompleted.time
$result.provider | Add-Member -Force -NotePropertyName protocol_tool_call_event_count -NotePropertyValue $protocolToolCalls
$result.provider | Add-Member -Force -NotePropertyName worker_session_tool_calls_used -NotePropertyValue $finalSessions[0].tool_calls_used
$result.provider | Add-Member -Force -NotePropertyName codex_terminal_usage_tool_calls_field -NotePropertyValue $state.provider_terminal_observations[0].data.usage.tool_calls
$result.provider | Add-Member -Force -NotePropertyName codex_terminal_usage_tool_calls_note -NotePropertyValue 'The Codex usage field is not authoritative for mediated Polis tool calls; WorkerSession.tool_calls_used and protocol item/tool/call events both record 13.'
$result.provider | Add-Member -Force -NotePropertyName reconnect_markers_in_protocol -NotePropertyValue $reconnectMarkers
$result.provider | Add-Member -Force -NotePropertyName reconnect_count_note -NotePropertyValue 'The runtime reconnect counter was not persisted. No reconnect/resume protocol markers were observed.'

$json = ConvertTo-Json -InputObject $result -Depth 40
[IO.File]::WriteAllText($resultPath, $json + "`n", [Text.UTF8Encoding]::new($false))
[pscustomobject]@{result=$result.result;mission_state_final=$result.mission_state_final;compat_state=($finalTasks|Where-Object kind -eq 'compat').state;session_state=$finalSessions[0].state;provider_egress=$result.provider.provider_egress;tool_calls=$protocolToolCalls;live_provider_processes=$liveProviderProcesses.Count;mission_cancel_posts=$browser.missionCancelPostCount}|ConvertTo-Json -Compress
