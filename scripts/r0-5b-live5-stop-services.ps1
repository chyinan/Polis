# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-5'
$stopped = @()
foreach ($name in @('polis-server.pid','frontend.pid')) {
  $path = Join-Path $evidence $name
  if (Test-Path -LiteralPath $path) {
    $processId = [int](Get-Content -Raw $path).Trim()
    $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if ($null -ne $process) {
      Stop-Process -Id $processId -Force
      $stopped += [pscustomobject]@{pid=$processId; record=$name; stopped=$true}
    } else {
      $stopped += [pscustomobject]@{pid=$processId; record=$name; stopped=$false; already_exited=$true}
    }
  }
}
$stopped | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $evidence 'local-services-stop.json')
Write-Output 'LIVE_5_LOCAL_SERVICES_STOPPED'
