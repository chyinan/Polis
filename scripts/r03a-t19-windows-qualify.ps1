param(
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-t19',
    [string]$Binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\bin\fd4c151a749f3ab4\codex.exe',
    [string]$AuthSource = 'C:\Users\chyinan\.codex\auth.json',
    [string]$SelectedLinuxConfig = 'D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml'
)

$ErrorActionPreference = 'Stop'
$sentinelPrompt = 'Reply with exactly: POLIS_TRANSPORT_CANARY_OK'
$proxyNames = @('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY', 'POLIS_NATIVE_PROXY')
$result = [ordered]@{
    schema_version = 't19-windows-run-v1'
    status = 'failed'
    diagnostic_profile = 'windows_native_isolated'
    provider_or_internet_accessed = $false
    auth_snapshot_used = $true
    config_snapshot_used = $true
    dynamic_tool_count = 0
    model = 'gpt-5.6-luna'
    effort = 'medium'
    sandbox = 'read-only'
    provider_transport_policy = 'native_default'
    proxy_policy = 'no_injected_proxy'
    invocation = 'app-server --stdio'
    prompt_semantics = 'same_tiny_sentinel'
    process_started = $false
    initialize_completed = $false
    thread_started = $false
    stdio_input_line_ending = 'CRLF'
    stdio_output_line_endings = @()
    pipe_semantics = 'redirected_standard_pipes'
    job_object_used = $false
    stop_confirmed = $false
    code_mode_host_resolved = $false
    structured_error_fixture = 'deferred_to_offline_go_tests'
}
$process = $null
$diagnosticRoot = Join-Path ([IO.Path]::GetTempPath()) ('polis-r03a-t19-' + [guid]::NewGuid().ToString('N'))
$diagnosticHome = Join-Path $diagnosticRoot 'codex-home'
$workspace = Join-Path $diagnosticRoot 'workspace'
$events = New-Object System.Collections.Generic.List[object]

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

function Add-ProtocolEvent($Direction, $Message, $LineEnding) {
    $event = [ordered]@{ direction = $Direction; line_ending = $LineEnding }
    if ($null -ne $Message.id) { $event.id = [int]$Message.id }
    if ($null -ne $Message.method) { $event.method = [string]$Message.method }
    if ($Direction -eq 'receive' -and $null -ne $Message.result) {
        $event.result_present = $true
        if ($null -ne $Message.result.userAgent) { $event.user_agent_version_match = ([string]$Message.result.userAgent -match '0\.153\.4') }
        if ($null -ne $Message.result.thread) {
            $event.model = [string]$Message.result.model
            $event.reasoning_effort = [string]$Message.result.reasoningEffort
            $event.approval_policy = [string]$Message.result.approvalPolicy
            $event.sandbox_type = [string]$Message.result.sandbox.type
            $event.cwd_present = ($null -ne $Message.result.cwd)
        }
    }
    $events.Add([pscustomobject]$event)
}

function Send-Frame($Message) {
    $json = $Message | ConvertTo-Json -Compress -Depth 20
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
        $task = $stream.ReadAsync($buffer, 0, 1)
        $remaining = [Math]::Max(1, [int]($deadline.Subtract([DateTime]::UtcNow).TotalMilliseconds))
        if (-not $task.Wait($remaining)) { return [pscustomobject]@{ ok = $false; error = 'read_timeout' } }
        if ($task.Result -eq 0) { return [pscustomobject]@{ ok = $false; error = 'eof' } }
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

function Wait-ForId([int]$Id, [int]$TimeoutMs) {
    $deadline = [DateTime]::UtcNow.AddMilliseconds($TimeoutMs)
    while ([DateTime]::UtcNow -lt $deadline) {
        $frame = Read-Frame ([Math]::Max(1, [int]($deadline.Subtract([DateTime]::UtcNow).TotalMilliseconds)))
        if (-not $frame.ok) { throw "protocol_$($frame.error)" }
        Add-ProtocolEvent 'receive' $frame.message $frame.line_ending
        if ($null -ne $frame.message.id -and [int]$frame.message.id -eq $Id) { return $frame.message }
    }
    throw "protocol_response_timeout_$Id"
}

function Stop-DiagnosticProcess {
    if ($null -eq $process) { return }
    try { if (-not $process.HasExited) { $process.StandardInput.Close() } } catch {}
    if (-not $process.WaitForExit(3000)) {
        try { $process.Kill() } catch {}
        $process.WaitForExit()
        $result.stop_mechanism = 'Windows Process handle Kill plus WaitForExit'
    } else {
        $result.stop_mechanism = 'stdin close plus Windows Process handle WaitForExit'
    }
    $result.stop_confirmed = $process.HasExited
    $result.exit_code = $process.ExitCode
}

try {
    if (Test-Path -LiteralPath (Join-Path $Evidence 'windows-run.json')) { throw 'T19 evidence already exists; refusing rerun' }
    if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) { throw 'Windows Codex binary is unavailable' }
    if (-not (Test-Path -LiteralPath $AuthSource -PathType Leaf)) { throw 'controlled auth source is unavailable' }
    if (-not (Test-Path -LiteralPath $SelectedLinuxConfig -PathType Leaf)) { throw 'selected Linux config is unavailable' }
    New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
    New-Item -ItemType Directory -Force -Path $diagnosticHome, $workspace | Out-Null
    $diagnosticConfig = Join-Path $diagnosticHome 'config.toml'
    $diagnosticAuth = Join-Path $diagnosticHome 'auth.json'
    Copy-Item -LiteralPath $SelectedLinuxConfig -Destination $diagnosticConfig
    Copy-Item -LiteralPath $AuthSource -Destination $diagnosticAuth
    Set-ItemProperty -LiteralPath $diagnosticAuth -Name IsReadOnly -Value $true
    $result.binary_sha256 = Hash-File $Binary
    $result.code_mode_host_sha256 = Hash-File (Join-Path (Split-Path $Binary) 'codex-code-mode-host.exe')
    $result.selected_config_raw_sha256 = Hash-File $diagnosticConfig
    $result.config_source_role = 'exact_selected_non_secret_T14C_config_only'
    $result.auth_source_role = 'read_only_external_snapshot; opaque identity/revision recorded by Go manifest'
    $result.diagnostic_home_role = 'isolated temporary CODEX_HOME outside repo/evidence'
    $result.workspace_role = 'diagnostic_workspace_outside_repo/evidence'
    $result.prompt_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($sentinelPrompt))
    $result.developer_instruction_digest = $result.prompt_digest
    $codeModeHost = Join-Path (Split-Path $Binary) 'codex-code-mode-host.exe'
    $result.code_mode_host_resolved = Test-Path -LiteralPath $codeModeHost -PathType Leaf

    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $Binary
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
    if (-not $process.Start()) { throw 'Windows native process did not start' }
    $result.process_started = $true
    $stderrTask = $process.StandardError.ReadToEndAsync()

    Send-Frame ([ordered]@{ id = 1; method = 'initialize'; params = [ordered]@{ clientInfo = [ordered]@{ name = 'polis-t19'; version = '0.1.0' }; capabilities = [ordered]@{ experimentalApi = $true } } })
    $initialize = Wait-ForId 1 10000
    if ($null -eq $initialize.result.userAgent -or -not ([string]$initialize.result.userAgent -match '0\.153\.4')) { throw 'Windows native initialize version mismatch' }
    $result.initialize_completed = $true
    Send-Frame ([ordered]@{ method = 'initialized' })
    Send-Frame ([ordered]@{ id = 2; method = 'thread/start'; params = [ordered]@{ model = 'gpt-5.6-luna'; allowProviderModelFallback = $false; approvalPolicy = 'never'; sandbox = 'read-only'; cwd = $workspace; environments = @(); ephemeral = $true; dynamicTools = @(); config = [ordered]@{ model_reasoning_effort = 'medium' }; developerInstructions = $sentinelPrompt } })
    $thread = Wait-ForId 2 10000
    if ($thread.result.model -ne 'gpt-5.6-luna' -or $thread.result.reasoningEffort -ne 'medium' -or $thread.result.approvalPolicy -ne 'never' -or $thread.result.sandbox.type -ne 'readOnly') { throw 'Windows native thread/start effective profile mismatch' }
    $result.thread_started = $true
    $result.thread_id_observed = $true
    $result.protocol_subset = 'initialize, initialized, thread/start, thread/started'
    Stop-DiagnosticProcess
    $stderr = $stderrTask.GetAwaiter().GetResult()
    $result.stderr_bytes = ([Text.Encoding]::UTF8.GetByteCount($stderr))
    $result.stderr_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes($stderr))
    $result.stdio_output_line_endings = @($events | Where-Object { $_.direction -eq 'receive' } | Select-Object -ExpandProperty line_ending -Unique)
    $result.protocol_event_count = $events.Count
    $protocolShapes = @($events | ForEach-Object {
        $method = [string]$_.method
        $id = if ($null -eq $_.id) { '' } else { [string]$_.id }
        if ($method -eq '' -and $id -ne '') { $method = 'response' }
        "$($_.direction):${method}:${id}"
    } | Sort-Object)
    $result.protocol_shape_digest = Hash-Bytes ([Text.Encoding]::UTF8.GetBytes(($protocolShapes -join "`n")))
    $result.status = 'passed'
} catch {
    $result.status = 'failed'
    $result.error_class = $_.Exception.Message
    Stop-DiagnosticProcess
} finally {
    if ($null -ne $process -and -not $process.HasExited) { Stop-DiagnosticProcess }
    if (Test-Path -LiteralPath $diagnosticRoot) { Remove-Item -LiteralPath $diagnosticRoot -Recurse -Force }
    $json = $result | ConvertTo-Json -Depth 20
    [IO.File]::WriteAllText((Join-Path $Evidence 'windows-run.json'), $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

if ($result.status -ne 'passed') { exit 1 }
