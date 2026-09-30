# pattern: Imperative Shell
$ErrorActionPreference = 'Stop'

$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$evidence = (Resolve-Path (Join-Path $workspace 'evidence\development\r0.5b-real-provider-product-smoke-live-3')).Path
if (Get-NetTCPConnection -State Listen -LocalPort 4182 -ErrorAction SilentlyContinue) { throw 'LIVE_3 frontend port 4182 is occupied' }

$env:VITE_WORKBENCH_MODE = 'real'
$env:VITE_WORKBENCH_COMPANY_ID = 'r05b-live-3'
$env:VITE_WORKBENCH_API_BASE_URL = '/api/workbench'
$env:VITE_WORKBENCH_BACKEND_URL = 'http://127.0.0.1:8092'
$env:VITE_WORKBENCH_LIVE_UPDATES = 'false'
$stdout = Join-Path $evidence 'frontend.stdout.log'
$stderr = Join-Path $evidence 'frontend.stderr.log'
$pidPath = Join-Path $evidence 'frontend.pid'

$process = Start-Process -FilePath 'npm.cmd' -ArgumentList @('run', 'dev', '--', '--host', '127.0.0.1', '--port', '4182') -WorkingDirectory (Join-Path $workspace 'frontend') -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
$process.Id | Set-Content -NoNewline -LiteralPath $pidPath
Start-Sleep -Seconds 3
$process.Refresh()
$listening = [bool](Get-NetTCPConnection -State Listen -LocalPort 4182 -ErrorAction SilentlyContinue)
[pscustomobject]@{pid=$process.Id; listening=$listening; exited=$process.HasExited; url='http://127.0.0.1:4182'; mode=$env:VITE_WORKBENCH_MODE; company=$env:VITE_WORKBENCH_COMPANY_ID; api=$env:VITE_WORKBENCH_API_BASE_URL; sse=$env:VITE_WORKBENCH_LIVE_UPDATES} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'frontend-preflight.json')
if ($process.HasExited -or -not $listening) {
  Get-Content -Raw $stderr
  throw 'LIVE_3 frontend readiness failed'
}
