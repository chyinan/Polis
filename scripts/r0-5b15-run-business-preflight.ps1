# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b15-business-path-provider-initialize-hardening\business-preflight')).Path
$testDb = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b15-business-path-provider-initialize-hardening\test-db-2')).Path
$env:POLIS_DSN = (Get-Content -Raw (Join-Path $testDb 'windows-runtime-dsn.txt')).Trim()
$env:POLIS_BLOB_ROOT = Join-Path $evidence 'blobstore'
$env:POLIS_R05B15_EVIDENCE = $evidence
$env:POLIS_PROVIDER_ROOT = Join-Path $evidence 'runtime'
$env:POLIS_PROVIDER_EVIDENCE = Join-Path $evidence 'provider'
$env:POLIS_PROVIDER_RUNTIME_MANIFEST = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'
$env:POLIS_PROVIDER_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
$env:POLIS_PROVIDER_EXECUTION_ENVELOPE = 'dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d'
$binary = Join-Path $evidence 'runtime\polis-r05b15.exe'
$stdout = Join-Path $evidence 'preflight.stdout.log'
$stderr = Join-Path $evidence 'preflight.stderr.log'
$process = Start-Process -FilePath $binary -WorkingDirectory $workspace -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
$process.WaitForExit()
if ($null -ne $process.ExitCode -and $process.ExitCode -ne 0) {
  Get-Content -Raw $stderr
  throw "R0.5B15 business preflight failed with exit code $($process.ExitCode)"
}
@{exit_code=if($null -eq $process.ExitCode){0}else{$process.ExitCode}; stdout=$stdout; stderr=$stderr; evidence=Join-Path $evidence 'business-context-preflight.json'} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'preflight-process-result.json')
