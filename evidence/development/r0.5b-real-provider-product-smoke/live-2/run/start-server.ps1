$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..\..')).Path
$evidence = $PSScriptRoot
$runtimePath = 'C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421'
$binary = Join-Path $runtimePath 'codex.exe'
$helper = Join-Path $runtimePath 'codex-code-mode-host.exe'
$authFile = Join-Path $env:USERPROFILE '.codex\auth.json'
$server = Join-Path $evidence 'polis-live2.exe'
$allowance = Join-Path $evidence 'allowance.json'
$outLog = Join-Path $evidence 'server.stdout.log'
$errLog = Join-Path $evidence 'server.stderr.log'
$pidFile = Join-Path $evidence 'server.pid'

if (-not (Test-Path -LiteralPath $server)) { throw 'LIVE_2 server executable is missing' }
if (-not (Test-Path -LiteralPath $binary) -or -not (Test-Path -LiteralPath $helper)) { throw 'B4 Codex runtime binary/helper is missing' }
if (-not (Test-Path -LiteralPath $authFile)) { throw 'local Codex credential source is missing' }
if (Test-Path -LiteralPath $allowance) { throw 'LIVE_2 allowance file already exists; refusing to reuse a prior reservation' }
if (Test-Path -LiteralPath $pidFile) { throw 'LIVE_2 server PID record already exists' }

$actualBinaryHash = (Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant()
$actualHelperHash = (Get-FileHash -LiteralPath $helper -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualBinaryHash -ne '081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575') { throw 'Codex binary does not match the B4 runtime hash' }
if ($actualHelperHash -ne 'fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e') { throw 'Codex helper does not match the B4 runtime hash' }

$dsn = (Get-Content -Raw (Join-Path $evidence 'windows-runtime-dsn.txt')).Trim()
$env:POLIS_DSN = $dsn
$env:POLIS_BLOB_ROOT = Join-Path $evidence 'workspace-cas'
$env:POLIS_WORKER_MODE = 'real'
$env:POLIS_PROVIDER_TRANSPORT = 'codex'
$env:POLIS_PROVIDER_BINARY = $binary
$env:POLIS_PROVIDER_HELPER_BINARY = $helper
$env:POLIS_PROVIDER_BINARY_SHA256 = '081e4de4be8e38fac6ed4d95e3b1a0b9f6d31c090ddc36e1696b349fe406f575'
$env:POLIS_PROVIDER_HELPER_SHA256 = 'fdf360c3a02adce2a29272357d61681bdddc2ab9828a5fa615c4528e4384a23e'
$env:POLIS_PROVIDER_AUTH_FILE = $authFile
$env:POLIS_PROVIDER_ROOT = Join-Path $evidence 'provider-root'
$env:POLIS_PROVIDER_EVIDENCE = Join-Path $evidence 'provider-evidence'
$env:POLIS_PROVIDER_MODEL = 'gpt-5.6-luna'
$env:POLIS_PROVIDER_EFFORT = 'medium'
$env:POLIS_PROVIDER_EXPECTED_VERSION = '0.154.0-alpha.6.2'
$env:POLIS_PROVIDER_PURPOSE = 'R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_2'
$env:POLIS_PROVIDER_EXACT_SURFACE_EXECUTION_FINGERPRINT = 'fecc17cac69231575c13d3e2d767e31560e60ebe8dc2fb20921b925ac8d95f08'
$env:POLIS_PROVIDER_L2_FINGERPRINT = '4c206409f827211cb9f7b18d37da5d096efb7f735c90eaed15e4692128d6a532'
$env:POLIS_PROVIDER_EXECUTION_ENVELOPE = '676052c13ae0380250c4fcb9dbdb3cd8794167e214e6b3766e6e38b4d68bdc51'
$env:POLIS_PROVIDER_TOOL_SURFACE_QUALIFICATION = 'polis-product-tool-surface@2'
$env:POLIS_PROVIDER_ALLOWANCE_PATH = $allowance
$env:POLIS_PROVIDER_TOOL_CALL_LIMIT = '16'
$env:POLIS_WORKBENCH_ADDR = '127.0.0.1:8081'

New-Item -ItemType Directory -Force -Path $env:POLIS_BLOB_ROOT, $env:POLIS_PROVIDER_ROOT, $env:POLIS_PROVIDER_EVIDENCE | Out-Null
$process = Start-Process -FilePath $server -ArgumentList 'serve' -WorkingDirectory $workspace -WindowStyle Hidden -RedirectStandardOutput $outLog -RedirectStandardError $errLog -PassThru
$process.Id | Set-Content -NoNewline -LiteralPath $pidFile
Start-Sleep -Seconds 2
$process.Refresh()
if ($process.HasExited) { throw "LIVE_2 server exited during runtime readiness preflight; inspect sanitized stderr log at $errLog" }
if (-not (Get-NetTCPConnection -State Listen -LocalPort 8081 -ErrorAction SilentlyContinue)) { throw 'LIVE_2 Workbench server did not bind its dedicated port' }
[pscustomobject]@{pid=$process.Id; address='127.0.0.1:8081'; model='gpt-5.6-luna'; effort='medium'; tool_call_limit=16; runtime_version='0.154.0-alpha.6.2'; b4_binary_hash=$actualBinaryHash; b4_helper_hash=$actualHelperHash; purpose='R0.5B_REAL_PROVIDER_PRODUCT_SMOKE_LIVE_2'; allowance_file_created=(Test-Path -LiteralPath $allowance); provider_egress=0} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'server-preflight.json')
