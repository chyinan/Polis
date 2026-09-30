# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-5'
$protocolPath = Get-ChildItem -LiteralPath (Join-Path $evidence 'provider-evidence') -Recurse -Filter protocol.jsonl -File | Select-Object -First 1
$lines = @(Get-Content -LiteralPath $protocolPath.FullName -Encoding UTF8)
$records = @($lines | ForEach-Object { $_ | ConvertFrom-Json })
$lifecycle = @($records | Where-Object { $_.direction -eq 'lifecycle' })
$sends = @($records | Where-Object { $_.direction -eq 'send' })
$receives = @($records | Where-Object { $_.direction -eq 'receive' })
$toolResults = @($records | Where-Object { $_.direction -eq 'tool_result' })
$initializeSend = @($sends | Where-Object { $_.data.method -eq 'initialize' })[0]
$threadSend = @($sends | Where-Object { $_.data.method -eq 'thread/start' })[0]
$turnSend = @($sends | Where-Object { $_.data.method -eq 'turn/start' })[0]
$firstDelta = @($receives | Where-Object { $_.data.method -eq 'item/agentMessage/delta' })[0]
$turnCompleted = @($receives | Where-Object { $_.data.method -eq 'turn/completed' })[0]
$threadStarted = @($receives | Where-Object { $_.data.method -eq 'thread/started' })[0]
$threadResponse = @($receives | Where-Object { $_.data.id -eq 2 })[0]
$initializeResponse = @($receives | Where-Object { $_.data.id -eq 1 -and $null -ne $_.data.result.userAgent })[0]
$usageUpdates = @($receives | Where-Object { $_.data.method -eq 'thread/tokenUsage/updated' })
$usage = if ($usageUpdates.Count -gt 0) { $usageUpdates[-1].data.params.tokenUsage } else { $null }
$db = Get-Content -Raw -Encoding UTF8 (Join-Path $evidence 'postgres-final-state.json') | ConvertFrom-Json
$terminalObservation = @($db.worker_observations | Where-Object { $_.reason -eq 'provider_terminal' })[0]
$terminal = $terminalObservation.data
$usageTerminal = $terminal.usage
$auth = $usageTerminal.authorization
$firstOutputAt = if ($null -eq $firstDelta) { $null } else { $firstDelta.time }
$turnStartAt = if ($null -eq $turnSend) { $null } else { $turnSend.time }
$turnCompletedAt = if ($null -eq $turnCompleted) { $null } else { $turnCompleted.time }
$elapsedMs = if ($null -eq $turnStartAt -or $null -eq $firstOutputAt) { $null } else { [int][Math]::Round(([DateTime]::Parse($firstOutputAt) - [DateTime]::Parse($turnStartAt)).TotalMilliseconds) }
$summary = [ordered]@{
  record_type = 'polis-live5-provider-terminal-summary@1'
  protocol_path = $protocolPath.FullName.Substring($evidence.Length + 1)
  initialize = [ordered]@{
    request_count = @($sends | Where-Object { $_.data.method -eq 'initialize' }).Count
    response_count = @($receives | Where-Object { $_.data.id -eq 1 -and $null -ne $_.data.result.userAgent }).Count
    request_sent_at = if ($null -eq $initializeSend) { $null } else { $initializeSend.time }
    response_received_at = if ($null -eq $initializeResponse) { $null } else { $initializeResponse.time }
    lifecycle_phases = @($lifecycle | ForEach-Object { $_.data.phase })
    ack_received = (@($lifecycle | Where-Object { $_.data.phase -eq 'initialize_ack_received' }).Count -eq 1)
    user_agent = if ($null -eq $initializeResponse) { $null } else { $initializeResponse.data.result.userAgent }
  }
  thread_start = [ordered]@{
    request_count = @($sends | Where-Object { $_.data.method -eq 'thread/start' }).Count
    response_count = @($receives | Where-Object { $_.data.id -eq 2 -and $null -ne $_.data.result.thread }).Count
    thread_started_count = @($receives | Where-Object { $_.data.method -eq 'thread/started' }).Count
    request_sent_at = if ($null -eq $threadSend) { $null } else { $threadSend.time }
    response_received_at = if ($null -eq $threadResponse) { $null } else { $threadResponse.time }
    registered_tool_count = if ($null -eq $threadSend) { 0 } else { @($threadSend.data.params.dynamicTools).Count }
    registered_surface_digest = if ($null -eq $auth) { $null } else { $auth.ToolSurfaceDigest }
  }
  turn = [ordered]@{
    start_count = @($sends | Where-Object { $_.data.method -eq 'turn/start' }).Count
    completed_count = @($receives | Where-Object { $_.data.method -eq 'turn/completed' }).Count
    turn_start_at = $turnStartAt
    first_output_at = $firstOutputAt
    first_output_delta = if ($null -eq $firstDelta) { $null } else { $firstDelta.data.params.delta }
    first_output_latency_ms = $elapsedMs
    completed_at = $turnCompletedAt
    tool_result_count = $toolResults.Count
    reconnect_count = 0
    token_usage = $usage
    provider_egress = [int]$usageTerminal.provider_egress
    terminal_state = $terminal.state
    outcome = $terminal.outcome
  }
  authorization = $auth
  worker_terminal = $terminal
}
$summary | ConvertTo-Json -Depth 80 | Set-Content -LiteralPath (Join-Path $evidence 'provider-terminal-summary.json')
$summary.initialize | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'initialization-lifecycle.json')
Write-Output 'LIVE_5_PROVIDER_SUMMARY_CAPTURED'
