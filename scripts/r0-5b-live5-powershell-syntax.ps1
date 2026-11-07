# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$paths = @(
  'r0-5b-live5-start-server.ps1',
  'r0-5b-live5-start-frontend.ps1',
  'r0-5b-live5-capture-http-state.ps1',
  'r0-5b-live5-capture-provider-summary.ps1',
  'r0-5b-live5-freeze-result.ps1',
  'r0-5b-live5-post-attempt-validation.ps1',
  'r0-5b-live5-stop-services.ps1'
)
foreach ($name in $paths) {
  $tokens = $null
  $errors = $null
  [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $name), [ref]$tokens, [ref]$errors) | Out-Null
  if ($errors.Count -gt 0) { throw "PowerShell syntax failed: $name" }
}
'LIVE_5_POWERSHELL_SYNTAX_PASS'
