# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$paths = @(
  'r0-5b-live3-start-server.ps1',
  'r0-5b-live3-start-frontend.ps1',
  'r0-5b-live3-capture-http-state.ps1',
  'r0-5b-live3-freeze-result.ps1',
  'r0-5b-live3-verify-integrity.ps1',
  'r0-5b15-run-business-preflight.ps1'
)
foreach ($name in $paths) {
  $tokens = $null
  $errors = $null
  [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $name), [ref]$tokens, [ref]$errors) | Out-Null
  if ($errors.Count -gt 0) { throw "PowerShell syntax failed: $name" }
}
'powershell_syntax_pass'
