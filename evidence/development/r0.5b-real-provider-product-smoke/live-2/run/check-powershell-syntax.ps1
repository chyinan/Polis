$ErrorActionPreference = 'Stop'
$evidence = $PSScriptRoot
$workspace = (Resolve-Path (Join-Path $evidence '..\..\..\..\..')).Path
$paths = @(
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/start-server.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/start-frontend.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/stop-services.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/freeze-post-turn-evidence.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/finalize-live2-result.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/verify-final-integrity.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/run/check-powershell-syntax.ps1',
  'evidence/development/r0.5b-real-provider-product-smoke/live-2/preflight-tests/inspect-auth-shape.ps1'
)
$results = foreach ($relative in $paths) {
  $tokens = $null
  $parseErrors = $null
  [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $workspace $relative),[ref]$tokens,[ref]$parseErrors) | Out-Null
  [pscustomobject]@{path=$relative;errors=@($parseErrors).Count}
}
if (@($results | Where-Object { $_.errors -ne 0 }).Count -gt 0) { throw 'At least one LIVE_2 PowerShell script has a parse error' }
$report = [pscustomobject]@{result='PASS';scripts=$results}
$json = ConvertTo-Json -InputObject $report -Depth 10
[IO.File]::WriteAllText((Join-Path $evidence 'powershell-syntax.json'),$json + "`n",[Text.UTF8Encoding]::new($false))
$report | ConvertTo-Json -Compress
