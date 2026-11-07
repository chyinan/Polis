param(
    [string]$OutputDirectory = "evidence/development/r0.3a-t4"
)

$ErrorActionPreference = "Stop"
$utf8 = [System.Text.UTF8Encoding]::new($false)
$sha = [System.Security.Cryptography.SHA256]::Create()

function Get-HashText([string]$Text) {
    $bytes = $utf8.GetBytes($Text)
    return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace("-", "").ToLowerInvariant()
}

function Get-CompactJson($Value) {
    return ($Value | ConvertTo-Json -Compress -Depth 100)
}

function Get-HashJson($Value) {
    return Get-HashText (Get-CompactJson $Value)
}

function Read-Protocol([string]$Path) {
    $rows = @()
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $rows += ($line | ConvertFrom-Json)
    }
    return $rows
}

function Get-StringBytes([string]$Text) {
    if ($null -eq $Text) { return 0 }
    return $utf8.GetByteCount($Text)
}

function Get-Prop($Object, [string]$Name) {
    if ($null -eq $Object) { return $null }
    $p = $Object.PSObject.Properties[$Name]
    if ($null -eq $p) { return $null }
    return $p.Value
}

function New-Run([string]$Name, [string]$Kind, [string]$Protocol, [string]$Session, [string]$Capability, [string]$Preflight) {
    return [pscustomobject]@{ Name=$Name; Kind=$Kind; Protocol=$Protocol; Session=$Session; Capability=$Capability; Preflight=$Preflight }
}

function Normalize-Run($Definition) {
    $rows = @(Read-Protocol $Definition.Protocol)
    $events = @()
    $turnStart = $null
    $turnStarted = $null
    $firstOutput = $null
    $firstTool = $null
    $disconnects = 0
    $reconnectPhases = @()
    $terminal = $null
    $firstAfterTurn = $null
    $firstOutputSeen = $false
    $threadParams = $null
    $turnParams = $null
    $toolCalls = @()
    $usageUpdates = 0
    $threadId = $null
    $turnId = $null
    $baseWall = $null

    foreach ($row in $rows) {
        $wall = [DateTimeOffset]::Parse([string]$row.time)
        if ($null -eq $baseWall) { $baseWall = $wall }
        $data = $row.data
        $direction = [string]$row.direction
        $method = [string](Get-Prop $data "method")
        $params = Get-Prop $data "params"
        $kind = $null
        if ($direction -eq "send" -and $method -eq "thread/start") {
            $kind = "thread_start"
            $threadParams = $params
        } elseif ($direction -eq "send" -and $method -eq "turn/start") {
            $kind = "turn_start"
            $turnStart = $wall
            $turnParams = $params
            $threadId = [string](Get-Prop $params "threadId")
        } elseif ($direction -eq "receive" -and $method -eq "thread/started") {
            $kind = "thread_started"
            $thread = Get-Prop $params "thread"
            if ($null -ne $thread) { $threadId = [string](Get-Prop $thread "id") }
        } elseif ($direction -eq "receive" -and $method -eq "turn/started") {
            $kind = "turn_started"
            $turnStarted = $wall
            $turn = Get-Prop $params "turn"
            if ($null -ne $turn) { $turnId = [string](Get-Prop $turn "id") }
        } elseif ($direction -eq "receive" -and $method -eq "error") {
            $err = Get-Prop $params "error"
            $info = if ($null -ne $err) { Get-Prop $err "codexErrorInfo" } else { $null }
            $disconnect = if ($null -ne $info) { Get-Prop $info "responseStreamDisconnected" } else { $null }
            $willRetry = [bool](Get-Prop $params "willRetry")
            if ($null -ne $disconnect) {
                $disconnects++
                $phase = if ($firstOutputSeen) { "post_first_output_reconnecting" } else { "pre_first_output_reconnecting" }
                $reconnectPhases += $phase
                $kind = "disconnect_reconnectable"
            } else {
                $kind = "provider_error"
            }
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = "error" }
        } elseif ($direction -eq "receive" -and $method -eq "thread/tokenUsage/updated") {
            $kind = "usage_update"
            $usageUpdates++
            if ($null -eq $firstOutput) { $firstOutput = $wall; $firstOutputSeen = $true }
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = "thread/tokenUsage/updated" }
        } elseif ($direction -eq "receive" -and $method -eq "item/tool/call") {
            $kind = "tool_call"
            $p = $params
            $toolCalls += [pscustomobject]@{ Name=[string](Get-Prop $p "tool"); CallID=[string](Get-Prop $p "callId"); ArgumentsBytes=Get-StringBytes (Get-CompactJson (Get-Prop $p "arguments")) }
            if ($null -eq $firstTool) { $firstTool = $wall }
            if ($null -eq $firstOutput) { $firstOutput = $wall; $firstOutputSeen = $true }
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = "item/tool/call" }
        } elseif ($direction -eq "receive" -and ($method -like "item/agentMessage*")) {
            $kind = "valid_output"
            if ($null -eq $firstOutput) { $firstOutput = $wall; $firstOutputSeen = $true }
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = $method }
        } elseif ($direction -eq "receive" -and ($method -eq "item/started" -or $method -eq "item/completed")) {
            $kind = "item_event"
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = $method }
        } elseif ($direction -eq "receive" -and $method -eq "turn/completed") {
            $kind = "turn_completed"
            $turn = Get-Prop $params "turn"
            if ($null -ne $turn) { $terminal = [string](Get-Prop $turn "status") }
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = "turn/completed" }
        } elseif ($direction -eq "receive" -and $method -eq "thread/status/changed") {
            $kind = "status_changed"
            if ($null -eq $firstAfterTurn -and $null -ne $turnStarted) { $firstAfterTurn = "thread/status/changed" }
        } elseif ($direction -eq "send" -and $method -eq "initialized") {
            $kind = "initialized"
        }
        if ($null -ne $kind) {
            $emitted = Get-Prop $data "emittedAtMs"
            $auditDelta = if ($null -ne $baseWall) { [math]::Round(($wall-$baseWall).TotalMilliseconds,3) } else { $null }
            $events += [pscustomobject]@{ Kind=$kind; Direction=$direction; Method=$method; WallTime=$wall.ToString("o"); AuditDeltaMs=$auditDelta; MonotonicDeltaMs=$null; MonotonicAvailable=$false; EmittedAtMs=$emitted }
        }
    }

    $session = $null
    if ($Definition.Session -and (Test-Path $Definition.Session)) {
        try { $session = Get-Content -Raw $Definition.Session | ConvertFrom-Json } catch { $session = $null }
    }
    if ($null -ne $session) {
        $events += [pscustomobject]@{ Kind="process_stop"; Direction="control"; Method="stop_proof"; WallTime=[string](Get-Prop $session "finished"); AuditDeltaMs=$null; MonotonicDeltaMs=$null; MonotonicAvailable=$false; EmittedAtMs=$null }
    }
    $threadStart = Get-Prop $threadParams "dynamicTools"
    $toolSizes = [ordered]@{}
    foreach ($tool in @($threadStart)) {
        $toolSizes[[string](Get-Prop $tool "name")] = Get-StringBytes (Get-CompactJson $tool)
    }
    $cap = if ($Definition.Capability -and (Test-Path $Definition.Capability)) { Get-Content -Raw $Definition.Capability | ConvertFrom-Json } else { $null }
    $pre = if ($Definition.Preflight -and (Test-Path $Definition.Preflight)) { Get-Content -Raw $Definition.Preflight | ConvertFrom-Json } else { $null }
    $threadEvent = @($rows | Where-Object { $_.direction -eq "receive" -and $_.data.method -eq "thread/started" } | Select-Object -First 1)
    $threadInfo = if ($threadEvent.Count -gt 0) { Get-Prop (Get-Prop $threadEvent[0].data.params "thread") "cliVersion" } else { $null }
    $model = Get-Prop $threadParams "model"
    $effort = Get-Prop (Get-Prop $threadParams "config") "model_reasoning_effort"
    if ($null -eq $effort) { $effort = Get-Prop $turnParams "effort" }
    $promptBytes = 0
    foreach ($item in @((Get-Prop $turnParams "input"))) { $promptBytes += Get-StringBytes ([string](Get-Prop $item "text")) }
    $summary = [pscustomobject]@{
        Name=$Definition.Name; Kind=$Definition.Kind; EventCount=$events.Count; Trace=$events
        FirstProviderEvent=$firstAfterTurn; FirstValidOutputTime=if($null -ne $firstOutput){$firstOutput.ToString("o")}else{$null}
        FirstToolCallTime=if($null -ne $firstTool){$firstTool.ToString("o")}else{$null}; DisconnectCount=$disconnects
        ReconnectPhases=@($reconnectPhases); TerminalState=$terminal; UsageUpdates=$usageUpdates
        MonotonicDeltaRecorded=$false; MonotonicLimitation="historical protocol contains no monotonic timestamps"
        Runtime=[pscustomobject]@{ CodexVersion=$threadInfo; BinarySHA256=if($null -ne $cap){Get-Prop $cap "binary_sha256"}else{Get-Prop $pre "binary_sha256"}; CodeModeHostSHA256=(Get-Prop $cap "code_mode_host_sha256"); SchemaDigest=if($null -ne $cap){Get-Prop $cap "schema_sha256"}else{Get-Prop $pre "schema_digest"}; CapabilityDigest=if($null -ne $pre){Get-Prop $pre "capability_digest"}else{$null}; NativeProtocol=if($null -ne $cap){Get-Prop $cap "native_protocol"}else{Get-Prop $pre "native_protocol"} }
        TurnRequest=[pscustomobject]@{ Model=$model; Effort=$effort; ThreadStartRequestBytes=if($null -ne $threadParams){Get-StringBytes (Get-CompactJson $threadParams)}else{$null}; TurnStartRequestBytes=if($null -ne $turnParams){Get-StringBytes (Get-CompactJson $turnParams)}else{$null}; DeveloperInstructionBytes=Get-StringBytes (Get-Prop $threadParams "developerInstructions"); PromptInputBytes=$promptBytes; ToolCount=@($threadStart).Count; ToolSchemaTotalBytes=($toolSizes.Values | Measure-Object -Sum).Sum; ToolSchemaDigest=if($null -ne $threadStart){Get-HashJson @($threadStart)}else{$null}; ToolSchemaBytes=$toolSizes; AttachmentsOrResources="not_recorded"; HistoryContext="not_recorded"; Cwd=Get-Prop $threadParams "cwd"; Sandbox=Get-Prop $threadParams "sandbox" }
        ToolBinding=[pscustomobject]@{ Names=@($toolSizes.Keys); SchemaDigest=if($null -ne $threadStart){Get-HashJson @($threadStart)}else{$null}; RegistrationOrder=@($toolSizes.Keys); CallbackReadiness=if($null -ne $pre -and $pre.native_protocol -eq "passed_without_inference"){"preflight_passed"}else{"not_recorded"}; EmployeeBinding="not_recorded" }
    }
    return $summary
}

$runs = @(
    (New-Run "r02-medium-1" "known_good" "evidence/development/r0.2/luna-1/e9485489eeece81bcce3678779e3f91c/protocol.jsonl" "evidence/development/r0.2/luna-1/e9485489eeece81bcce3678779e3f91c/r02-session.json" "evidence/development/r0.2/luna-1/capability.json" $null),
    (New-Run "r02-medium-2" "known_good" "evidence/development/r0.2/luna-1/8cc15e9fa95d3feb5407d9254dd78860/protocol.jsonl" "evidence/development/r0.2/luna-1/8cc15e9fa95d3feb5407d9254dd78860/r02-session.json" "evidence/development/r0.2/luna-1/capability.json" $null),
    (New-Run "r02h3-high" "known_good_auxiliary" "evidence/development/r0.2h3/30dbe7437478dbd0b9af8d64a9912698/protocol.jsonl" "evidence/development/r0.2h3/30dbe7437478dbd0b9af8d64a9912698/ r02-session.json" $null "evidence/development/r0.2h3/contamination-preflight.json"),
    (New-Run "r03a-initial-backend" "failing" "evidence/development/r0.3a-real/luna-1/70a7fb6d45e09588eca30154d2789732/protocol.jsonl" "evidence/development/r0.3a-real/luna-1/70a7fb6d45e09588eca30154d2789732/r03a-session.json" $null "evidence/development/r0.3a-real/luna-1/preflight.json"),
    (New-Run "r03a-t2-backend" "failing" "evidence/development/r0.3a-t2/luna-1/caf04cf9e13fc9f35cf2115cd7dcec59/protocol.jsonl" "evidence/development/r0.3a-t2/luna-1/caf04cf9e13fc9f35cf2115cd7dcec59/r03a-session.json" $null "evidence/development/r0.3a-t2/luna-1/preflight.json"),
    (New-Run "r03a-t3-backend" "failing" "evidence/development/r0.3a-t3/luna-1/33dadffd74f2f73d290961e07163ccd8/protocol.jsonl" "evidence/development/r0.3a-t3/luna-1/33dadffd74f2f73d290961e07163ccd8/r03a-session.json" $null "evidence/development/r0.3a-t3/luna-1/preflight.json")
)
# Correct the known-good auxiliary path typo defensively without touching source evidence.
$runs[2].Session = "evidence/development/r0.2h3/30dbe7437478dbd0b9af8d64a9912698/r02-session.json"
$summaries = @($runs | ForEach-Object { Normalize-Run $_ })

$dimensions = @("CodexVersion","BinarySHA256","CodeModeHostSHA256","SchemaDigest","NativeProtocol","Model","Effort","Cwd","Sandbox","ToolSchemaDigest","ToolCount","DeveloperInstructionBytes","PromptInputBytes","ThreadStartRequestBytes","TurnStartRequestBytes","ProxyEnvironment","HistoryContext","CallbackReadiness")
$matrix = @()
foreach ($dimension in $dimensions) {
    $known = @($summaries | Where-Object {$_.Kind -eq "known_good"} | ForEach-Object { Get-Prop $_.Runtime $dimension; if($dimension -in @("Model","Effort","Cwd","Sandbox","ToolSchemaDigest","ToolCount","DeveloperInstructionBytes","PromptInputBytes","ThreadStartRequestBytes","TurnStartRequestBytes","HistoryContext","CallbackReadiness")){Get-Prop $_.TurnRequest $dimension} }) | Where-Object {$null -ne $_}
    $failing = @($summaries | Where-Object {$_.Kind -eq "failing"} | ForEach-Object { Get-Prop $_.Runtime $dimension; if($dimension -in @("Model","Effort","Cwd","Sandbox","ToolSchemaDigest","ToolCount","DeveloperInstructionBytes","PromptInputBytes","ThreadStartRequestBytes","TurnStartRequestBytes","HistoryContext","CallbackReadiness")){Get-Prop $_.TurnRequest $dimension} }) | Where-Object {$null -ne $_}
    $all = @($known + $failing | ForEach-Object {[string]$_} | Select-Object -Unique)
    $classification = "not_recorded"
    if ($known.Count -gt 0 -and $failing.Count -gt 0) {
        $classification = if ((@($known | ForEach-Object {[string]$_} | Select-Object -Unique) -join "|") -eq (@($failing | ForEach-Object {[string]$_} | Select-Object -Unique) -join "|")) { "confirmed_same" } else { "confirmed_different" }
    }
    $matrix += [pscustomobject]@{ Dimension=$dimension; Classification=$classification; KnownGood=@($known | Select-Object -Unique); Failing=@($failing | Select-Object -Unique); Note=if($known.Count -eq 0 -or $failing.Count -eq 0){"not_recorded in one or more run groups"}else{$null} }
}

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$output = [pscustomobject]@{ GeneratedAt=(Get-Date).ToUniversalTime().ToString("o"); MonotonicClock="not_recorded in historical traces; audit deltas use wall timestamps/emittedAtMs"; Runs=$summaries; DifferenceMatrix=$matrix }
$jsonPath = Join-Path $OutputDirectory "normalized-diff.json"
$mdPath = Join-Path $OutputDirectory "normalized-diff.md"
$output | ConvertTo-Json -Depth 100 | Set-Content -LiteralPath $jsonPath -Encoding UTF8
$md = @()
$md += "# R0.3A-T4 normalized native differential"
$md += ""
$md += "Historical protocol files contain no monotonic timestamps; normalized traces preserve wall/audit deltas and mark monotonic deltas as `not_recorded`."
$md += ""
$md += "## First post-turn provider event"
$md += ""
foreach($s in $summaries){$md += ('- {0} [{1}]: `{2}`; first output=`{3}`; tools={4}; disconnects={5}; terminal=`{6}`.' -f $s.Name,$s.Kind,$s.FirstProviderEvent,$s.FirstValidOutputTime,$s.TurnRequest.ToolCount,$s.DisconnectCount,$s.TerminalState)}
$md += ""
$md += "## Difference matrix"
$md += ""
foreach($m in $matrix){$md += ('- **{0}** — `{1}`; known-good=[{2}]; failing=[{3}].' -f $m.Dimension,$m.Classification,($m.KnownGood -join ', '),($m.Failing -join ', '))}
$md | Set-Content -LiteralPath $mdPath -Encoding UTF8
Write-Output "wrote $jsonPath and $mdPath"
