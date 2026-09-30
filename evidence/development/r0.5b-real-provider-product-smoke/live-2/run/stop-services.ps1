$ErrorActionPreference = 'Stop'
$evidence = $PSScriptRoot
foreach ($name in @('frontend','server')) {
  $pidFile = Join-Path $evidence ($name + '.pid')
  if (-not (Test-Path -LiteralPath $pidFile)) { continue }
  $processId = [int](Get-Content -Raw -LiteralPath $pidFile)
  $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
  if ($null -ne $process) { Stop-Process -Id $processId -Force; Wait-Process -Id $processId -Timeout 10 -ErrorAction SilentlyContinue }
  Remove-Item -LiteralPath $pidFile
}
$listeners = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in @(8081,4174) })
if ($listeners.Count -gt 0) { throw 'LIVE_2 frontend/backend listener remains after stopping the recorded processes' }
[pscustomobject]@{server='stopped';frontend='stopped';backend_port=8081;frontend_port=4174} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'services-cleanup.json')
