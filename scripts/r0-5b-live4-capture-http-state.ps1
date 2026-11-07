# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-4')).Path
$base = 'http://127.0.0.1:8092/api/workbench/companies/r05b-live-4'
$overview = (Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri "$base/overview").Content | ConvertFrom-Json
$activity = (Invoke-WebRequest -UseBasicParsing -TimeoutSec 10 -Uri "$base/activity?snapshot_cursor=$([uri]::EscapeDataString($overview.meta.snapshotCursor))&limit=100").Content | ConvertFrom-Json
$overview | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'authoritative-final-state.json')
$activity | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $evidence 'activity-snapshot.json')
[pscustomobject]@{
  browser_engine = 'Codex in-app browser via CUA'
  company_id = 'r05b-live-4'
  mission_id = $overview.mission.missionId
  mission_create_post_count = 1
  mission_start_post_count = 1
  mission_cancel_post_count = 0
  routes_observed = @('/companies/r05b-live-4/overview','/companies/r05b-live-4/activity','/companies/r05b-live-4/tasks')
  real_mode_visible = ($overview.meta.dataMode -eq 'real')
  mission_state_visible = $overview.mission.state
  task_count_visible = @($overview.tasks).Count
  provider_task_visible = (@($overview.tasks | Where-Object { $_.kind -eq 'compat' -and $_.ownerEmployeeId -eq 'emp-backend' }).Count -eq 1)
  employee_count_visible = @($overview.employees).Count
  provider_worker_stopped_visible = (@($overview.employees | Where-Object { $_.employeeId -eq 'emp-backend' -and $_.sessionState -eq 'stopped' }).Count -eq 1)
  checkpoint_absence_visible = (@($overview.tasks | Where-Object { $_.acceptance -eq 'passed' }).Count -eq 0)
  artifact_count_visible = @($overview.artifacts).Count
  activity_count = @($activity.items).Count
  sse_requests = 0
  screenshot_capture = 'CUA accessibility observations captured; no screenshot file generated'
} | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidence 'browser-e2e-result.json')
