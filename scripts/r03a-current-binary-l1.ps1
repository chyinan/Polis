# pattern: Imperative Shell
param(
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$RuntimeManifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml',
    [string]$HistoricalExecutionConfig = 'D:\Programs\Polis\evidence\development\r0.3a-t20\execution-config.json'
)

$ErrorActionPreference = 'Stop'
$expectedCodexVersion = 'codex-cli 0.154.0-alpha.6.2'
$expectedCodexSHA256 = '081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575'
$expectedCodeModeHostSHA256 = 'fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e'
$expectedCodexSize = [int64]297858352
$expectedCodeModeHostSize = [int64]72471856
$expectedRuntimeManifestSHA256 = '12d8b4813c999a1c1343adcdaea893b03e38b2b6a6a3689c312bea8044346abb'
$expectedAuthSourceClass = 'controlled_diagnostic_auth_material'
$model = 'gpt-5.6-luna'
$effort = 'medium'
$sentinelPrompt = 'Reply with exactly: POLIS_TRANSPORT_CANARY_OK'
$sentinel = 'POLIS_TRANSPORT_CANARY_OK'
$proxyNames = @('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY', 'POLIS_NATIVE_PROXY')
$firstOutputDeadlineMs = 90000
$totalDeadlineMs = 600000

$result = [ordered]@{
    schema_version = 'r03a-current-binary-l1-windows-live-v1'
    qualification = 'R0.3A-CURRENT-BINARY-L1'
    status = 'not_started'
    diagnostic_profile = 'windows_native_isolated'
    controlled_runtime_manifest = $RuntimeManifest
    controlled_runtime_manifest_digest = ''
    codex_version = $expectedCodexVersion
    codex_binary_sha256 = $expectedCodexSHA256
    codex_binary_size = $expectedCodexSize
    code_mode_host_sha256 = $expectedCodeModeHostSHA256
    code_mode_host_size = $expectedCodeModeHostSize
    binary_path = ''
    code_mode_host_path = ''
    auth_source_class = $expectedAuthSourceClass
    auth_source_path = $AuthSource
    auth_identity_fingerprint = ''
    auth_credential_revision_fingerprint = ''
    effective_config_digest = ''
    effective_transport_config_digest = ''
    selected_config_path = $SelectedConfig
    selected_config_raw_sha256 = ''
    provider_transport_policy = 'native_default'
    websocket_policy = 'native_default'
    proxy_policy = 'no_injected_proxy'
    invocation = 'app-server --stdio'
    runtime_profile = 'windows-native'
    sandbox = 'read-only'
    dynamic_tool_count = 0
    zero_tool_surface = $true
    model = $model
    effort = $effort
    prompt_digest = ''
    execution_fingerprint = ''
    auth_preflight_passed = $false
    config_preflight_passed = $false
    controlled_runtime_preflight_passed = $false
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
    native_usage_updates = 0
    token_usage = $null
    turn_completed = $false
    turn_state = ''
    sentinel_match = 'not_run'
    unresolved_transport_state = $false
    failure_mode = ''
    terminal_state = ''
    allowance_created = $false
    medium_started = 0
    provider_egress = 0
    business_side_effects = 0
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
    next_step = 'do not automatically run revised exact-11-tool L2 or Backend'
    error = ''
}

$process = $null
$stderrTask = $null
$diagnosticRoot = Join-Path ([IO.Path]::GetTempPath()) ('polis-r03a-current-l1-' + [guid]::NewGuid().ToString('N'))
$diagnosticHome = Join-Path $diagnosticRoot 'codex-home'
$workspace = Join-Path $diagnosticRoot 'workspace'
$events = New-Object System.Collections.Generic.List[object]
$outputParts = New-Object System.Collections.Generic.List[string]
$evidenceReady = $false
$preflightWritten = $false
$allowance = $null

function Hash-Bytes([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function Hash-File([string]$Path) {
    return Hash-Bytes ([IO.File]::ReadAllBytes($Path))
}

function Write-JsonFile([string]$Path, $Value) {
    $json = $Value | ConvertTo-Json -Compress -Depth 40
    [IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

function Add-ProtocolEvent($Direction, $Message, $LineEnding) {
    $entry = [ordered]@{
        timestamp = [DateTime]::UtcNow.ToString('o')
        direction = $Direction
        line_ending = $LineEnding
    }
    if ($null -ne $Message.id) { $entry.id = [string]$Message.id }
    if ($null -ne $Message.method) { $entry.method = [string]$Message.method }
    if ($Direction -eq 'receive' -and [string]$Message.method -eq 'error') {
        $entry.error_class = if ($null -ne $Message.params.error.codexErrorInfo.responseStreamDisconnected) { 'response_stream_disconnected' } else { 'structured_error' }
        $entry.will_retry = [bool]$Message.params.willRetry
    }
    if ($Direction -eq 'receive' -and [string]$Message.method -match '^item/agentMessage/') {
        $entry.text = Get-MessageText $Message
    }
    if ($Direction -eq 'receive' -and [string]$Message.method -match 'tokenUsage|usage') {
        $entry.usage_present = $true
    }
    if ($Direction -eq 'receive' -and [string]$Message.method -eq 'turn/completed' -and $null -ne $Message.params.turn.status) {
        $entry.turn_status = [string]$Message.params.turn.status
    }
    $null = $events.Add([pscustomobject]$entry)
}

function Send-Frame($Message) {
    $json = $Message | ConvertTo-Json -Compress -Depth 30
    $process.StandardInput.Write($json + "`r`n")
    $process.StandardInput.Flush()
    Add-ProtocolEvent 'send' ($json | ConvertFrom-Json) 'CRLF'
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
            if ($line.Count -gt 0 -and $line[$line.Count - 1] -eq 13) {
                $line.RemoveAt($line.Count - 1)
                $lineEnding = 'CRLF'
            }
            $text = [Text.Encoding]::UTF8.GetString($line.ToArray())
            try {
                return [pscustomobject]@{ ok = $true; message = ($text | ConvertFrom-Json); line_ending = $lineEnding }
            } catch {
                return [pscustomobject]@{ ok = $false; error = 'invalid_json_frame'; line_ending = $lineEnding }
            }
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
        foreach ($content in @($Message.params.item.content)) {
            if ([string]$content.type -eq 'text' -and $null -ne $content.text) { $null = $parts.Add([string]$content.text) }
        }
    }
    return ($parts -join '')
}

function Assert-ControlledRuntime {
    if (-not (Test-Path -LiteralPath $RuntimeManifest -PathType Leaf)) { throw 'controlled runtime manifest is unavailable' }
    $manifestRaw = [IO.File]::ReadAllBytes($RuntimeManifest)
    $manifestDigest = Hash-Bytes $manifestRaw
    if ($manifestDigest -ne $expectedRuntimeManifestSHA256) { throw 'controlled runtime manifest digest drifted' }
    $manifest = [Text.Encoding]::UTF8.GetString($manifestRaw) | ConvertFrom-Json
    if ([string]$manifest.schema_version -ne 'windows-runtime-artifact-v1') { throw 'controlled runtime manifest schema mismatch' }
    if ([string]$manifest.codex_version -ne $expectedCodexVersion) { throw 'controlled runtime version mismatch' }
    if ([string]$manifest.codex_binary_sha256 -ne $expectedCodexSHA256 -or [int64]$manifest.codex_binary_size -ne $expectedCodexSize) { throw 'controlled runtime codex binding mismatch' }
    if ([string]$manifest.code_mode_host_sha256 -ne $expectedCodeModeHostSHA256 -or [int64]$manifest.code_mode_host_size -ne $expectedCodeModeHostSize) { throw 'controlled runtime helper binding mismatch' }
    if ([string]$manifest.historical_l1_binary_equivalence -eq 'true') { throw 'current runtime incorrectly claims historical binary equivalence' }

    $binary = [string]$manifest.codex_binary_staged_path
    $helper = [string]$manifest.code_mode_host_staged_path
    if ([string]::IsNullOrWhiteSpace($binary) -or [string]::IsNullOrWhiteSpace($helper)) { throw 'controlled staged paths are missing' }
    if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw 'controlled staged codex binary is unavailable' }
    if (-not (Test-Path -LiteralPath $helper -PathType Leaf)) { throw 'controlled staged code-mode-host is unavailable' }
    if ((Hash-File $binary) -ne $expectedCodexSHA256) { throw 'controlled staged codex hash drifted' }
    if ((Hash-File $helper) -ne $expectedCodeModeHostSHA256) { throw 'controlled staged helper hash drifted' }
    if ([int64](Get-Item -LiteralPath $binary).Length -ne $expectedCodexSize) { throw 'controlled staged codex size drifted' }
    if ([int64](Get-Item -LiteralPath $helper).Length -ne $expectedCodeModeHostSize) { throw 'controlled staged helper size drifted' }
    $observedVersion = (& $binary --version 2>&1 | Out-String).Trim()
    if ($observedVersion -ne $expectedCodexVersion) { throw "controlled staged codex version mismatch: $observedVersion" }
    return [pscustomobject]@{ manifest = $manifest; manifest_digest = $manifestDigest; binary = $binary; helper = $helper; observed_version = $observedVersion }
}

function Read-HistoricalAuthConfigReference {
    if (-not (Test-Path -LiteralPath $HistoricalExecutionConfig -PathType Leaf)) { throw 'historical controlled auth/config reference is unavailable' }
    $reference = Get-Content -Raw -LiteralPath $HistoricalExecutionConfig | ConvertFrom-Json
    if ([string]$reference.schema_version -ne 't20-effective-execution-config-v1') { throw 'historical auth/config reference schema mismatch' }
    if ([string]$reference.canonical_manifest.combination.codex_version -ne '0.153.4') { throw 'historical reference binary was not the frozen 0.153.4 control' }
    if ([string]$reference.canonical_manifest.combination.model -ne $model -or [string]$reference.canonical_manifest.combination.effort -ne $effort) { throw 'historical auth/config reference model or effort mismatch' }
    if ([string]$reference.canonical_manifest.combination.runtime_profile -ne 'windows-native' -or [string]$reference.canonical_manifest.combination.sandbox_class -ne 'read-only') { throw 'historical auth/config reference runtime mismatch' }
    if ([string]$reference.canonical_manifest.transport.provider_transport_policy -ne 'native_default' -or [string]$reference.canonical_manifest.transport.websocket_policy -ne 'native_default') { throw 'historical transport policy reference mismatch' }
    if ([string]$reference.canonical_manifest.auth.auth_source_class -ne $expectedAuthSourceClass) { throw 'historical auth source class mismatch' }
    return $reference
}

function Assert-AuthConfig($Reference) {
    if (-not (Test-Path -LiteralPath $AuthSource -PathType Leaf)) { throw 'controlled auth source is unavailable' }
    if (-not (Test-Path -LiteralPath $SelectedConfig -PathType Leaf)) { throw 'selected diagnostic config is unavailable' }
    $authRevision = Hash-File $AuthSource
    $expectedAuthRevision = [string]$Reference.canonical_manifest.auth.auth_credential_revision_fingerprint
    if ($authRevision -ne $expectedAuthRevision) { throw 'auth credential revision drifted from controlled boundary' }
    $configHash = Hash-File $SelectedConfig
    $expectedConfigHash = [string]$Reference.selected_config_raw_sha256
    if ($configHash -ne $expectedConfigHash) { throw 'selected diagnostic config drifted from controlled boundary' }
    return [pscustomobject]@{
        auth_identity = [string]$Reference.canonical_manifest.auth.auth_identity_fingerprint
        auth_revision = $authRevision
        config_sha256 = $configHash
        effective_config_digest = [string]$Reference.canonical_manifest.effective_config_digest
        effective_transport_config_digest = [string]$Reference.canonical_manifest.effective_transport_config_digest
    }
}

function Stop-DiagnosticProcess {
    if ($null -eq $process) { return }
    try {
        if (-not $process.HasExited) { $process.StandardInput.Close() }
    } catch {}
    try {
        if ($process.WaitForExit(3000)) {
            $result.stop_mechanism = 'stdin close plus Windows Process handle WaitForExit'
        } else {
            try { $process.Kill() } catch {}
            $process.WaitForExit()
            $result.stop_mechanism = 'Windows Process handle Kill plus WaitForExit'
        }
        $result.stop_confirmed = $process.HasExited
        if ($result.stop_confirmed) { $result.exit_code = $process.ExitCode }
    } catch {
        $result.stop_confirmed = $false
        $result.stop_mechanism = 'Windows Process handle wait failed'
    }
    $result.process_stop = [DateTime]::UtcNow.ToString('o')
}

function Write-ProviderEvents {
    $lines = foreach ($event in $events) { $event | ConvertTo-Json -Compress -Depth 30 }
    [IO.File]::WriteAllText((Join-Path $Evidence 'provider-events.jsonl'), (($lines -join [Environment]::NewLine) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

try {
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -gt 0) { throw 'current-binary L1 evidence path is not fresh; refusing retry/reset' }
    }
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    $evidenceReady = $true

    $reference = Read-HistoricalAuthConfigReference
    $runtime = Assert-ControlledRuntime
    $result.controlled_runtime_manifest_digest = $runtime.manifest_digest
    $result.binary_path = $runtime.binary
    $result.code_mode_host_path = $runtime.helper
    $result.controlled_runtime_preflight_passed = $true

    $auth = Assert-AuthConfig $reference
    $result.auth_identity_fingerprint = $auth.auth_identity
    $result.auth_credential_revision_fingerprint = $auth.auth_revision
    $result.selected_config_raw_sha256 = $auth.config_sha256
    $result.effective_config_digest = $auth.effective_config_digest
    $result.effective_transport_config_digest = $auth.effective_transport_config_digest
    $result.auth_preflight_passed = $true
    $result.config_preflight_passed = $true
    $result.prompt_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($sentinelPrompt))

    $identityManifest = [ordered]@{
        schema_version = 'r03a-current-binary-l1-execution-v1'
        codex_version = $expectedCodexVersion
        codex_binary_sha256 = $expectedCodexSHA256
        code_mode_host_sha256 = $expectedCodeModeHostSHA256
        controlled_runtime_manifest_digest = $runtime.manifest_digest
        runtime_profile = 'windows-native'
        auth_source_class = $expectedAuthSourceClass
        auth_identity_fingerprint = $auth.auth_identity
        auth_credential_revision_fingerprint = $auth.auth_revision
        effective_config_digest = $auth.effective_config_digest
        effective_transport_config_digest = $auth.effective_transport_config_digest
        provider_transport_policy = 'native_default'
        websocket_policy = 'native_default'
        proxy_policy = 'no_injected_proxy'
        invocation = @('app-server', '--stdio')
        model = $model
        effort = $effort
        sandbox = 'read-only'
        dynamic_tool_count = 0
        zero_tool_surface = $true
        prompt_digest = $result.prompt_digest
    }
    $identityCanonicalJSON = $identityManifest | ConvertTo-Json -Compress -Depth 20
    $executionFingerprint = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($identityCanonicalJSON))
    $result.execution_fingerprint = $executionFingerprint

    $executionManifest = [ordered]@{
        schema_version = 'r03a-current-binary-l1-execution-manifest-v1'
        execution_fingerprint = $executionFingerprint
        canonical_json = $identityCanonicalJSON
        controlled_runtime = [ordered]@{
            manifest_path = $RuntimeManifest
            manifest_digest = $runtime.manifest_digest
            staged_binary_path = $runtime.binary
            staged_helper_path = $runtime.helper
            codex_version = $expectedCodexVersion
            codex_binary_sha256 = $expectedCodexSHA256
            code_mode_host_sha256 = $expectedCodeModeHostSHA256
        }
        auth = [ordered]@{
            source_class = $expectedAuthSourceClass
            identity_fingerprint = $auth.auth_identity
            credential_revision_fingerprint = $auth.auth_revision
        }
        config = [ordered]@{
            selected_config_raw_sha256 = $auth.config_sha256
            effective_config_digest = $auth.effective_config_digest
            effective_transport_config_digest = $auth.effective_transport_config_digest
        }
        transport = [ordered]@{
            provider_transport_policy = 'native_default'
            websocket_policy = 'native_default'
            proxy_policy = 'no_injected_proxy'
            invocation = @('app-server', '--stdio')
            dynamic_tool_count = 0
        }
        model = $model
        effort = $effort
        sandbox = 'read-only'
        historical_binary_equivalence = $false
        created_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-JsonFile (Join-Path $Evidence 'execution-manifest.json') $executionManifest

    $preflight = [ordered]@{
        schema_version = 'r03a-current-binary-l1-preflight-v1'
        qualification = 'R0.3A-CURRENT-BINARY-L1'
        status = 'preflight_passed'
        passed = $true
        controlled_runtime_manifest_digest = $runtime.manifest_digest
        controlled_runtime_preflight_passed = $true
        binary_hashes_verified = $true
        version_verified = $true
        auth_source_class = $expectedAuthSourceClass
        auth_identity_fingerprint = $auth.auth_identity
        auth_credential_revision_fingerprint = $auth.auth_revision
        auth_recheck_passed = $true
        selected_config_raw_sha256 = $auth.config_sha256
        config_recheck_passed = $true
        effective_config_digest = $auth.effective_config_digest
        effective_transport_config_digest = $auth.effective_transport_config_digest
        provider_transport_policy = 'native_default'
        proxy_policy = 'no_injected_proxy'
        model = $model
        effort = $effort
        dynamic_tool_count = 0
        execution_fingerprint = $executionFingerprint
        medium_authorized = 1
        high_authorized = 0
        medium_consumed = 0
        provider_egress = 0
        allowance_created = $false
        no_fallback_to_appdata_latest = $true
        historical_evidence_modified = $false
        checked_at = [DateTime]::UtcNow.ToString('o')
    }
    Write-JsonFile (Join-Path $Evidence 'preflight.json') $preflight
    $preflightWritten = $true

    New-Item -ItemType Directory -Force -Path $diagnosticHome, $workspace | Out-Null
    $diagnosticConfig = Join-Path $diagnosticHome 'config.toml'
    $diagnosticAuth = Join-Path $diagnosticHome 'auth.json'
    Copy-Item -LiteralPath $SelectedConfig -Destination $diagnosticConfig
    Copy-Item -LiteralPath $AuthSource -Destination $diagnosticAuth
    Set-ItemProperty -LiteralPath $diagnosticAuth -Name IsReadOnly -Value $true
    if ((Hash-File $diagnosticConfig) -ne $auth.config_sha256) { throw 'diagnostic config snapshot differs from preflight' }
    if ((Hash-File $diagnosticAuth) -ne $auth.auth_revision) { throw 'diagnostic auth snapshot differs from preflight' }

    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $runtime.binary
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.WorkingDirectory = $workspace
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    if ($null -ne $psi.PSObject.Properties['ArgumentList']) {
        $psi.ArgumentList.Add('app-server')
        $psi.ArgumentList.Add('--stdio')
    } else {
        $psi.Arguments = 'app-server --stdio'
    }
    $environment = $psi.EnvironmentVariables
    $environment['HOME'] = $diagnosticHome
    $environment['CODEX_HOME'] = $diagnosticHome
    $environment['PATH'] = "$env:WINDIR\System32;$env:WINDIR"
    foreach ($name in $proxyNames) { $null = $environment.Remove($name) }

    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $psi
    if (-not $process.Start()) { throw 'controlled current-binary process did not start' }
    $result.process_started = $true
    $result.process_start = [DateTime]::UtcNow.ToString('o')
    $stderrTask = $process.StandardError.ReadToEndAsync()

    Send-Frame ([ordered]@{ id = 1; method = 'initialize'; params = [ordered]@{ clientInfo = [ordered]@{ name = 'polis-current-binary-l1'; version = '0.1.0' }; capabilities = [ordered]@{ experimentalApi = $true } } })
    $initialize = Wait-ForResponse 1 10000
    $userAgent = [string]$initialize.result.userAgent
    if ([string]::IsNullOrWhiteSpace($userAgent) -or $userAgent -notmatch [regex]::Escape('0.154.0-alpha.6.2')) { throw "initialize userAgent version mismatch: $userAgent" }
    $result.initialize_completed = $true
    $result.observed_user_agent = $userAgent
    Send-Frame ([ordered]@{ method = 'initialized' })
    Send-Frame ([ordered]@{ id = 2; method = 'thread/start'; params = [ordered]@{ model = $model; allowProviderModelFallback = $false; approvalPolicy = 'never'; sandbox = 'read-only'; cwd = $workspace; environments = @(); ephemeral = $true; dynamicTools = @(); config = [ordered]@{ model_reasoning_effort = $effort }; developerInstructions = $sentinelPrompt } })
    $threadResponse = Wait-ForResponse 2 20000
    if ([string]$threadResponse.result.model -ne $model -or [string]$threadResponse.result.reasoningEffort -ne $effort -or [string]$threadResponse.result.approvalPolicy -ne 'never' -or [string]$threadResponse.result.sandbox.type -ne 'readOnly') { throw 'current-binary thread/start effective profile mismatch' }
    $threadId = [string]$threadResponse.result.thread.id
    if ([string]::IsNullOrEmpty($threadId)) { throw 'current-binary thread/start did not return a thread id' }

    $runtimeFinal = Assert-ControlledRuntime
    $authFinal = Assert-AuthConfig $reference
    if ($runtimeFinal.manifest_digest -ne $runtime.manifest_digest -or $runtimeFinal.binary -ne $runtime.binary -or $runtimeFinal.helper -ne $runtime.helper -or $authFinal.auth_identity -ne $auth.auth_identity -or $authFinal.auth_revision -ne $auth.auth_revision -or $authFinal.config_sha256 -ne $auth.config_sha256) { throw 'final exact revalidation drifted from preflight' }
    $result.final_exact_revalidation_passed = $true

    $allowance = [ordered]@{
        schema_version = 'r03a-current-binary-l1-diagnostic-allowance-v1'
        qualification = 'R0.3A-CURRENT-BINARY-L1'
        authorized_model = $model
        authorized_effort = $effort
        medium_authorized = 1
        high_authorized = 0
        concurrency = 1
        retry = $false
        reset = $false
        tool_count = 0
        runtime_safety_hard_cap = $null
        execution_fingerprint = $executionFingerprint
        controlled_runtime_manifest_digest = $runtime.manifest_digest
        auth_identity_fingerprint = $auth.auth_identity
        auth_credential_revision_fingerprint = $auth.auth_revision
        effective_config_digest = $auth.effective_config_digest
        effective_transport_config_digest = $auth.effective_transport_config_digest
        created_at = [DateTime]::UtcNow.ToString('o')
        medium_started = 0
        provider_egress = 0
    }
    Write-JsonFile (Join-Path $Evidence 'allowance.json') $allowance
    $result.allowance_created = $true

    $turnClock = [Diagnostics.Stopwatch]::StartNew()
    Send-Frame ([ordered]@{ id = 3; method = 'turn/start'; params = [ordered]@{ threadId = $threadId; model = $model; effort = $effort; input = @([ordered]@{ type = 'text'; text = $sentinelPrompt }) } })
    $result.provider_egress = 1
    $allowance.provider_egress = 1
    Write-JsonFile (Join-Path $Evidence 'allowance.json') $allowance

    $sawAgentDelta = $false
    $reconnectOpen = $false
    while ($turnClock.ElapsedMilliseconds -lt $totalDeadlineMs) {
        $remainingTotal = $totalDeadlineMs - [int]$turnClock.ElapsedMilliseconds
        $remainingFirst = if ($result.first_valid_output) { $remainingTotal } else { $firstOutputDeadlineMs - [int]$turnClock.ElapsedMilliseconds }
        if ($remainingFirst -le 0) { break }
        $frame = Read-Frame ([Math]::Max(1, [Math]::Min($remainingTotal, $remainingFirst)))
        if (-not $frame.ok) {
            if ($frame.error -eq 'read_timeout') { break }
            $result.failure_mode = 'stdio_failure'
            $result.terminal_state = $frame.error
            $result.unresolved_transport_state = $true
            break
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
        if ($method -eq 'item/agentMessage/delta') {
            $text = Get-MessageText $frame.message
            if (-not [string]::IsNullOrEmpty($text)) {
                if (-not $result.first_valid_output) {
                    $result.first_valid_output = $true
                    $result.time_to_first_valid_output_ms = [int64]$turnClock.ElapsedMilliseconds
                    $result.first_valid_output_at = [DateTime]::UtcNow.ToString('o')
                }
                $null = $outputParts.Add($text)
                $sawAgentDelta = $true
            }
            if ($reconnectOpen) {
                $reconnectOpen = $false
                $result.recovery_timestamps += [DateTime]::UtcNow.ToString('o')
            }
        }
        if ($method -eq 'item/agentMessage/completed' -and -not $sawAgentDelta) {
            $text = Get-MessageText $frame.message
            if (-not [string]::IsNullOrEmpty($text)) {
                $result.first_valid_output = $true
                $result.time_to_first_valid_output_ms = [int64]$turnClock.ElapsedMilliseconds
                $result.first_valid_output_at = [DateTime]::UtcNow.ToString('o')
                $null = $outputParts.Add($text)
            }
        }
        if ($method -match 'tokenUsage|usage') {
            $result.native_usage_updates = [int]$result.native_usage_updates + 1
            if ($null -ne $frame.message.params.tokenUsage) { $result.token_usage = $frame.message.params.tokenUsage }
            elseif ($null -ne $frame.message.params.usage) { $result.token_usage = $frame.message.params.usage }
        }
        if ($method -eq 'error') {
            $isReconnect = ($true -eq [bool]$frame.message.params.willRetry) -and ($null -ne $frame.message.params.error.codexErrorInfo.responseStreamDisconnected)
            if ($isReconnect) {
                $result.reconnect_count = [int]$result.reconnect_count + 1
                $phase = if ($result.first_valid_output) { 'post_first_output' } else { 'pre_first_output' }
                $result.reconnect_phases += $phase
                $reconnectOpen = $true
                continue
            }
            $result.failure_mode = 'structured_provider_error'
            $result.terminal_state = 'structured_provider_error'
            $result.unresolved_transport_state = $true
            break
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
        if ($result.reconnect_count -gt 0 -and -not $result.first_valid_output) {
            $result.failure_mode = 'no_output_reconnect'
            $result.turn_state = 'first_valid_output_deadline_exceeded'
            $result.terminal_state = 'first_valid_output_deadline_exceeded'
        } elseif (-not $result.first_valid_output) {
            $result.failure_mode = 'turn_incomplete'
            $result.turn_state = 'first_valid_output_deadline_exceeded'
            $result.terminal_state = 'first_valid_output_deadline_exceeded'
        } else {
            $result.failure_mode = 'turn_incomplete'
            $result.terminal_state = 'turn_completed_not_observed'
        }
    }
    if ($result.turn_completed -and $result.first_valid_output -and -not $result.unresolved_transport_state) {
        $result.status = 'passed'
        $result.current_binary_windows_l1 = 'QUALIFIED'
        $result.eligible_for_revised_l2 = $true
    } else {
        $result.status = 'inconclusive'
        $result.current_binary_windows_l1 = 'UNQUALIFIED'
        $result.eligible_for_revised_l2 = $false
    }
} catch {
    $result.error = $_.Exception.Message
    if ($result.provider_egress -gt 0) {
        if ([string]::IsNullOrEmpty($result.failure_mode)) { $result.failure_mode = 'stdio_failure' }
        $result.status = 'inconclusive'
        $result.current_binary_windows_l1 = 'UNQUALIFIED'
        $result.eligible_for_revised_l2 = $false
    } else {
        $result.status = 'preflight_failed'
        $result.failure_mode = 'preflight_failed'
        $result.current_binary_windows_l1 = 'NOT_STARTED'
        $result.eligible_for_revised_l2 = $false
        $result.medium_started = 0
        $result.provider_egress = 0
    }
} finally {
    Stop-DiagnosticProcess
    if ($null -ne $stderrTask) {
        try {
            $stderr = $stderrTask.GetAwaiter().GetResult()
            $result.stderr_bytes = [Text.Encoding]::UTF8.GetByteCount($stderr)
            $result.stderr_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($stderr))
        } catch {}
    }
    $result.stdio_output_line_endings = @($events | Where-Object { $_.direction -eq 'receive' } | Select-Object -ExpandProperty line_ending -Unique)
    $result.protocol_event_count = $events.Count
    $result.sentinel = $sentinel
    $result.developer_instruction_digest = $result.prompt_digest
    $result.finished_at = [DateTime]::UtcNow.ToString('o')
    if ($evidenceReady) {
        if (-not $preflightWritten) {
            $failedPreflight = [ordered]@{
                schema_version = 'r03a-current-binary-l1-preflight-v1'
                qualification = 'R0.3A-CURRENT-BINARY-L1'
                status = 'preflight_failed'
                passed = $false
                medium_consumed = 0
                provider_egress = 0
                allowance_created = $false
                no_fallback_to_appdata_latest = $true
                error = $result.error
                checked_at = [DateTime]::UtcNow.ToString('o')
            }
            Write-JsonFile (Join-Path $Evidence 'preflight.json') $failedPreflight
        }
        if ($events.Count -gt 0) { try { Write-ProviderEvents } catch {} }
        try { Write-JsonFile (Join-Path $Evidence 'windows-live-run.json') $result } catch {}
        try { Write-JsonFile (Join-Path $Evidence 'result.json') $result } catch {}
    }
    if (Test-Path -LiteralPath $diagnosticRoot) { Remove-Item -LiteralPath $diagnosticRoot -Recurse -Force }
}

if ($result.status -eq 'preflight_failed') { exit 1 }
if ($result.status -ne 'passed') { exit 2 }
