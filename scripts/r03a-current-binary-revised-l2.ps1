# pattern: Imperative Shell
param(
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2-live',
    [string]$OfflineEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$RuntimeManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [int]$ExpectedToolCount = 11,
    [string]$Qualification = 'R0.3A-CURRENT-BINARY-REVISED-11-TOOL-L2',
    [string]$SurfaceRole = 'peer_backend',
    [string]$TerminalLedger = ''
)

$ErrorActionPreference = 'Stop'
$model = 'gpt-5.6-luna'
$effort = 'medium'
$sentinelPrompt = "Do not call tools.`nReply only with the transport sentinel."
$sentinel = 'POLIS_TRANSPORT_CANARY_OK'
$proxyNames = @('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY', 'POLIS_NATIVE_PROXY')
$expectedManifestSHA256 = '12d8b4813c999a1c1343adcdaea893b03e38b2b6a6a3689c312bea8044346abb'
$firstOutputDeadlineMs = 90000
$totalDeadlineMs = 600000

$liveSchemaVersion = if ($SurfaceRole -eq 'peer_frontend') { 'r03a-current-binary-revised-frontend-l2-windows-live-v1' } else { 'r03a-current-binary-revised-l2-windows-live-v1' }
$result = [ordered]@{
    schema_version = $liveSchemaVersion
    qualification = $Qualification
    surface_role = $SurfaceRole
    status = 'not_started'
    diagnostic_profile = 'windows_native_isolated'
    offline_evidence = $OfflineEvidence
    l1_evidence = $L1Evidence
    controlled_runtime_manifest = $RuntimeManifest
    controlled_runtime_manifest_digest = ''
    l1_fingerprint = ''
    execution_fingerprint = ''
    codex_version = ''
    codex_binary_sha256 = ''
    code_mode_host_sha256 = ''
    binary_path = ''
    code_mode_host_path = ''
    auth_source_class = 'controlled_diagnostic_auth_material'
    auth_identity_fingerprint = ''
    auth_credential_revision_fingerprint = ''
    effective_config_digest = ''
    effective_transport_config_digest = ''
    provider_transport_policy = 'native_default'
    websocket_policy = 'native_default'
    proxy_policy = 'no_injected_proxy'
    invocation = 'app-server --stdio'
    runtime_profile = 'windows-native'
    sandbox = 'read-only'
    model = $model
    effort = $effort
    dynamic_tool_count = $ExpectedToolCount
    registered_tool_count = 0
    tool_names = @()
    tool_manifest_digest = ''
    aggregate_schema_digest = ''
    aggregate_schema_bytes = 0
    handler_binding_digest = ''
    policy_revisions = $null
    normalized_thread_start_payload_digest = ''
    normalized_thread_start_payload_bytes = 0
    actual_thread_start_request_digest = ''
    actual_thread_start_request_bytes = 0
    exact_surface_preflight_passed = $false
    current_binary_l1_preflight_passed = $false
    final_exact_revalidation_passed = $false
    process_started = $false
    initialize_completed = $false
    thread_started = $false
    turn_started = $false
    user_message_item = $false
    first_valid_output = $false
    first_valid_output_text = ''
    time_to_first_valid_output_ms = 0
    reconnect_count = 0
    reconnect_phases = @()
    recovery_timestamps = @()
    native_usage_updates = 0
    token_usage = $null
    attempted_tool_calls = @()
    diagnostic_tool_denials = 0
    business_side_effects = 0
    turn_completed = $false
    turn_state = ''
    sentinel_match = 'not_run'
    unresolved_transport_state = $false
    failure_mode = ''
    terminal_state = ''
    allowance_created = $false
    medium_started = 0
    provider_egress = 0
    stop_confirmed = $false
    stop_mechanism = ''
    process_start = ''
    process_stop = ''
    exit_code = $null
    stderr_bytes = 0
    stderr_digest = ''
    protocol_event_count = 0
    stdio_output_line_endings = @()
    no_fallback_to_appdata_latest = $true
    historical_evidence_modified = $false
    error = ''
    current_frontend_revised_l2 = 'NOT_STARTED'
    eligible_for_revised_frontend_initial_live = $false
}

$process = $null
$stderrTask = $null
$diagnosticRoot = Join-Path ([IO.Path]::GetTempPath()) ('polis-r03a-current-l2-' + [guid]::NewGuid().ToString('N'))
$diagnosticHome = Join-Path $diagnosticRoot 'codex-home'
$workspace = Join-Path $diagnosticRoot 'workspace'
$events = New-Object System.Collections.Generic.List[object]
$outputParts = New-Object System.Collections.Generic.List[string]
$evidenceReady = $false
$preflightWritten = $false
$allowance = $null
$qualificationRunKey = "$Qualification|$SurfaceRole|$OfflineEvidence"

function Hash-Bytes([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Hash-File([string]$Path) { return Hash-Bytes ([IO.File]::ReadAllBytes($Path)) }

function Write-JsonFile([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Compress -Depth 60
    [IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

function Add-ProtocolEvent($Direction, $Message, $LineEnding) {
    $entry = [ordered]@{ timestamp = [DateTime]::UtcNow.ToString('o'); direction = $Direction; line_ending = $LineEnding }
    if ($null -ne $Message.id) { $entry.id = [string]$Message.id }
    if ($null -ne $Message.method) { $entry.method = [string]$Message.method }
    if ([string]$Message.method -eq 'item/tool/call') {
        $entry.tool_call = $true
        $entry.tool = if ($null -ne $Message.params.tool) { [string]$Message.params.tool } else { [string]$Message.params.name }
        $entry.call_id = if ($null -ne $Message.params.callId) { [string]$Message.params.callId } else { [string]$Message.id }
    }
    if ([string]$Message.method -eq 'error') {
        $entry.error_class = if ($null -ne $Message.params.error.codexErrorInfo.responseStreamDisconnected) { 'response_stream_disconnected' } else { 'structured_error' }
        $entry.will_retry = [bool]$Message.params.willRetry
    }
    if ([string]$Message.method -match '^item/agentMessage/') { $entry.text = Get-MessageText $Message }
    if ([string]$Message.method -match 'tokenUsage|usage') { $entry.usage_present = $true }
    if ([string]$Message.method -eq 'turn/completed' -and $null -ne $Message.params.turn.status) { $entry.turn_status = [string]$Message.params.turn.status }
    $null = $events.Add([pscustomobject]$entry)
}

function Send-Frame($Message) {
    $json = $Message | ConvertTo-Json -Compress -Depth 60
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
        try {
            $task = $stream.ReadAsync($buffer, 0, 1)
            $remaining = [Math]::Max(1, [int]($deadline.Subtract([DateTime]::UtcNow).TotalMilliseconds))
            if (-not $task.Wait($remaining)) { return [pscustomobject]@{ ok = $false; error = 'read_timeout' } }
            if ($task.Result -eq 0) { return [pscustomobject]@{ ok = $false; error = 'eof' } }
        } catch {
            return [pscustomobject]@{ ok = $false; error = 'read_error' }
        }
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
    $parts = New-Object System.Collections.Generic.List[string]
    if ($null -ne $Message.params.delta) { $null = $parts.Add([string]$Message.params.delta) }
    if ($null -ne $Message.params.text) { $null = $parts.Add([string]$Message.params.text) }
    if ($null -ne $Message.params.item.text) { $null = $parts.Add([string]$Message.params.item.text) }
    if ($null -ne $Message.params.item.content) {
        foreach ($content in @($Message.params.item.content)) { if ([string]$content.type -eq 'text' -and $null -ne $content.text) { $null = $parts.Add([string]$content.text) } }
    }
    return ($parts -join '')
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
    $lines = foreach ($event in $events) { $event | ConvertTo-Json -Compress -Depth 40 }
    [IO.File]::WriteAllText((Join-Path $Evidence 'provider-events.jsonl'), (($lines -join [Environment]::NewLine) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

try {
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -gt 0) { throw 'current-binary revised L2 evidence path is not fresh; refusing retry/reset' }
    }
    if (-not [string]::IsNullOrWhiteSpace($TerminalLedger) -and (Test-Path -LiteralPath $TerminalLedger -PathType Leaf)) {
        $ledger = Get-Content -Raw -LiteralPath $TerminalLedger | ConvertFrom-Json
        if ([string]$ledger.qualification_key -eq $qualificationRunKey -and [string]$ledger.terminal_status -in @('passed', 'inconclusive')) {
            throw 'terminal live result already exists for this qualification run; refusing second provider reservation/egress'
        }
    }
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    $evidenceReady = $true

    $l1 = Get-Content -Raw -LiteralPath (Join-Path $L1Evidence 'result.json') | ConvertFrom-Json
    $offline = Get-Content -Raw -LiteralPath (Join-Path $OfflineEvidence 'offline-result.json') | ConvertFrom-Json
    $offlineManifest = Get-Content -Raw -LiteralPath (Join-Path $OfflineEvidence 'execution-manifest.json') | ConvertFrom-Json
    $offlinePreflight = Get-Content -Raw -LiteralPath (Join-Path $OfflineEvidence 'preflight.json') | ConvertFrom-Json
    $toolFile = Get-Content -Raw -LiteralPath (Join-Path $OfflineEvidence 'tool-surface.json') | ConvertFrom-Json
    . (Join-Path $PSScriptRoot 'r03a-t21a-tool-materialization.ps1')
    $toolRegistryCanonical = (Get-Content -Raw -LiteralPath (Join-Path $OfflineEvidence 'tool-registry.json')).TrimEnd("`r", "`n")
    $tools = Get-CanonicalToolRecordsFromJson $toolRegistryCanonical
    $materialization = Assert-CanonicalToolRecords $tools $toolFile.surface $toolRegistryCanonical ([string]$offline.tool_manifest_digest)
    if ($l1.status -ne 'passed') { throw "current-binary L1 qualification status is not passed: $($l1.status)" }
    if ([string]$l1.current_binary_windows_l1 -ne 'QUALIFIED') { throw "current-binary L1 qualification state is not QUALIFIED: $($l1.current_binary_windows_l1)" }
    if ($l1.eligible_for_revised_l2 -ne $true) { throw "current-binary L1 is not eligible for revised L2: $($l1.eligible_for_revised_l2)" }
    if ([string]$offline.qualification -ne $Qualification) { throw "offline qualification identity mismatch: $($offline.qualification)" }
    if ([string]$offline.status -ne 'PASSED') { throw "offline qualification status is not PASSED: $($offline.status)" }
    if ([string]$offline.current_binary_l1 -ne 'QUALIFIED') { throw "offline L1 state is not QUALIFIED: $($offline.current_binary_l1)" }
    if ($offline.unrelated_drift -ne $false) { throw "offline surface has unrelated drift: $($offline.unrelated_drift)" }
    if ([string]$offlinePreflight.status -ne 'offline_preflight_passed') { throw "offline preflight status mismatch: $($offlinePreflight.status)" }
    if ($offlinePreflight.passed -ne $true) { throw "offline preflight did not pass: $($offlinePreflight.passed)" }
    if ($materialization.count -ne $ExpectedToolCount) { throw "materialized tool count mismatch: $($materialization.count), expected $ExpectedToolCount" }
    if ([int]$toolFile.surface.tool_count -ne $ExpectedToolCount) { throw "tool surface count mismatch: $($toolFile.surface.tool_count), expected $ExpectedToolCount" }
    if ([int]$offline.tool_count -ne $ExpectedToolCount) { throw "offline tool count mismatch: $($offline.tool_count), expected $ExpectedToolCount" }
    if ([string]$offline.tool_manifest_digest -ne [string]$toolFile.surface.aggregate_manifest_digest) { throw "offline/tool surface manifest mismatch: $($offline.tool_manifest_digest) vs $($toolFile.surface.aggregate_manifest_digest)" }
    if ([string]$offline.aggregate_schema_digest -ne [string]$toolFile.surface.aggregate_schema_digest) { throw "offline/tool surface schema digest mismatch: $($offline.aggregate_schema_digest) vs $($toolFile.surface.aggregate_schema_digest)" }
    if ([int]$offline.aggregate_schema_bytes -ne [int]$toolFile.surface.aggregate_schema_bytes) { throw "offline/tool surface schema bytes mismatch: $($offline.aggregate_schema_bytes) vs $($toolFile.surface.aggregate_schema_bytes)" }
    $result.registered_tool_count = $materialization.count
    $result.tool_names = @($materialization.names)
    $result.tool_manifest_digest = [string]$offline.tool_manifest_digest
    $result.aggregate_schema_digest = [string]$offline.aggregate_schema_digest
    $result.aggregate_schema_bytes = [int]$offline.aggregate_schema_bytes
    $result.handler_binding_digest = [string]$offline.handler_binding_digest
    $result.policy_revisions = $offline.policy_revisions
    $result.normalized_thread_start_payload_digest = [string]$offline.thread_start_payload_digest
    $result.normalized_thread_start_payload_bytes = [int]$offline.thread_start_payload_bytes
    $result.exact_surface_preflight_passed = $true
    $result.l1_fingerprint = [string]$offline.l1_fingerprint
    $result.execution_fingerprint = [string]$offline.execution_fingerprint
    $result.current_binary_l1_preflight_passed = $true

    if (-not (Test-Path -LiteralPath $RuntimeManifest -PathType Leaf)) { throw 'controlled runtime manifest is unavailable' }
    if ((Hash-File $RuntimeManifest) -ne $expectedManifestSHA256 -or [string]$offline.controlled_runtime_manifest_digest -ne $expectedManifestSHA256) { throw 'controlled runtime manifest drifted' }
    $runtime = Get-Content -Raw -LiteralPath $RuntimeManifest | ConvertFrom-Json
    if ([string]$runtime.source_class -ne 'windows_native_controlled_staged' -or [string]$runtime.codex_version -ne [string]$l1.codex_version -or [string]$runtime.codex_binary_sha256 -ne [string]$l1.codex_binary_sha256 -or [string]$runtime.code_mode_host_sha256 -ne [string]$l1.code_mode_host_sha256) { throw 'controlled runtime does not match qualified L1' }
    $binary = [string]$runtime.codex_binary_staged_path
    $helper = [string]$runtime.code_mode_host_staged_path
    if ($binary -ne [string]$l1.binary_path -or $helper -ne [string]$l1.code_mode_host_path) { throw 'current L2 did not use the qualified staged pair' }
    if (-not (Test-Path -LiteralPath $binary -PathType Leaf) -or -not (Test-Path -LiteralPath $helper -PathType Leaf)) { throw 'qualified staged binary/helper is unavailable' }
    if ((Hash-File $binary) -ne [string]$l1.codex_binary_sha256 -or (Hash-File $helper) -ne [string]$l1.code_mode_host_sha256) { throw 'qualified staged binary/helper hash drifted' }
    $observedVersion = (& $binary --version 2>&1 | Out-String).Trim()
    if ($observedVersion -ne [string]$l1.codex_version) { throw "qualified staged version drifted: $observedVersion" }
    $result.controlled_runtime_manifest_digest = [string]$offline.controlled_runtime_manifest_digest
    $result.codex_version = [string]$l1.codex_version
    $result.codex_binary_sha256 = [string]$l1.codex_binary_sha256
    $result.code_mode_host_sha256 = [string]$l1.code_mode_host_sha256
    $result.binary_path = $binary
    $result.code_mode_host_path = $helper

    if (-not (Test-Path -LiteralPath $AuthSource -PathType Leaf) -or -not (Test-Path -LiteralPath $SelectedConfig -PathType Leaf)) { throw 'current controlled auth/config is unavailable' }
    if ((Hash-File $AuthSource) -ne [string]$l1.auth_credential_revision_fingerprint -or (Hash-File $SelectedConfig) -ne [string]$l1.selected_config_raw_sha256) { throw 'current auth/config drifted from qualified L1' }
    $result.auth_source_class = [string]$l1.auth_source_class
    $result.auth_identity_fingerprint = [string]$l1.auth_identity_fingerprint
    $result.auth_credential_revision_fingerprint = [string]$l1.auth_credential_revision_fingerprint
    $result.effective_config_digest = [string]$l1.effective_config_digest
    $result.effective_transport_config_digest = [string]$l1.effective_transport_config_digest
    if ([string]$offlineManifest.canonical_manifest.base_manifest.auth.auth_identity_fingerprint -ne [string]$l1.auth_identity_fingerprint -or [string]$offlineManifest.canonical_manifest.base_manifest.auth.auth_credential_revision_fingerprint -ne [string]$l1.auth_credential_revision_fingerprint -or [string]$offlineManifest.canonical_manifest.base_manifest.effective_config_digest -ne [string]$l1.effective_config_digest -or [string]$offlineManifest.canonical_manifest.base_manifest.effective_transport_config_digest -ne [string]$l1.effective_transport_config_digest) { throw 'current L2 auth/config binding does not match qualified L1' }

    $preflight = [ordered]@{
        schema_version = 'r03a-current-binary-revised-l2-live-preflight-v1'
        qualification = $Qualification
        surface_role = $SurfaceRole
        status = 'live_preflight_passed'
        passed = $true
        current_binary_l1 = 'QUALIFIED'
        l1_fingerprint = $result.l1_fingerprint
        execution_fingerprint = $result.execution_fingerprint
        controlled_runtime_manifest_digest = $result.controlled_runtime_manifest_digest
        codex_version = $result.codex_version
        codex_binary_sha256 = $result.codex_binary_sha256
        code_mode_host_sha256 = $result.code_mode_host_sha256
        tool_count = $result.registered_tool_count
        tool_names = $result.tool_names
        tool_manifest_digest = $result.tool_manifest_digest
        aggregate_schema_digest = $result.aggregate_schema_digest
        aggregate_schema_bytes = $result.aggregate_schema_bytes
        handler_binding_digest = $result.handler_binding_digest
        policy_revisions = $result.policy_revisions
        auth_identity_fingerprint = $result.auth_identity_fingerprint
        auth_credential_revision_fingerprint = $result.auth_credential_revision_fingerprint
        effective_config_digest = $result.effective_config_digest
        effective_transport_config_digest = $result.effective_transport_config_digest
        provider_transport_policy = 'native_default'
        proxy_policy = 'no_injected_proxy'
        model = $model
        effort = $effort
        dynamic_tool_count = $ExpectedToolCount
        business_write_policy = 'diagnostic_denied'
        medium = 0
        high = 0
        provider_egress = 0
        no_fallback_to_appdata_latest = $true
        historical_evidence_modified = $false
        checked_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-JsonFile (Join-Path $Evidence 'live-preflight.json') $preflight
    $preflightWritten = $true

    New-Item -ItemType Directory -Force -Path $diagnosticHome, $workspace | Out-Null
    $diagnosticConfig = Join-Path $diagnosticHome 'config.toml'
    $diagnosticAuth = Join-Path $diagnosticHome 'auth.json'
    Copy-Item -LiteralPath $SelectedConfig -Destination $diagnosticConfig
    Copy-Item -LiteralPath $AuthSource -Destination $diagnosticAuth
    Set-ItemProperty -LiteralPath $diagnosticAuth -Name IsReadOnly -Value $true
    if ((Hash-File $diagnosticConfig) -ne [string]$l1.selected_config_raw_sha256 -or (Hash-File $diagnosticAuth) -ne [string]$l1.auth_credential_revision_fingerprint) { throw 'diagnostic auth/config snapshot differs from preflight' }

    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $binary
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
    if (-not $process.Start()) { throw 'current-binary revised L2 process did not start' }
    $result.process_started = $true
    $result.process_start = [DateTime]::UtcNow.ToString('o')
    $stderrTask = $process.StandardError.ReadToEndAsync()

    $null = Send-Frame ([ordered]@{ id = 1; method = 'initialize'; params = [ordered]@{ clientInfo = [ordered]@{ name = 'polis-current-binary-l2'; version = '0.1.0' }; capabilities = [ordered]@{ experimentalApi = $true } } })
    $initialize = Wait-ForResponse 1 10000
    if ([string]$initialize.result.userAgent -notmatch [regex]::Escape('0.154.0-alpha.6.2')) { throw 'current-binary revised L2 initialize version mismatch' }
    $result.initialize_completed = $true
    $result.observed_user_agent = [string]$initialize.result.userAgent
    $null = Send-Frame ([ordered]@{ method = 'initialized' })
    $payload = Get-Content -Raw -LiteralPath (Join-Path $OfflineEvidence 'execution-manifest.json') | ConvertFrom-Json
    $developerInstructions = [string]$payload.thread_start_payload_canonical_json | ConvertFrom-Json
    $developerText = [string]$developerInstructions.developerInstructions
    if ($developerText -ne $sentinelPrompt) { throw 'current L2 sentinel semantics drifted' }
    $threadParams = [ordered]@{ model = $model; allowProviderModelFallback = $false; approvalPolicy = 'never'; sandbox = 'read-only'; cwd = $workspace; environments = @(); ephemeral = $true; dynamicTools = $tools; config = [ordered]@{ model_reasoning_effort = $effort }; developerInstructions = $developerText }
    $threadFrame = [ordered]@{ id = 2; method = 'thread/start'; params = $threadParams }
    $threadStartJSON = Send-Frame $threadFrame
    $result.actual_thread_start_request_bytes = [Text.Encoding]::UTF8.GetByteCount($threadStartJSON)
    $result.actual_thread_start_request_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($threadStartJSON))
    [IO.File]::WriteAllText((Join-Path $Evidence 'thread-start-request.json'), $threadStartJSON + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    $threadResponse = Wait-ForResponse 2 20000
    if ([string]$threadResponse.result.model -ne $model -or [string]$threadResponse.result.reasoningEffort -ne $effort -or [string]$threadResponse.result.approvalPolicy -ne 'never' -or [string]$threadResponse.result.sandbox.type -ne 'readOnly') { throw 'current-binary revised L2 thread/start effective profile mismatch' }
    $threadId = [string]$threadResponse.result.thread.id
    if ([string]::IsNullOrEmpty($threadId)) { throw 'current-binary revised L2 thread/start did not return a thread id' }

    if ((Hash-File $RuntimeManifest) -ne $expectedManifestSHA256 -or (Hash-File $binary) -ne [string]$l1.codex_binary_sha256 -or (Hash-File $helper) -ne [string]$l1.code_mode_host_sha256 -or (Hash-File $AuthSource) -ne [string]$l1.auth_credential_revision_fingerprint -or (Hash-File $SelectedConfig) -ne [string]$l1.selected_config_raw_sha256) { throw 'final exact L2 revalidation drifted before provider call' }
    $result.final_exact_revalidation_passed = $true
    $allowance = [ordered]@{
        schema_version = 'r03a-current-binary-revised-l2-diagnostic-allowance-v1'
        qualification = $Qualification
        surface_role = $SurfaceRole
        authorized_model = $model
        authorized_effort = $effort
        medium_authorized = 1
        high_authorized = 0
        concurrency = 1
        retry = $false
        reset = $false
        tool_count = $ExpectedToolCount
        business_write_policy = 'diagnostic_denied'
        l1_fingerprint = $result.l1_fingerprint
        execution_fingerprint = $result.execution_fingerprint
        controlled_runtime_manifest_digest = $result.controlled_runtime_manifest_digest
        tool_manifest_digest = $result.tool_manifest_digest
        policy_revisions = $result.policy_revisions
        auth_identity_fingerprint = $result.auth_identity_fingerprint
        auth_credential_revision_fingerprint = $result.auth_credential_revision_fingerprint
        effective_config_digest = $result.effective_config_digest
        effective_transport_config_digest = $result.effective_transport_config_digest
        created_at = [DateTime]::UtcNow.ToString('o')
        medium_started = 0
        provider_egress = 0
    }
    Write-JsonFile (Join-Path $Evidence 'allowance.json') $allowance
    $result.allowance_created = $true

    $turnClock = [Diagnostics.Stopwatch]::StartNew()
    $turnFrame = [ordered]@{ id = 3; method = 'turn/start'; params = [ordered]@{ threadId = $threadId; model = $model; effort = $effort; input = @([ordered]@{ type = 'text'; text = $sentinel }) } }
    $null = Send-Frame $turnFrame
    $result.provider_egress = 1
    $allowance.provider_egress = 1
    Write-JsonFile (Join-Path $Evidence 'allowance.json') $allowance

    $reconnectOpen = $false
    $sawAgentDelta = $false
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
        if ($method -eq 'thread/started') { $result.thread_started = $true }
        if ($method -eq 'turn/started') {
            $result.turn_started = $true
            $result.medium_started = 1
            $allowance.medium_started = 1
            Write-JsonFile (Join-Path $Evidence 'allowance.json') $allowance
        }
        if ($method -eq 'item/started' -and [string]$frame.message.params.item.type -eq 'userMessage') { $result.user_message_item = $true }
        if ($method -eq 'item/tool/call') {
            $toolName = if ($null -ne $frame.message.params.tool) { [string]$frame.message.params.tool } else { [string]$frame.message.params.name }
            $result.attempted_tool_calls += $toolName
            $result.diagnostic_tool_denials = [int]$result.diagnostic_tool_denials + 1
            if ($null -ne $frame.message.id) {
                $denial = [ordered]@{ contentItems = @([ordered]@{ type = 'inputText'; text = '{"error":"diagnostic_tool_call_denied"}' }); success = $false }
                $null = Send-Frame ([ordered]@{ id = $frame.message.id; result = $denial })
            }
            continue
        }
        if ($method -eq 'item/agentMessage/delta') {
            $text = Get-MessageText $frame.message
            if (-not [string]::IsNullOrEmpty($text)) {
                if (-not $result.first_valid_output) { $result.first_valid_output = $true; $result.time_to_first_valid_output_ms = [int64]$turnClock.ElapsedMilliseconds; $result.first_valid_output_at = [DateTime]::UtcNow.ToString('o') }
                $null = $outputParts.Add($text)
                $sawAgentDelta = $true
            }
            if ($reconnectOpen) { $reconnectOpen = $false; $result.recovery_timestamps += [DateTime]::UtcNow.ToString('o') }
        }
        if ($method -eq 'item/agentMessage/completed' -and -not $sawAgentDelta) {
            $text = Get-MessageText $frame.message
            if (-not [string]::IsNullOrEmpty($text)) { $result.first_valid_output = $true; $result.time_to_first_valid_output_ms = [int64]$turnClock.ElapsedMilliseconds; $result.first_valid_output_at = [DateTime]::UtcNow.ToString('o'); $null = $outputParts.Add($text) }
        }
        if ($method -match 'tokenUsage|usage') {
            $result.native_usage_updates = [int]$result.native_usage_updates + 1
            if ($null -ne $frame.message.params.tokenUsage) { $result.token_usage = $frame.message.params.tokenUsage } elseif ($null -ne $frame.message.params.usage) { $result.token_usage = $frame.message.params.usage }
        }
        if ($method -eq 'error') {
            $isReconnect = ($true -eq [bool]$frame.message.params.willRetry) -and ($null -ne $frame.message.params.error.codexErrorInfo.responseStreamDisconnected)
            if ($isReconnect) { $result.reconnect_count = [int]$result.reconnect_count + 1; $phase = if ($result.first_valid_output) { 'post_first_output' } else { 'pre_first_output' }; $result.reconnect_phases += $phase; $reconnectOpen = $true; continue }
            $result.failure_mode = 'structured_provider_error'; $result.terminal_state = 'structured_provider_error'; $result.unresolved_transport_state = $true; break
        }
        if ($method -eq 'turn/completed') {
            $result.turn_completed = $true
            $result.turn_state = if ($null -ne $frame.message.params.turn.status) { [string]$frame.message.params.turn.status } else { 'completed' }
            $result.terminal_state = $result.turn_state
            break
        }
    }
    $result.first_valid_output_text = ($outputParts -join '')
    if ($result.first_valid_output) { $result.sentinel_match = if ($result.first_valid_output_text.Trim() -eq $sentinel) { 'exact' } else { 'mismatch' } }
    if (-not $result.turn_completed -and [string]::IsNullOrEmpty($result.failure_mode)) {
        $result.unresolved_transport_state = $true
        if ($result.reconnect_count -gt 0 -and -not $result.first_valid_output) { $result.failure_mode = 'no_output_reconnect'; $result.turn_state = 'first_valid_output_deadline_exceeded'; $result.terminal_state = 'first_valid_output_deadline_exceeded' }
        elseif (-not $result.first_valid_output) { $result.failure_mode = 'turn_incomplete'; $result.turn_state = 'first_valid_output_deadline_exceeded'; $result.terminal_state = 'first_valid_output_deadline_exceeded' }
        else { $result.failure_mode = 'turn_incomplete'; $result.terminal_state = 'turn_completed_not_observed' }
    }
    if ($result.turn_completed -and $result.first_valid_output -and -not $result.unresolved_transport_state) { $result.status = 'passed'; $result.current_binary_revised_11_tool_l2 = 'QUALIFIED'; $result.eligible_for_revised_backend_run = $true }
    else { $result.status = 'inconclusive'; $result.current_binary_revised_11_tool_l2 = 'UNQUALIFIED'; $result.eligible_for_revised_backend_run = $false }
} catch {
    $result.error = $_.Exception.Message
    if ($result.provider_egress -gt 0) { if ([string]::IsNullOrEmpty($result.failure_mode)) { $result.failure_mode = 'stdio_failure' }; $result.status = 'inconclusive'; $result.current_binary_revised_11_tool_l2 = 'UNQUALIFIED'; $result.eligible_for_revised_backend_run = $false }
    else { $result.status = 'not_started'; $result.failure_mode = 'preflight_failed'; $result.current_binary_revised_11_tool_l2 = 'NOT_STARTED'; $result.eligible_for_revised_backend_run = $false; $result.medium_started = 0; $result.provider_egress = 0 }
} finally {
    if ($SurfaceRole -eq 'peer_frontend') {
        if ($result.status -eq 'passed') { $result.current_frontend_revised_l2 = 'QUALIFIED'; $result.eligible_for_revised_frontend_initial_live = $true }
        elseif ($result.status -eq 'not_started') { $result.current_frontend_revised_l2 = 'NOT_STARTED'; $result.eligible_for_revised_frontend_initial_live = $false }
        else { $result.current_frontend_revised_l2 = 'UNQUALIFIED'; $result.eligible_for_revised_frontend_initial_live = $false }
    }
    Stop-DiagnosticProcess
    if ($null -ne $stderrTask) { try { $stderr = $stderrTask.GetAwaiter().GetResult(); $result.stderr_bytes = [Text.Encoding]::UTF8.GetByteCount($stderr); $result.stderr_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($stderr)) } catch {} }
    $result.stdio_output_line_endings = @($events | Where-Object { $_.direction -eq 'receive' } | Select-Object -ExpandProperty line_ending -Unique)
    $result.protocol_event_count = $events.Count
    $result.sentinel = $sentinel
    $result.developer_instruction_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($sentinelPrompt))
    $result.finished_at = [DateTime]::UtcNow.ToString('o')
    if ($evidenceReady) {
        if ($events.Count -gt 0) { try { Write-ProviderEvents } catch {} }
        try { Write-JsonFile (Join-Path $Evidence 'windows-live-run.json') $result } catch {}
        try { Write-JsonFile (Join-Path $Evidence 'result.json') $result } catch {}
        if (-not [string]::IsNullOrWhiteSpace($TerminalLedger) -and $result.status -in @('passed', 'inconclusive')) {
            try {
                Write-JsonFile $TerminalLedger ([ordered]@{ qualification_key = $qualificationRunKey; terminal_status = $result.status; result_path = (Join-Path $Evidence 'result.json'); provider_egress = $result.provider_egress; recorded_at = [DateTime]::UtcNow.ToString('o') })
            } catch {}
        }
    }
    if (Test-Path -LiteralPath $diagnosticRoot) { Remove-Item -LiteralPath $diagnosticRoot -Recurse -Force }
}

if ($result.status -eq 'not_started') { exit 1 }
if ($result.status -ne 'passed') { exit 2 }
