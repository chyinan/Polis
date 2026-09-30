$ErrorActionPreference = 'Stop'

$evidence = $PSScriptRoot
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $evidence '..\..\..\..'))
$frontendRoot = Join-Path $repoRoot 'frontend'
$node = 'C:\Program Files\nodejs\node.exe'
$vite = Join-Path $frontendRoot 'node_modules\vite\bin\vite.js'
$stdout = Join-Path $evidence 'frontend.stdout.txt'
$stderr = Join-Path $evidence 'frontend.stderr.txt'

if (-not (Test-Path -LiteralPath $node -PathType Leaf)) { throw 'Node.js runtime is unavailable' }
if (-not (Test-Path -LiteralPath $vite -PathType Leaf)) { throw 'Frontend dependencies are not installed' }

$env:VITE_WORKBENCH_MODE = 'real'
$env:VITE_WORKBENCH_COMPANY_ID = 'r05b-live-1'
$env:VITE_WORKBENCH_API_BASE_URL = '/api/workbench'
$env:VITE_WORKBENCH_BACKEND_URL = 'http://127.0.0.1:8080'
$env:VITE_WORKBENCH_LIVE_UPDATES = ''

$process = Start-Process -FilePath $node -ArgumentList @($vite, '--host', '127.0.0.1', '--port', '4173') -WorkingDirectory $frontendRoot -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
$process.Id | Set-Content -LiteralPath (Join-Path $evidence 'frontend.pid')
Start-Sleep -Seconds 2
$process.Refresh()
$result = [ordered]@{
  pid = $process.Id
  exited = $process.HasExited
  exit_code = if ($process.HasExited) { $process.ExitCode } else { $null }
  mode = $env:VITE_WORKBENCH_MODE
  company_id = $env:VITE_WORKBENCH_COMPANY_ID
  api_base_url = $env:VITE_WORKBENCH_API_BASE_URL
  backend_proxy = $env:VITE_WORKBENCH_BACKEND_URL
  live_updates = 'HTTP refetch; SSE disabled'
}
$result | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'frontend-readiness.json')
if ($process.HasExited) { throw "Vite server exited during readiness: $($process.ExitCode)" }
Write-Output ($result | ConvertTo-Json -Compress)
