# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'

$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-3')).Path
$runtime = (Resolve-Path (Join-Path $evidence 'runtime')).Path
$server = Join-Path $runtime 'polis-live3.exe'
$binary = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe'
$helper = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe'
$manifest = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json'
$allowance = Join-Path $evidence 'allowance.json'
$stdout = Join-Path $evidence 'polis-server.stdout.log'
$stderr = Join-Path $evidence 'polis-server.stderr.log'
$pidPath = Join-Path $evidence 'polis-server.pid'

function Get-Sha256([string]$path) {
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $stream = [System.IO.File]::OpenRead($path)
    try {
      return ([System.BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-', '').ToLowerInvariant()
    } finally {
      $stream.Dispose()
    }
  } finally {
    $sha.Dispose()
  }
}

if (-not (Test-Path -LiteralPath $server)) { throw 'LIVE_3 server executable is missing' }
if (-not (Test-Path -LiteralPath $binary) -or -not (Test-Path -LiteralPath $helper) -or -not (Test-Path -LiteralPath $manifest)) { throw 'LIVE_3 controlled provider runtime files are missing' }
if (Test-Path -LiteralPath $allowance) { throw 'LIVE_3 allowance file already exists; refusing reuse' }
if (Test-Path -LiteralPath $pidPath) { throw 'LIVE_3 server PID record already exists' }
if (Get-NetTCPConnection -State Listen -LocalPort 8092 -ErrorAction SilentlyContinue) { throw 'LIVE_3 Workbench port 8092 is occupied' }

$binaryHash = Get-Sha256 $binary
$helperHash = Get-Sha256 $helper
if ($binaryHash -ne '081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575') { throw 'LIVE_3 provider binary hash drifted' }
if ($helperHash -ne 'fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e') { throw 'LIVE_3 provider helper hash drifted' }

$env:POLIS_DSN = (Get-Content -Raw (Join-Path $evidence 'windows-runtime-dsn.txt')).Trim()
$env:POLIS_BLOB_ROOT = Join-Path $evidence 'cas'
$env:POLIS_WORKER_MODE = 'real'
$env:POLIS_PROVIDER_TRANSPORT = 'codex'
$env:POLIS_PROVIDER_BINARY = $binary
$env:POLIS_PROVIDER_HELPER_BINARY = $helper
$env:POLIS_PROVIDER_RUNTIME_MANIFEST = $manifest
$env:POLIS_PROVIDER_BINARY_SHA256 = $binaryHash
$env:POLIS_PROVIDER_HELPER_SHA256 = $helperHash
$env:POLIS_PROVIDER_AUTH_FILE = 'C:\Users\chyinan\.codex\auth.json'
$env:POLIS_PROVIDER_ROOT = Join-Path $evidence 'provider-runtime'
$env:POLIS_PROVIDER_EVIDENCE = Join-Path $evidence 'provider-evidence'
$env:POLIS_PROVIDER_MODEL = 'gpt-5.6-luna'
$env:POLIS_PROVIDER_EFFORT = 'medium'
$env:POLIS_PROVIDER_EXPECTED_VERSION = '0.154.0-alpha.6.2'
$env:POLIS_PROVIDER_PURPOSE = 'R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_3'
$env:POLIS_PROVIDER_EXACT_SURFACE_EXECUTION_FINGERPRINT = 'e736d8298f0920c6dd81796006b2486881647f0fdbc66bb111bbd2387a1e99ff'
$env:POLIS_PROVIDER_L2_FINGERPRINT = '59a4ac003fb0f49360382a707f7057edc03e0574ba328d0cf3a59eeab2726781'
$env:POLIS_PROVIDER_EXECUTION_ENVELOPE = 'dfa3a3d252b2b781e47f932ff350e5eb99f0d6ac0112c1153531248dee0df24d'
$env:POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION = 'polis-product-tool-surface@4'
$env:POLIS_PROVIDER_ALLOWANCE_PATH = $allowance
$env:POLIS_PROVIDER_TOOL_CALL_LIMIT = '16'
$env:POLIS_WORKBENCH_ADDR = '127.0.0.1:8092'

New-Item -ItemType Directory -Force -Path $env:POLIS_BLOB_ROOT, $env:POLIS_PROVIDER_ROOT, $env:POLIS_PROVIDER_EVIDENCE | Out-Null
$process = Start-Process -FilePath $server -ArgumentList 'serve' -WorkingDirectory $workspace -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
$process.Id | Set-Content -NoNewline -LiteralPath $pidPath
Start-Sleep -Seconds 3
$process.Refresh()
$listening = [bool](Get-NetTCPConnection -State Listen -LocalPort 8092 -ErrorAction SilentlyContinue)
[pscustomobject]@{pid=$process.Id; listening=$listening; exited=$process.HasExited; allowance_exists=(Test-Path -LiteralPath $allowance); address='127.0.0.1:8092'; mode=$env:POLIS_WORKER_MODE; purpose=$env:POLIS_PROVIDER_PURPOSE; surface=$env:POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION; execution_fingerprint=$env:POLIS_PROVIDER_EXACT_SURFACE_EXECUTION_FINGERPRINT; provider_l2=$env:POLIS_PROVIDER_L2_FINGERPRINT; execution_envelope=$env:POLIS_PROVIDER_EXECUTION_ENVELOPE; binary_sha256=$binaryHash; helper_sha256=$helperHash} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'server-preflight.json')
if ($process.HasExited -or -not $listening) {
  Get-Content -Raw $stderr
  throw 'LIVE_3 server readiness failed'
}
