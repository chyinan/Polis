# Derived inventory only. Original per-run JSONL logs remain authoritative.
$ErrorActionPreference = 'Stop'
$evidenceRoot = $PSScriptRoot
$runs = @()
$finalRun = $null
foreach ($directory in (Get-ChildItem ($evidenceRoot + '/postgres-runtime') -Directory | Sort-Object Name)) {
  $summaryPath = Join-Path $directory.FullName 'summary.txt'
  if (!(Test-Path -LiteralPath $summaryPath)) { continue }
  $summary = @(Get-Content -LiteralPath $summaryPath)
  if (!($summary | Where-Object { $_ -like 'postgres_integration_exit=*' })) { continue }
  $runs += [pscustomobject]@{ run=$directory.Name; summary=$summary; original_events=($directory.Name+'/go-postgres-tests.jsonl') }
  if ($summary -contains 'postgres_backed_tests=PASS') { $finalRun=$directory }
}
$runs | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath ($evidenceRoot + '/postgres-run-inventory.json') -Encoding UTF8
if (!$finalRun) { throw 'No successful complete PG-backed run was recorded.' }
# Stream one completed run. Do not retain/re-expand every historical event.
$outcomes = New-Object 'System.Collections.Generic.List[object]'
$outputByTest = @{}
Get-Content -LiteralPath ($finalRun.FullName+'/go-postgres-tests.jsonl') | ForEach-Object {
  $event = $_ | ConvertFrom-Json
  if ($event.Test) {
    $key = $event.Package + '|' + $event.Test
    if ($event.Output) {
      if (!$outputByTest.ContainsKey($key)) { $outputByTest[$key] = New-Object System.Text.StringBuilder }
      [void]$outputByTest[$key].Append($event.Output)
    }
    if ($event.Action -in @('pass','fail','skip')) {
      $outcomes.Add([pscustomobject]@{ package=$event.Package; test=$event.Test; result=$event.Action; elapsed=$event.Elapsed; skip_reason='' })
    }
  }
}
foreach ($row in $outcomes) {
  if ($row.result -eq 'skip') { $row.skip_reason=$outputByTest[$row.package+'|'+$row.test].ToString().Trim() }
}
$outcomes | Export-Csv -LiteralPath ($evidenceRoot + '/postgres-final-tests.csv') -NoTypeInformation -Encoding UTF8
@($outcomes | Where-Object result -eq 'skip') | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath ($evidenceRoot + '/postgres-final-skips.json') -Encoding UTF8
$topLevel = @($outcomes | Where-Object { $_.test -notlike '*/*' })
[pscustomobject]@{ final_run=$finalRun.Name; test_events=$outcomes.Count; passed=@($outcomes | Where-Object result -eq 'pass').Count; failed=@($outcomes | Where-Object result -eq 'fail').Count; skipped=@($outcomes | Where-Object result -eq 'skip').Count; top_level_passed=@($topLevel | Where-Object result -eq 'pass').Count; top_level_skipped=@($topLevel | Where-Object result -eq 'skip').Count } | ConvertTo-Json | Set-Content -LiteralPath ($evidenceRoot + '/postgres-final-summary.json') -Encoding UTF8
Get-Content -LiteralPath ($evidenceRoot + '/postgres-final-summary.json')
