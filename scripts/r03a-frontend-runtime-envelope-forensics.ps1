$ErrorActionPreference = 'Stop'

$root = 'D:\Programs\Polis\evidence\development\r0.3a-frontend-runtime-initialize-forensics-v1'
$wsl = Get-Content -Raw -LiteralPath (Join-Path $root 'wsl-runtime-preflight-v2.json') | ConvertFrom-Json
$windows = Get-Content -Raw -LiteralPath (Join-Path $root 'windows-evidence-v2\windows-runtime-preflight-v2.json') | ConvertFrom-Json
$l2 = Get-Content -Raw -LiteralPath 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-frontend-l2-live\result.json' | ConvertFrom-Json

$same = [ordered]@{
    codex_binary_sha256 = ($wsl.launch_envelope.binary_sha256 -eq $windows.launch_envelope.binary_sha256)
    code_mode_host_sha256 = ($wsl.launch_envelope.helper_sha256 -eq $windows.launch_envelope.helper_sha256)
    model = 'SAME:gpt-5.6-luna'
    effort = 'SAME:medium'
    tool_count = ($windows.registered_tool_count -eq 12 -and $wsl.registered_tool_count -eq 12 -and $l2.registered_tool_count -eq 12)
    tool_manifest = 'SAME:91bfe5116ae4e7c2b6ab1c117e03879e67f9ee0a863d68e77b513958b9c99615'
    aggregate_schema_bytes = 'SAME:2217'
    launch_mode = 'DIFFERENT'
    process_executable = 'DIFFERENT:bwrap vs codex.exe'
    argv = 'DIFFERENT:bwrap namespace argv vs app-server --stdio'
    runtime_os = 'DIFFERENT:linux WSL vs windows'
    working_directory = 'DIFFERENT'
    home_codex_home = 'DIFFERENT'
    stdio_framing = 'SAME:stdin_stdout_jsonl;stderr_separate_bounded'
    process_boundary = 'DIFFERENT:WSL process group/bubblewrap vs Windows process handle'
}
$report = [ordered]@{
    schema_version = 'r03a-frontend-runtime-envelope-diff@1'
    classification = 'EXECUTION_FINGERPRINT_COVERAGE_DEFECT'
    secondary_runtime_finding = 'WSL_BWRAP_WINDOWS_INTEROP_INCOMPATIBLE'
    historical_execution_fingerprint = '708790c760a905f8eeced8ba426b024341eec789bdb79ad1d89f87cb31982091'
    provider_tool_surface_changed = $false
    execution_fingerprint_coverage_changed = $true
    coverage = [ordered]@{
        covered = @('binary_sha256','code_mode_host_sha256','model','effort','tool_manifest','schema_digest','schema_bytes')
        not_bound = @('launch_mode','process_executable','argv','runtime_os','working_directory','HOME','CODEX_HOME','stdio_boundary','process_stop_semantics')
    }
    comparison = $same
    old_failure = [ordered]@{ status=$wsl.status; launch_mode=$wsl.launch_envelope.launch_mode; process=$wsl.launch_envelope.process_executable; initialize=$wsl.initialize_completed; error=$wsl.error; stderr_classification=$wsl.stderr_classification; provider_egress=$wsl.provider_egress }
    hardened_local_probe = [ordered]@{ status=$windows.status; launch_mode=$windows.launch_envelope.launch_mode; process=$windows.launch_envelope.process_executable; initialize=$windows.initialize_completed; local_protocol_ready=$windows.local_protocol_ready; turn_started=$windows.turn_started; provider_egress=$windows.provider_egress; business_mutation=$windows.business_mutation; stop_confirmed=$windows.stop_confirmed; process_exit=$windows.process_exit }
    historical_l2 = [ordered]@{ execution_fingerprint=$l2.execution_fingerprint; runtime_profile=$l2.runtime_profile; invocation=$l2.invocation; registered_tool_count=$l2.registered_tool_count; aggregate_schema_bytes=$l2.aggregate_schema_bytes; first_valid_output=$l2.first_valid_output; provider_egress=$l2.provider_egress }
    historical_evidence_modified = $false
}
$out = Join-Path $root 'execution-envelope-diff.json'
[IO.File]::WriteAllText($out, (($report | ConvertTo-Json -Depth 20) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
Write-Output $out
