$ErrorActionPreference = 'Stop'

$evidence = $PSScriptRoot
$expected = 'C:\Users\chyinan\AppData\Local\Temp\polis-r05b-live-1-20260917'
$target = [System.IO.Path]::GetFullPath($expected)
if ($target -cne $expected) { throw "Refusing to remove unexpected temporary path: $target" }
if (Test-Path -LiteralPath $target) {
  Remove-Item -LiteralPath $target -Recurse -Force
}
$result = [ordered]@{target = $target; removed = -not (Test-Path -LiteralPath $target)}
$result | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $evidence 'windows-temp-cleanup.json')
Write-Output ($result | ConvertTo-Json -Compress)
