$ErrorActionPreference = 'Stop'

$evidence = $PSScriptRoot
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $evidence '..\..\..\..'))
$runtimeRoot = 'C:\Users\chyinan\AppData\Local\Temp\polis-r05b-live-1-20260917'
$server = Join-Path $runtimeRoot 'polis.exe'
$stdout = Join-Path $evidence 'polis-server.stdout.txt'
$stderr = Join-Path $evidence 'polis-server.stderr.txt'
$pidPath = Join-Path $evidence 'polis-server.pid'
$allowance = Join-Path $evidence 'business-allowance.json'

if (-not (Test-Path -LiteralPath $server -PathType Leaf)) { throw 'LIVE_1 Polis server binary is missing' }
if (-not (Test-Path -LiteralPath 'C:\Users\chyinan\.codex\auth.json' -PathType Leaf)) { throw 'Qualified provider auth source is unavailable' }
if (Test-Path -LiteralPath $allowance) { throw 'LIVE_1 business allowance path already exists' }

$env:POLIS_DSN = 'postgres://polis_runtime@172.24.72.52:55481/polis_r0_5b_live_1?sslmode=disable'
$env:POLIS_BLOB_ROOT = Join-Path $runtimeRoot 'blobstore'
$env:POLIS_WORKBENCH_ADDR = '127.0.0.1:8080'
$env:POLIS_WORKER_MODE = 'real'
$env:POLIS_PROVIDER_TRANSPORT = 'codex'
$env:POLIS_PROVIDER_BINARY = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$env:POLIS_PROVIDER_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
$env:POLIS_PROVIDER_ROOT = Join-Path $runtimeRoot 'provider-runtime'
$env:POLIS_PROVIDER_EVIDENCE = Join-Path $evidence 'provider'
$env:POLIS_PROVIDER_MODEL = 'gpt-5.6-luna'
$env:POLIS_PROVIDER_EFFORT = 'medium'
$env:POLIS_PROVIDER_EXPECTED_VERSION = '0.154.0-alpha.6.2'
$env:POLIS_PROVIDER_EXECUTION_ENVELOPE = 'fecc17cac69231575c13d3e2d767e31560e60ebe8dc2fb20921b925ac8d95f08'
$env:POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION = 'polis-product-tool-surface@2'
$env:POLIS_PROVIDER_ALLOWANCE_PATH = $allowance
$env:POLIS_PROVIDER_TOOL_CALL_LIMIT = '16'

New-Item -ItemType Directory -Path $env:POLIS_BLOB_ROOT, $env:POLIS_PROVIDER_ROOT, $env:POLIS_PROVIDER_EVIDENCE -Force | Out-Null
$process = Start-Process -FilePath $server -ArgumentList 'serve' -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
$process.Id | Set-Content -LiteralPath $pidPath
Start-Sleep -Seconds 2
$process.Refresh()
$result = [ordered]@{
  pid = $process.Id
  exited = $process.HasExited
  exit_code = if ($process.HasExited) { $process.ExitCode } else { $null }
  worker_mode = $env:POLIS_WORKER_MODE
  provider_transport = $env:POLIS_PROVIDER_TRANSPORT
  model = $env:POLIS_PROVIDER_MODEL
  effort = $env:POLIS_PROVIDER_EFFORT
  expected_version = $env:POLIS_PROVIDER_EXPECTED_VERSION
  execution_fingerprint = $env:POLIS_PROVIDER_EXECUTION_ENVELOPE
  product_surface = $env:POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION
  tool_call_limit = [int]$env:POLIS_PROVIDER_TOOL_CALL_LIMIT
  allowance_path_fresh = -not (Test-Path -LiteralPath $allowance)
  authentication_file_present = Test-Path -LiteralPath $env:POLIS_PROVIDER_AUTH_FILE -PathType Leaf
  evidence_root = $env:POLIS_PROVIDER_EVIDENCE
}
$result | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'runtime-readiness.json')
if ($process.HasExited) { throw "Polis server exited during readiness: $($process.ExitCode)" }
Write-Output ($result | ConvertTo-Json -Compress)
