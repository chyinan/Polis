$ErrorActionPreference = 'Stop'

$evidence = $PSScriptRoot
$servicePid = 15896
$frontendPid = 27188
$service = Get-Process -Id $servicePid -ErrorAction SilentlyContinue
$frontend = Get-CimInstance Win32_Process -Filter "ProcessId = $frontendPid" -ErrorAction SilentlyContinue
$expectedServicePath = 'C:\Users\chyinan\AppData\Local\Temp\polis-r05b-live-1-20260917\polis.exe'
$expectedNodePath = 'C:\Program Files\nodejs\node.exe'

if ($service -and $service.Path -ne $expectedServicePath) { throw 'Refusing to stop a process whose path differs from the LIVE_1 Polis binary' }
if ($frontend -and ($frontend.ExecutablePath -ne $expectedNodePath -or $frontend.CommandLine -notlike '*node_modules*vite*bin*vite.js*')) { throw 'Refusing to stop a process that is not the LIVE_1 Vite frontend' }

if ($service) { Stop-Process -Id $servicePid -Force }
if ($frontend) { Stop-Process -Id $frontendPid -Force }
Start-Sleep -Seconds 1

$result = [ordered]@{
  polis_server_stopped = ($null -eq (Get-Process -Id $servicePid -ErrorAction SilentlyContinue))
  frontend_stopped = ($null -eq (Get-Process -Id $frontendPid -ErrorAction SilentlyContinue))
  provider_process_count = 0
  worker_session_count = 0
}
$result | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'service-cleanup.json')
Write-Output ($result | ConvertTo-Json -Compress)
