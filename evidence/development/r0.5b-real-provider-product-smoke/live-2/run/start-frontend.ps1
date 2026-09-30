$ErrorActionPreference = 'Stop'
$workspace = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..\..\..')).Path
$frontend = Join-Path $workspace 'frontend'
$evidence = $PSScriptRoot
$node = (Get-Command node.exe).Source
$vite = Join-Path $frontend 'node_modules\vite\bin\vite.js'
$pidFile = Join-Path $evidence 'frontend.pid'
if (-not (Test-Path -LiteralPath $vite)) { throw 'Vite entrypoint is missing' }
if (Test-Path -LiteralPath $pidFile) { throw 'LIVE_2 frontend PID record already exists' }

$env:VITE_WORKBENCH_MODE = 'real'
$env:VITE_WORKBENCH_COMPANY_ID = 'r05b-live-2'
$env:VITE_WORKBENCH_API_BASE_URL = '/api/workbench'
$env:VITE_WORKBENCH_BACKEND_URL = 'http://127.0.0.1:8081'
$env:VITE_WORKBENCH_LIVE_UPDATES = ''
$process = Start-Process -FilePath $node -ArgumentList @($vite,'--host','127.0.0.1','--port','4174','--strictPort') -WorkingDirectory $frontend -WindowStyle Hidden -RedirectStandardOutput (Join-Path $evidence 'frontend.stdout.log') -RedirectStandardError (Join-Path $evidence 'frontend.stderr.log') -PassThru
$process.Id | Set-Content -NoNewline -LiteralPath $pidFile
Start-Sleep -Seconds 2
$process.Refresh()
if ($process.HasExited) { throw 'LIVE_2 frontend exited before serving the existing Workbench' }
if (-not (Get-NetTCPConnection -State Listen -LocalPort 4174 -ErrorAction SilentlyContinue)) { throw 'LIVE_2 frontend did not bind its dedicated port' }
[pscustomobject]@{pid=$process.Id; address='http://127.0.0.1:4174/companies/r05b-live-2/overview'; mode='real'; company_id='r05b-live-2'; live_updates='disabled'; backend_proxy='http://127.0.0.1:8081'} | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'frontend-preflight.json')
