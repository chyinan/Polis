# pattern: Imperative Shell
param([string]$RunRoot = 'preflight')
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = Join-Path $workspace ("evidence\development\r0.5b16-reservation-to-initialize-bridge-hardening\" + $RunRoot)
$testDb = Join-Path $workspace 'evidence\development\r0.5b16-reservation-to-initialize-bridge-hardening\test-db'
$result = Join-Path $evidence 'r0-5b16-result.json'
if (Test-Path -LiteralPath $result) {
  throw 'R0.5B16 preflight evidence already exists; refusing reuse or retry'
}
$dsnPath = Join-Path $testDb 'windows-runtime-dsn.txt'
if (-not (Test-Path -LiteralPath $dsnPath)) {
  throw 'R0.5B16 dedicated PostgreSQL DSN is missing'
}
New-Item -ItemType Directory -Force -Path $evidence | Out-Null
$env:POLIS_DSN = (Get-Content -Raw -LiteralPath $dsnPath).Trim()
$env:POLIS_BLOB_ROOT = Join-Path $evidence 'blobstore'
$env:POLIS_R05B16_EVIDENCE = $evidence
$env:POLIS_PROVIDER_ROOT = Join-Path $evidence 'runtime'
$env:POLIS_PROVIDER_EVIDENCE = Join-Path $evidence 'provider'
$env:POLIS_PROVIDER_RUNTIME_MANIFEST = if ($env:POLIS_PROVIDER_RUNTIME_MANIFEST) { $env:POLIS_PROVIDER_RUNTIME_MANIFEST } else { 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json' }
$env:POLIS_PROVIDER_AUTH_FILE = if ($env:POLIS_PROVIDER_AUTH_FILE) { $env:POLIS_PROVIDER_AUTH_FILE } else { 'C:\Users\chyinan\.codex\auth.json' }
$env:POLIS_PROVIDER_EXECUTION_ENVELOPE = 'dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d'
$env:POLIS_R05B16_LIVE4_RESULT = Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-4\live-4-result.json'
$env:POLIS_R05B16_B15_RESULT = Join-Path $workspace 'evidence\development\r0.5b15-business-path-provider-initialize-hardening\r0-5b15-result.json'
$binary = Join-Path $evidence 'runtime\polis-r05b16.exe'
if (-not (Test-Path -LiteralPath $binary)) {
  throw "R0.5B16 Windows preflight binary is missing: $binary"
}
$stdout = Join-Path $evidence 'preflight.stdout.log'
$stderr = Join-Path $evidence 'preflight.stderr.log'
$process = Start-Process -FilePath $binary -WorkingDirectory $workspace -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
$process.WaitForExit()
$stderrText = [string](Get-Content -Raw -LiteralPath $stderr)
$exitCode = $process.ExitCode
$stderrNonEmpty = -not [string]::IsNullOrWhiteSpace($stderrText)
if (($null -ne $exitCode -and $exitCode -ne 0) -or $stderrNonEmpty) {
  $stderrText
  throw "R0.5B16 reservation-bearing preflight failed with exit code $exitCode"
}
@{exit_code=if($null -eq $exitCode){0}else{$exitCode}; stdout=$stdout; stderr=$stderr; evidence=$result} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'preflight-process-result.json')
