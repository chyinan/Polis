$ErrorActionPreference = 'Stop'
$authPath = Join-Path $env:USERPROFILE '.codex\auth.json'
if (-not (Test-Path -LiteralPath $authPath -PathType Leaf)) {
  @{ auth_file_present = $false } | ConvertTo-Json -Compress
  exit 1
}
$raw = [System.IO.File]::ReadAllText($authPath)
try {
  $document = ConvertFrom-Json -InputObject $raw
} catch {
  @{ auth_file_present = $true; valid_json = $false; byte_length = $raw.Length } | ConvertTo-Json -Compress
  exit 1
}
$tokens = $document.tokens
$result = [ordered]@{
  auth_file_present = $true
  valid_json = $true
  byte_length = $raw.Length
  auth_mode = $document.auth_mode
  top_level_keys = @($document.PSObject.Properties.Name)
  token_fields = if ($tokens) { @($tokens.PSObject.Properties.Name) } else { @() }
  id_token_present = [bool]($tokens.id_token)
  access_token_present = [bool]($tokens.access_token)
  refresh_token_present = [bool]($tokens.refresh_token)
  api_key_present = [bool]$document.OPENAI_API_KEY
}
$result | ConvertTo-Json -Compress
