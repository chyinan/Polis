# pattern: Imperative Shell
param(
    [string]$Repo = 'D:\Programs\Polis',
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-backend-l2-v2',
    [string]$OldEvidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l2',
    [string]$L1Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-current-binary-l1',
    [string]$Acceptance = 'D:\Programs\Polis\evidence\development\r0.3a-pagination-acceptance-remediation\qualification.json',
    [string]$Executable = 'D:\Programs\Polis\.runtime\windows\r0.3a-backend-l2-requalification\polis-r03a-backend-l2-requalification.exe'
)

$ErrorActionPreference = 'Stop'
$oldLocation = Get-Location
Set-Location -LiteralPath $Repo
try {
    if (Test-Path -LiteralPath $Evidence) {
        $existing = @(Get-ChildItem -LiteralPath $Evidence -Force)
        if ($existing.Count -ne 0) { throw 'Backend L2 requalification evidence path is not fresh' }
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Executable) | Out-Null
    & bash scripts/build-r03a-backend-l2-requalification-windows.sh
    if ($LASTEXITCODE -ne 0) { throw "Windows build exited with code $LASTEXITCODE" }
    & $Executable -evidence $Evidence -old-evidence $OldEvidence -l1-evidence $L1Evidence -acceptance $Acceptance
    if ($LASTEXITCODE -ne 0) { throw "offline Backend L2 qualification exited with code $LASTEXITCODE" }
    Get-Content -LiteralPath (Join-Path $Evidence 'offline-result.json') -Raw
} finally {
    Set-Location -LiteralPath $oldLocation
}
