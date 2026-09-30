param(
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-t21',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\bin\fd4c151a749f3ab4\codex.exe',
    [string]$CodeModeHost = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\bin\fd4c151a749f3ab4\codex-code-mode-host.exe',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedLinuxConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [switch]$PreflightOnly,
    [string]$PreflightEvidence = '',
    [string]$Qualification = 'R0.3A-T21'
)

$ErrorActionPreference = 'Stop'
$SourceEvidence = if ([string]::IsNullOrEmpty($PreflightEvidence)) { $Evidence } else { $PreflightEvidence }
$sentinelPrompt = "Do not call tools.`nReply only with the transport sentinel."
$sentinel = 'POLIS_TRANSPORT_CANARY_OK'
$proxyNames = @('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY', 'POLIS_NATIVE_PROXY')
$result = [ordered]@{
    schema_version = 't21-windows-live-run-v1'
    status = 'preflight_failed'
    preflight_only = [bool]$PreflightOnly
    preflight_passed = $false
    diagnostic_profile = 'windows_native_isolated'
    provider_or_internet_accessed = $false
    provider_egress = 0
    medium_started = 0
    auth_snapshot_used = $true
    config_snapshot_used = $true
    auth_source_role = 'read_only_external_auth_snapshot'
    config_source_role = 'exact_selected_non_secret_T14C_config_snapshot'
    model = 'gpt-5.6-luna'
    effort = 'medium'
    sandbox = 'read-only'
    provider_transport_policy = 'native_default'
    proxy_policy = 'no_injected_proxy'
    invocation = 'app-server --stdio'
    dynamic_tool_count = 11
    callback_registration = 'formal_PeerEmployeeTools_surface_with_diagnostic_denial_adapter'
    business_write_policy = 'diagnostic_denied'
    process_started = $false
    initialize_completed = $false
    thread_started = $false
    turn_started = $false
    user_message_item = $false
    registered_tool_count = 0
    tool_manifest_digest = ''
    aggregate_schema_digest = ''
    aggregate_schema_bytes = 0
    thread_start_payload_bytes = 0
    thread_start_payload_digest = ''
    attempted_tool_calls = @()
    business_side_effects = 0
    first_valid_output = $false
    first_valid_output_text = ''
    time_to_first_valid_output_ms = 0
    reconnect_count = 0
    reconnect_phases = @()
    recovery_timestamps = @()
    first_disconnect_delta_ms = 0
    recovery_delta_ms = 0
    native_usage_updates = 0
    token_usage = $null
    turn_completed = $false
    turn_state = ''
    sentinel_match = 'not_run'
    unresolved_transport_state = $false
    failure_mode = ''
    terminal_state = ''
    stop_confirmed = $false
    stop_mechanism = ''
    process_start = ''
    process_stop = ''
    error = ''
}
$process = $null
$diagnosticRoot = Join-Path ([IO.Path]::GetTempPath()) ('polis-r03a-t21-' + [guid]::NewGuid().ToString('N'))
$diagnosticHome = Join-Path $diagnosticRoot 'codex-home'
$workspace = Join-Path $diagnosticRoot 'workspace'
$events = New-Object System.Collections.Generic.List[object]
$outputParts = New-Object System.Collections.Generic.List[string]
$turnClock = $null
$reconnectOpen = $false
$stderrTask = $null

function Hash-Bytes([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}
function Hash-File([string]$Path) { return Hash-Bytes ([IO.File]::ReadAllBytes($Path)) }
function Write-JsonFile([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Depth 50
    [IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}
function Add-ProtocolEvent($Direction, $Message, $LineEnding) {
    $entry = [ordered]@{ timestamp = [DateTime]::UtcNow.ToString('o'); direction = $Direction; line_ending = $LineEnding }
    if ($null -ne $Message.id) { $entry.id = [string]$Message.id }
    if ($null -ne $Message.method) { $entry.method = [string]$Message.method }
    if ([string]$Message.method -eq 'item/tool/call') {
        $entry.tool_call = $true
        $entry.tool = [string]$Message.params.tool
        $entry.call_id = [string]$Message.params.callId
    }
    if ([string]$Message.method -eq 'error') {
        $entry.error_class = if ($null -ne $Message.params.error.codexErrorInfo.responseStreamDisconnected) { 'response_stream_disconnected' } else { 'structured_error' }
        $entry.will_retry = [bool]$Message.params.willRetry
    }
    if ([string]$Message.method -eq 'item/agentMessage/delta') { $entry.text = [string]$Message.params.delta }
    if ([string]$Message.method -match 'tokenUsage|usage') { $entry.usage_present = $true }
    $null = $events.Add([pscustomobject]$entry)
}
function Send-Frame($Message) {
    $json = $Message | ConvertTo-Json -Compress -Depth 50
    $process.StandardInput.Write($json + "`r`n")
    $process.StandardInput.Flush()
    Add-ProtocolEvent 'send' ($json | ConvertFrom-Json) 'CRLF'
    return $json
}
function Read-Frame([int]$TimeoutMs) {
    $stream = $process.StandardOutput.BaseStream
    $buffer = [byte[]]::new(1)
    $line = New-Object System.Collections.Generic.List[byte]
    $deadline = [DateTime]::UtcNow.AddMilliseconds($TimeoutMs)
    while ([DateTime]::UtcNow -lt $deadline) {
        $task = $stream.ReadAsync($buffer, 0, 1)
        $remaining = [Math]::Max(1, [int]$deadline.Subtract([DateTime]::UtcNow).TotalMilliseconds)
        try { $completed = $task.Wait($remaining) } catch { return [pscustomobject]@{ ok = $false; error = 'read_error' } }
        if (-not $completed) { return [pscustomobject]@{ ok = $false; error = 'read_timeout' } }
        if ($task.Result -eq 0) { return [pscustomobject]@{ ok = $false; error = 'eof' } }
        if ($buffer[0] -eq 10) {
            $lineEnding = 'LF'
            if ($line.Count -gt 0 -and $line[$line.Count - 1] -eq 13) { $line.RemoveAt($line.Count - 1); $lineEnding = 'CRLF' }
            $text = [Text.Encoding]::UTF8.GetString($line.ToArray())
            try { return [pscustomobject]@{ ok = $true; message = ($text | ConvertFrom-Json); line_ending = $lineEnding } }
            catch { return [pscustomobject]@{ ok = $false; error = 'invalid_json_frame'; line_ending = $lineEnding } }
        }
        $line.Add($buffer[0])
    }
    return [pscustomobject]@{ ok = $false; error = 'read_timeout' }
}
function Wait-ForResponse([int]$Id, [int]$TimeoutMs) {
    $deadline = [DateTime]::UtcNow.AddMilliseconds($TimeoutMs)
    while ([DateTime]::UtcNow -lt $deadline) {
        $frame = Read-Frame ([Math]::Max(1, [int]$deadline.Subtract([DateTime]::UtcNow).TotalMilliseconds))
        if (-not $frame.ok) { throw "protocol_$($frame.error)" }
        Add-ProtocolEvent 'receive' $frame.message $frame.line_ending
        if ($null -ne $frame.message.id -and [string]$frame.message.id -eq [string]$Id) { return $frame.message }
    }
    throw "protocol_response_timeout_$Id"
}
function Get-MessageText($Message) {
    if ($null -ne $Message.params.delta) { return [string]$Message.params.delta }
    if ($null -ne $Message.params.text) { return [string]$Message.params.text }
    if ($null -ne $Message.params.item.text) { return [string]$Message.params.item.text }
    if ($null -ne $Message.params.item.content) {
        $parts = foreach ($content in @($Message.params.item.content)) { if ([string]$content.type -eq 'text') { [string]$content.text } }
        return ($parts -join '')
    }
    return ''
}
function Stop-DiagnosticProcess {
    if ($null -eq $process) { return }
    try { if (-not $process.HasExited) { $process.StandardInput.Close() } } catch {}
    try {
        if ($process.WaitForExit(3000)) { $result.stop_mechanism = 'stdin close plus Windows Process handle WaitForExit' }
        else { try { $process.Kill() } catch {}; $process.WaitForExit(); $result.stop_mechanism = 'Windows Process handle Kill plus WaitForExit' }
        $result.stop_confirmed = $process.HasExited
        if ($result.stop_confirmed) { $result.exit_code = $process.ExitCode }
    } catch { $result.stop_confirmed = $false; $result.stop_mechanism = 'Windows Process handle wait failed' }
    $result.process_stop = [DateTime]::UtcNow.ToString('o')
}
function Write-ProviderEvents {
    $lines = foreach ($event in $events) { $event | ConvertTo-Json -Compress -Depth 30 }
    [IO.File]::WriteAllText((Join-Path $Evidence 'provider-events.jsonl'), (($lines -join [Environment]::NewLine) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

try {
    if (Test-Path -LiteralPath (Join-Path $Evidence 'windows-live-run.json')) { throw 'T21A runner evidence already exists; refusing retry/reset' }
    if (Test-Path -LiteralPath (Join-Path $Evidence 'allowance.json')) { throw 'T21 allowance already exists; refusing retry/reset' }
    $preflight = Get-Content -Raw (Join-Path $SourceEvidence 'preflight.json') | ConvertFrom-Json
    $execution = Get-Content -Raw (Join-Path $SourceEvidence 'execution-config.json') | ConvertFrom-Json
    $toolFile = Get-Content -Raw (Join-Path $SourceEvidence 'tool-surface.json') | ConvertFrom-Json
    . (Join-Path $PSScriptRoot 'r03a-t21a-tool-materialization.ps1')
    $toolRegistryCanonical = (Get-Content -Raw (Join-Path $SourceEvidence 'tool-registry.json')).TrimEnd("`r", "`n")
    $tools = Get-CanonicalToolRecordsFromJson $toolRegistryCanonical
    if (-not $preflight.passed -or [string]$preflight.status -ne 'offline_preflight_passed') { throw 'T21 offline preflight did not pass' }
    if ([string]$execution.schema_version -ne 't21-effective-execution-config-v1' -or [string]$execution.prompt_semantics -ne 'tiny_no_tools_sentinel') { throw 'T21 execution config schema or prompt semantics mismatch' }
    $materialization = Assert-CanonicalToolRecords $tools $toolFile.surface $toolRegistryCanonical $execution.tool_manifest_digest
    if ($materialization.count -ne 11 -or [int]$toolFile.surface.tool_count -ne 11 -or [int]$execution.tool_surface.tool_count -ne 11) { throw 'T21 exact 11-tool surface is unavailable' }
    if ((Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($toolRegistryCanonical))) -ne [string]$execution.tool_manifest_digest) { throw 'T21 formal tool registry digest drifted' }
    $toolNames = @($tools | ForEach-Object { [string]$_.name })
    $expectedNames = @($execution.tool_names | ForEach-Object { [string]$_ })
    if (($toolNames -join "`n") -ne ($expectedNames -join "`n")) { throw 'T21 exact tool order or names drifted' }
    $result.registered_tool_count = $tools.Count
    $result.tool_manifest_digest = [string]$execution.tool_manifest_digest
    $result.aggregate_schema_digest = [string]$execution.aggregate_schema_digest
    $result.aggregate_schema_bytes = [int]$execution.aggregate_schema_bytes
    $result.materialization_validation = 'passed'
    $result.binding_metadata_count = $materialization.binding_metadata.Count
    if (-not (Test-Path -LiteralPath $Binary -PathType Leaf) -or -not (Test-Path -LiteralPath $CodeModeHost -PathType Leaf) -or -not (Test-Path -LiteralPath $AuthSource -PathType Leaf) -or -not (Test-Path -LiteralPath $SelectedLinuxConfig -PathType Leaf)) { throw 'T21 controlled Windows input is unavailable' }
    if ((Hash-File $Binary) -ne [string]$execution.canonical_manifest.base_manifest.combination.binary_sha256) { throw 'T21 Windows binary drifted after preflight' }
    if ((Hash-File $CodeModeHost) -ne [string]$execution.canonical_manifest.base_manifest.combination.code_mode_host_sha256) { throw 'T21 code-mode-host drifted after preflight' }
    if ((Hash-File $SelectedLinuxConfig) -ne [string]$execution.selected_config_raw_sha256) { throw 'T21 selected config drifted after preflight' }
    if ((Hash-File $AuthSource) -ne [string]$execution.auth_credential_revision_fingerprint) { throw 'T21 auth credential revision drifted before process start' }

    New-Item -ItemType Directory -Force -Path $Evidence, $diagnosticHome, $workspace | Out-Null
    Copy-Item -LiteralPath $SelectedLinuxConfig -Destination (Join-Path $diagnosticHome 'config.toml')
    Copy-Item -LiteralPath $AuthSource -Destination (Join-Path $diagnosticHome 'auth.json')
    Set-ItemProperty -LiteralPath (Join-Path $diagnosticHome 'auth.json') -Name IsReadOnly -Value $true
    if ((Hash-File (Join-Path $diagnosticHome 'auth.json')) -ne [string]$execution.auth_credential_revision_fingerprint) { throw 'T21 auth snapshot differs from preflight material' }

    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $Binary
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.WorkingDirectory = $workspace
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    if ($null -ne $psi.PSObject.Properties['ArgumentList']) { $psi.ArgumentList.Add('app-server'); $psi.ArgumentList.Add('--stdio') } else { $psi.Arguments = 'app-server --stdio' }
    $environment = $psi.EnvironmentVariables
    $environment['HOME'] = $diagnosticHome
    $environment['CODEX_HOME'] = $diagnosticHome
    $environment['PATH'] = "$env:WINDIR\System32;$env:WINDIR"
    foreach ($name in $proxyNames) { $null = $environment.Remove($name) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $psi
    if (-not $process.Start()) { throw 'Windows native process did not start' }
    $result.process_started = $true
    $result.process_start = [DateTime]::UtcNow.ToString('o')
    $stderrTask = $process.StandardError.ReadToEndAsync()

    $null = Send-Frame ([ordered]@{ id = 1; method = 'initialize'; params = [ordered]@{ clientInfo = [ordered]@{ name = 'polis-t21'; version = '0.1.0' }; capabilities = [ordered]@{ experimentalApi = $true } } })
    $initialize = Wait-ForResponse 1 10000
    if ($null -eq $initialize.result.userAgent -or -not ([string]$initialize.result.userAgent -match '0\.153\.4')) { throw 'Windows native initialize version mismatch' }
    $result.initialize_completed = $true
    $null = Send-Frame ([ordered]@{ method = 'initialized' })
    $threadParams = [ordered]@{ model = 'gpt-5.6-luna'; allowProviderModelFallback = $false; approvalPolicy = 'never'; sandbox = 'read-only'; cwd = $workspace; environments = @(); ephemeral = $true; dynamicTools = $tools; config = [ordered]@{ model_reasoning_effort = 'medium' }; developerInstructions = [string]$execution.thread_start_developer_instruction }
    $threadFrame = [ordered]@{ id = 2; method = 'thread/start'; params = $threadParams }
    $threadStartJSON = Send-Frame $threadFrame
    $result.thread_start_payload_bytes = [Text.Encoding]::UTF8.GetByteCount($threadStartJSON)
    $result.thread_start_payload_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($threadStartJSON))
    [IO.File]::WriteAllText((Join-Path $Evidence 'thread-start-request.json'), $threadStartJSON + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    $threadResponse = Wait-ForResponse 2 20000
    if ([string]$threadResponse.result.model -ne 'gpt-5.6-luna' -or [string]$threadResponse.result.reasoningEffort -ne 'medium' -or [string]$threadResponse.result.approvalPolicy -ne 'never' -or [string]$threadResponse.result.sandbox.type -ne 'readOnly') { throw 'Windows native thread/start effective profile mismatch' }
    $threadId = [string]$threadResponse.result.thread.id
    if ([string]::IsNullOrEmpty($threadId)) { throw 'Windows native thread/start did not return a thread id' }
    $result.thread_started = $true
    $result.thread_id = $threadId
    $result.preflight_passed = $true
    if ((Hash-File $AuthSource) -ne [string]$execution.auth_credential_revision_fingerprint) { throw 'T21 auth credential revision drifted before provider call' }
    if ($PreflightOnly) {
        $result.status = 'preflight_passed'
        $result.provider_egress = 0
        $result.medium_started = 0
        return
    }
    $allowance = [ordered]@{ schema_version = 't21-diagnostic-allowance-v1'; qualification = $Qualification; authorized_model = 'gpt-5.6-luna'; authorized_effort = 'medium'; medium_authorized = 1; high_authorized = 0; concurrency = 1; retry = $false; reset = $false; medium_started = 1; provider_egress = 1; t20_control_fingerprint = [string]$execution.t20_control_fingerprint; t21_fingerprint = [string]$preflight.t21_fingerprint; created_at = [DateTime]::UtcNow.ToString('o') }
    Write-JsonFile (Join-Path $Evidence 'allowance.json') $allowance
    $result.medium_started = 1
    $result.provider_egress = 1
    $result.provider_or_internet_accessed = $true
    $turnClock = [Diagnostics.Stopwatch]::StartNew()
    $turnFrame = [ordered]@{ id = 3; method = 'turn/start'; params = [ordered]@{ threadId = $threadId; model = 'gpt-5.6-luna'; effort = 'medium'; input = @([ordered]@{ type = 'text'; text = $sentinelPrompt }) } }
    $null = Send-Frame $turnFrame

    $firstOutputDeadlineMs = 90000
    $totalDeadlineMs = 600000
    while ($turnClock.ElapsedMilliseconds -lt $totalDeadlineMs) {
        $remainingTotal = $totalDeadlineMs - [int]$turnClock.ElapsedMilliseconds
        $remainingFirst = if ($result.first_valid_output) { $remainingTotal } else { $firstOutputDeadlineMs - [int]$turnClock.ElapsedMilliseconds }
        if ($remainingFirst -le 0) { break }
        $frame = Read-Frame ([Math]::Max(1, [Math]::Min($remainingTotal, $remainingFirst)))
        if (-not $frame.ok) {
            if ($frame.error -eq 'read_timeout') { break }
            $result.failure_mode = 'stdio_failure'; $result.terminal_state = $frame.error; $result.unresolved_transport_state = $true; break
        }
        Add-ProtocolEvent 'receive' $frame.message $frame.line_ending
        $method = [string]$frame.message.method
        if ($method -eq 'turn/started') { $result.turn_started = $true }
        if ($method -eq 'item/started' -and [string]$frame.message.params.item.type -eq 'userMessage') { $result.user_message_item = $true }
        if ($method -eq 'item/tool/call') {
            $toolName = [string]$frame.message.params.tool
            $result.attempted_tool_calls += $toolName
            $denial = [ordered]@{ contentItems = @([ordered]@{ type = 'inputText'; text = '{"error":"diagnostic_tool_call_denied"}' }); success = $false }
            $null = Send-Frame ([ordered]@{ id = $frame.message.id; result = $denial })
            continue
        }
        $isOutputDelta = $method -eq 'item/agentMessage/delta'
        $isOutputCompleted = $method -eq 'item/agentMessage/completed' -or ($method -eq 'item/completed' -and [string]$frame.message.params.item.type -eq 'agentMessage')
        if ($isOutputDelta -or $isOutputCompleted) {
            $text = Get-MessageText $frame.message
            if ($isOutputDelta -and -not [string]::IsNullOrEmpty($text)) { $null = $outputParts.Add($text) }
            if ($isOutputCompleted -and $outputParts.Count -eq 0 -and -not [string]::IsNullOrEmpty($text)) { $null = $outputParts.Add($text) }
            if (-not $result.first_valid_output) { $result.first_valid_output = $true; $result.time_to_first_valid_output_ms = [int64]$turnClock.ElapsedMilliseconds; $result.first_valid_output_at = [DateTime]::UtcNow.ToString('o') }
            if ($reconnectOpen) { $reconnectOpen = $false; $result.recovery_timestamps += [DateTime]::UtcNow.ToString('o'); $result.recovery_delta_ms = [int64]$turnClock.ElapsedMilliseconds - [int64]$result.first_disconnect_delta_ms }
        }
        if ($method -match 'tokenUsage|usage') {
            $result.native_usage_updates = [int]$result.native_usage_updates + 1
            if ($null -ne $frame.message.params.tokenUsage) { $result.token_usage = $frame.message.params.tokenUsage } elseif ($null -ne $frame.message.params.usage) { $result.token_usage = $frame.message.params.usage }
        }
        if ($method -eq 'error') {
            $isReconnect = ($true -eq [bool]$frame.message.params.willRetry) -and ($null -ne $frame.message.params.error.codexErrorInfo.responseStreamDisconnected)
            if ($isReconnect) { $result.reconnect_count = [int]$result.reconnect_count + 1; $phase = if ($result.first_valid_output) { 'post_first_output' } else { 'pre_first_output' }; $result.reconnect_phases += $phase; if ($result.reconnect_count -eq 1) { $result.first_disconnect_delta_ms = [int64]$turnClock.ElapsedMilliseconds }; $reconnectOpen = $true; continue }
            $result.failure_mode = 'structured_provider_error'; $result.terminal_state = 'structured_provider_error'; $result.unresolved_transport_state = $true; break
        }
        if ($method -eq 'turn/completed') { $result.turn_completed = $true; $result.turn_state = if ($null -ne $frame.message.params.turn.status) { [string]$frame.message.params.turn.status } else { 'completed' }; $result.terminal_state = $result.turn_state; break }
    }
    $result.first_valid_output_text = ($outputParts -join '')
    if ($result.first_valid_output) { $result.sentinel_match = if ($result.first_valid_output_text.Trim() -eq $sentinel) { 'exact' } else { 'mismatch' } }
    if (-not $result.turn_completed -and [string]::IsNullOrEmpty($result.failure_mode)) {
        $result.unresolved_transport_state = $true
        if ($result.reconnect_count -gt 0 -and -not $result.first_valid_output) { $result.failure_mode = 'no_output_reconnect'; $result.turn_state = 'first_valid_output_deadline_exceeded'; $result.terminal_state = 'first_valid_output_deadline_exceeded' }
        elseif (-not $result.first_valid_output) { $result.failure_mode = 'turn_incomplete'; $result.turn_state = 'first_valid_output_deadline_exceeded'; $result.terminal_state = 'first_valid_output_deadline_exceeded' }
        else { $result.failure_mode = 'turn_incomplete'; $result.terminal_state = 'turn_completed_not_observed' }
    }
    if ($result.turn_completed -and $result.first_valid_output -and -not $result.unresolved_transport_state) { $result.status = 'completed' } else { $result.status = 'inconclusive' }
} catch {
    $result.error = $_.Exception.Message
    if ($result.provider_egress -gt 0) { if ([string]::IsNullOrEmpty($result.failure_mode)) { $result.failure_mode = 'stdio_failure' }; $result.status = 'inconclusive' }
    else { $result.status = 'preflight_failed'; $result.failure_mode = 'preflight_failed' }
} finally {
    Stop-DiagnosticProcess
    if ($null -ne $stderrTask) { try { $stderr = $stderrTask.GetAwaiter().GetResult(); $result.stderr_bytes = [Text.Encoding]::UTF8.GetByteCount($stderr); $result.stderr_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($stderr)) } catch {} }
    $result.stdio_output_line_endings = @($events | Where-Object { $_.direction -eq 'receive' } | Select-Object -ExpandProperty line_ending -Unique)
    $result.protocol_event_count = $events.Count
    $result.sentinel = $sentinel
    $result.prompt_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($sentinelPrompt))
    $result.developer_instruction_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes([string]$execution.thread_start_developer_instruction))
    try { New-Item -ItemType Directory -Force -Path $Evidence | Out-Null; Write-ProviderEvents } catch { $result.provider_events_write_error = $_.Exception.Message }
    if (Test-Path -LiteralPath $diagnosticRoot) { Remove-Item -LiteralPath $diagnosticRoot -Recurse -Force }
    Write-JsonFile (Join-Path $Evidence 'windows-live-run.json') $result
}

if ($result.status -eq 'preflight_failed') { exit 1 }
