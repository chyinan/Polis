# Generates evidence from recorded Go JSON events; never starts a test or a job.
$ErrorActionPreference = 'Stop'
$evidenceRoot = $PSScriptRoot
$runs = @()
$finalEvents = @()
$finalRun = $null
foreach ($directory in (Get-ChildItem ($evidenceRoot + '/postgres-runtime') -Directory | Sort-Object Name)) {
  $summaryPath = Join-Path $directory.FullName 'summary.txt'
  $eventsPath = Join-Path $directory.FullName 'go-postgres-tests.jsonl'
  if (!(Test-Path -LiteralPath $summaryPath) -or !(Test-Path -LiteralPath $eventsPath)) { continue }
  $summary = Get-Content -LiteralPath $summaryPath
  if (!($summary | Where-Object { $_ -like 'postgres_integration_exit=*' })) { continue }
  $events = @(Get-Content -LiteralPath $eventsPath | ForEach-Object { $_ | ConvertFrom-Json })
  $outcomes = @($events | Where-Object { $_.Test -and $_.Action -in @('pass','fail','skip') })
  $failures = @($outcomes | Where-Object Action -eq 'fail' | ForEach-Object {
    $failure = $_
    [pscustomobject]@{ package=$failure.Package; test=$failure.Test; output=(($events | Where-Object { $_.Package -eq $failure.Package -and $_.Test -eq $failure.Test -and $_.Output } | ForEach-Object Output) -join '') }
  })
  $runs += [pscustomobject]@{ run=$directory.Name; summary=$summary; passed=@($outcomes | Where-Object Action -eq 'pass').Count; failed=$failures.Count; skipped=@($outcomes | Where-Object Action -eq 'skip').Count; failures=$failures }
  if ($summary -contains 'postgres_backed_tests=PASS') { $finalRun=$directory.Name; $finalEvents=$events }
}
$runs | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath ($evidenceRoot + '/postgres-run-inventory.json') -Encoding UTF8
if (!$finalRun) { throw 'No successful complete PG-backed suite was recorded; no PASS inventory generated.' }
$outcomes = @($finalEvents | Where-Object { $_.Test -and $_.Action -in @('pass','fail','skip') })
$rows = @($outcomes | ForEach-Object {
  $test = $_
  $reason = ''
  if ($test.Action -eq 'skip') { $reason=(($finalEvents | Where-Object { $_.Test -eq $test.Test -and $_.Package -eq $test.Package -and $_.Output } | ForEach-Object Output) -join '').Trim() }
  [pscustomobject]@{ package=$test.Package; test=$test.Test; result=$test.Action; elapsed=$test.Elapsed; skip_reason=$reason }
})
$rows | Export-Csv -LiteralPath ($evidenceRoot + '/postgres-final-tests.csv') -NoTypeInformation -Encoding UTF8
$rows | Where-Object result -eq 'skip' | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath ($evidenceRoot + '/postgres-final-skips.json') -Encoding UTF8
$topLevel = @($rows | Where-Object { $_.test -notlike '*/*' })
[pscustomobject]@{ final_run=$finalRun; test_events=$rows.Count; passed=@($rows | Where-Object result -eq 'pass').Count; failed=@($rows | Where-Object result -eq 'fail').Count; skipped=@($rows | Where-Object result -eq 'skip').Count; top_level_passed=@($topLevel | Where-Object result -eq 'pass').Count; top_level_skipped=@($topLevel | Where-Object result -eq 'skip').Count } | ConvertTo-Json | Set-Content -LiteralPath ($evidenceRoot + '/postgres-final-summary.json') -Encoding UTF8
Get-Content -LiteralPath ($evidenceRoot + '/postgres-final-summary.json')
