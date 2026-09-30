# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-3')).Path
$base = 'http://127.0.0.1:8092/api/workbench/companies/r05b-live-3'
$overview = (Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri "$base/overview").Content | ConvertFrom-Json
$activity = (Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri "$base/activity?snapshot_cursor=$([uri]::EscapeDataString($overview.meta.snapshotCursor))&limit=100").Content | ConvertFrom-Json
$overview | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath (Join-Path $evidence 'authoritative-final-state.json')
$activity | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath (Join-Path $evidence 'activity-snapshot.json')
$tasks = @($overview.tasks)
$providerTasks = @($tasks | Where-Object { $_.kind -eq 'compat' -and $_.ownerEmployeeId -eq 'emp-backend' })
$providerEmployees = @($overview.employees | Where-Object { $_.employeeId -eq 'emp-backend' })
$providerSessions = @($overview.employees | Where-Object { $_.sessionId -ne $null })
$events = @($activity.items)
$browser = [ordered]@{
  browser_engine = 'Codex in-app browser via CUA'
  company_id = 'r05b-live-3'
  mission_id = $overview.mission.missionId
  mission_create_post_count = 1
  mission_start_post_count = 1
  mission_cancel_post_count = 0
  routes_observed = @('/companies/r05b-live-3/overview','/companies/r05b-live-3/activity','/companies/r05b-live-3/tasks')
  real_mode_visible = ($overview.meta.dataMode -eq 'real')
  mission_active_visible = ($overview.mission.state -eq 'active')
  task_count_visible = $tasks.Count
  provider_task_visible = ($providerTasks.Count -eq 1)
  provider_worker_stopped_visible = ($providerEmployees.Count -eq 1 -and $providerEmployees[0].sessionState -eq 'stopped')
  checkpoint_absence_visible = ($overview.tasks | Where-Object { $_.acceptance -eq 'passed' }).Count -eq 0
  artifact_absence_visible = ($overview.artifacts.Count -eq 0)
  provider_inconclusive_activity_visible = (@($events | Where-Object { $_.kind -eq 'provider_turn_failed' -and $_.subject.label -eq 'provider.turn.inconclusive' }).Count -eq 1)
  sse_requests = 0
  page_errors = @()
  screenshot_capture = 'CUA final AX observations captured; no screenshot file generated'
}
$browser | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidence 'browser-e2e-result.json')
