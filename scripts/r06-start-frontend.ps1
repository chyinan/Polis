$env:VITE_WORKBENCH_COMPANY_ID = 'browser-company'
$env:VITE_WORKBENCH_LIVE_UPDATES = 'deferred'
$env:VITE_WORKBENCH_BACKEND_URL = 'http://127.0.0.1:8081'
Set-Location 'D:\Programs\Polis\frontend'
& npm run dev -- --host 127.0.0.1
