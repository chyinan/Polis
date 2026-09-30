# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$outDir = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b15-business-path-provider-initialize-hardening')).Path
$b14 = Get-Content -Raw (Join-Path $workspace 'evidence\development\r0.5b14-product-tool-surface-v4-live-qualification-attempt-3\product-execution-manifest.json') | ConvertFrom-Json
$live = Get-Content -Raw (Join-Path $workspace 'evidence/development/r0.5b-real-provider-product-smoke-live-3/live-3-result.json') | ConvertFrom-Json
$i = $b14.execution_identity
$l = $i.launch_runtime_envelope
$rows = @()
function Add-Row([string]$field, [string]$b14Value, [string]$liveValue, [string]$classification, [string]$basis) {
  $script:rows += [pscustomobject]@{field=$field;b14=$b14Value;live3=$liveValue;classification=$classification;basis=$basis}
}
Add-Row 'surface_id' ([string]$i.surface_id) ([string]$live.surface.surface_id) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same exact surface'
Add-Row 'tool_count' ([string]$i.tool_count) ([string]$live.surface.tool_count) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same exact surface'
Add-Row 'manifest_digest' ([string]$i.manifest_digest) ([string]$live.surface.manifest_digest) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same exact surface'
Add-Row 'aggregate_schema_bytes' ([string]$i.aggregate_schema_bytes) ([string]$live.surface.schema_bytes) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same exact surface'
Add-Row 'aggregate_schema_digest' ([string]$i.aggregate_schema_digest) ([string]$live.surface.schema_digest) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same exact surface'
Add-Row 'exact_surface_execution_fingerprint' ([string]$b14.product_exact_surface_execution_fingerprint) ([string]$live.surface.exact_surface_execution_fingerprint) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same exact surface execution binding'
Add-Row 'runtime_version' ([string]$i.provider_runtime.native_version) ([string]$live.provider_binding.runtime_version) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same verified runtime version'
Add-Row 'binary_sha256' ([string]$i.provider_runtime.binary_sha256) ([string]$live.provider_binding.binary_sha256) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same verified binary'
Add-Row 'helper_sha256' ([string]$i.provider_runtime.helper_sha256) ([string]$live.provider_binding.helper_sha256) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same verified helper'
Add-Row 'launch_envelope_fingerprint' ([string]$l.fingerprint) ([string]$live.provider_binding.execution_envelope) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'same canonical envelope fingerprint'
Add-Row 'argv' (($l.argv -join ' | ')) '' 'UNEXPECTED_LAUNCH_DIVERGENCE' 'LIVE_3 did not persist resolved argv'
Add-Row 'working_directory' ([string]$l.working_directory) '' 'UNEXPECTED_LAUNCH_DIVERGENCE' 'LIVE_3 did not persist resolved cwd'
Add-Row 'controlled_environment' (($l.environment | ForEach-Object { $_.name + ':' + $_.value_sha256 }) -join ' | ') '' 'UNEXPECTED_LAUNCH_DIVERGENCE' 'LIVE_3 did not persist safe environment fingerprint'
Add-Row 'home_codex_home_userprofile' (($l.home + ' / ' + $l.codex_home)) '' 'UNEXPECTED_LAUNCH_DIVERGENCE' 'LIVE_3 session-home semantics unavailable'
Add-Row 'temp_tmp' ([string]$l.temp_directory) '' 'UNEXPECTED_LAUNCH_DIVERGENCE' 'LIVE_3 temp semantics unavailable'
Add-Row 'stdio_pipe_flags' (($l.stdio + ' / ' + $l.pipe_setup_mode + ' / ' + ($l.process_creation_flags -join ','))) '' 'UNEXPECTED_LAUNCH_DIVERGENCE' 'LIVE_3 launch spec unavailable'
Add-Row 'runtime_evidence_root' ([string]$l.evidence_directory) '' 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'distinct business session evidence root not persisted'
Add-Row 'business_identity' '' ($live.company.id + ' / ' + $live.mission.id + ' / ' + $live.provider_binding.task_id + ' / ' + $live.provider_binding.employee_id + ' / ' + $live.provider_binding.session_id) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'diagnostic has no business identity'
Add-Row 'task_validation_binding' '' ([string]$live.provider_binding.task_validation_binding_digest) 'POSSIBLE_INITIALIZE_INPUT' 'business binding outside diagnostic path'
Add-Row 'workspace_digest_revision' '' ($live.provider_binding.workspace_digest + ' / ' + $live.provider_binding.workspace_revision) 'POSSIBLE_INITIALIZE_INPUT' 'business workspace binding outside diagnostic path'
Add-Row 'business_authorization_reservation' '' ([string]$live.counters.business_authorizations + ' / ' + [string]$live.counters.provider_reservations) 'EXPECTED_BUSINESS_CONTEXT_DIFFERENCE' 'business-only accounting path'
$comparison = [ordered]@{record_type='polis-r0.5b15-b14-live3-execution-context-diff@1';historical_precise_failure_cause='NOT_DETERMINABLE_FROM_FROZEN_EVIDENCE';b14=[ordered]@{surface_id=[string]$i.surface_id;execution_fingerprint=[string]$b14.product_exact_surface_execution_fingerprint;launch_envelope=[string]$l.fingerprint;runtime_version=[string]$i.provider_runtime.native_version;binary_sha256=[string]$i.provider_runtime.binary_sha256;helper_sha256=[string]$i.provider_runtime.helper_sha256};live3=[ordered]@{surface_id=[string]$live.surface.surface_id;execution_fingerprint=[string]$live.surface.exact_surface_execution_fingerprint;launch_envelope=[string]$live.provider_binding.execution_envelope;runtime_version=[string]$live.provider_binding.runtime_version;binary_sha256=[string]$live.provider_binding.binary_sha256;helper_sha256=[string]$live.provider_binding.helper_sha256;process_attached=[string]$live.provider.process_created_and_attached;initialize='NOT_OBSERVED';thread_start='NOT_OBSERVED';turn_start=[string]$live.provider.turn_start;provider_egress=[string]$live.provider.provider_egress};differences=$rows;missing_business_path_observability=@('resolved ProcessLaunchSpec/argv/cwd/environment safe fingerprint','initialize request/ack lifecycle evidence','child exit code/safe stderr category','full terminal phase/reason code')}
$comparison | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $outDir 'b14-live3-execution-context-diff.json')
'R0.5B15_SIMPLE_DIFF_WRITTEN'
