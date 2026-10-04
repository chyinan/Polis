[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$BinaryPath,
    [switch]$CheckOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-SigningContext {
    $thumbprint = ($env:POLIS_WINDOWS_SIGNING_CERT_THUMBPRINT -replace '\s', '').ToUpperInvariant()
    if ($thumbprint -notmatch '^[0-9A-F]{40}$') {
        throw 'POLIS_WINDOWS_SIGNING_CERT_THUMBPRINT must be the 40-character SHA-1 thumbprint of the publisher certificate.'
    }
    $timestampText = $env:POLIS_WINDOWS_TIMESTAMP_URL
    $timestampUri = $null
    if ([string]::IsNullOrWhiteSpace($timestampText) -or -not [System.Uri]::TryCreate($timestampText, [System.UriKind]::Absolute, [ref]$timestampUri) -or
        $timestampUri.Scheme -notin @('http', 'https') -or [string]::IsNullOrWhiteSpace($timestampUri.Host) -or $timestampUri.UserInfo -ne '') {
        throw 'POLIS_WINDOWS_TIMESTAMP_URL must be an absolute HTTP(S) timestamp URL without embedded credentials.'
    }

    $certificatePath = "Cert:\CurrentUser\My\$thumbprint"
    $certificate = Get-Item -LiteralPath $certificatePath -ErrorAction Stop
    if (-not $certificate.HasPrivateKey) {
        throw 'The selected signing certificate has no private key available to the current Windows user.'
    }
    $now = [DateTime]::UtcNow
    if ($now -lt $certificate.NotBefore.ToUniversalTime() -or $now -ge $certificate.NotAfter.ToUniversalTime()) {
        throw 'The selected signing certificate is outside its validity period.'
    }
    $eku = $certificate.Extensions |
        Where-Object { $_ -is [System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension] } |
        Select-Object -First 1
    $hasCodeSigningUsage = $false
    if ($null -ne $eku) {
        $hasCodeSigningUsage = @($eku.EnhancedKeyUsages | Where-Object { $_.Value -eq '1.3.6.1.5.5.7.3.3' }).Count -gt 0
    }
    if (-not $hasCodeSigningUsage) {
        throw 'The selected certificate does not have the Code Signing extended key usage.'
    }

    $signTool = Get-Command 'signtool.exe' -ErrorAction SilentlyContinue
    if ($null -eq $signTool) {
        $kitsRoot = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
        $candidates = @(Get-ChildItem -Path (Join-Path $kitsRoot '*\x64\signtool.exe') -File -ErrorAction SilentlyContinue)
        if ($candidates.Count -eq 0) {
            throw 'signtool.exe is unavailable; install the Windows SDK signing tools or add signtool.exe to PATH.'
        }
        $signTool = $candidates | Sort-Object -Property @{Expression = { [Version]$_.Directory.Parent.Name }; Descending = $true} | Select-Object -First 1
    }
    $signToolPath = if ($signTool -is [System.IO.FileInfo]) { $signTool.FullName } else { $signTool.Source }

    return @{
        Thumbprint = $thumbprint
        SignTool = $signToolPath
        TimestampUrl = $timestampUri.AbsoluteUri
    }
}

try {
    $signing = Get-SigningContext
    if ($CheckOnly) {
        Write-Output 'Windows publisher certificate and SignTool preflight passed.'
        exit 0
    }
    if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
        throw 'A binary path is required.'
    }
    if (-not (Test-Path -LiteralPath $BinaryPath -PathType Leaf)) {
        throw 'The binary to sign does not exist or is not a regular file.'
    }
    $resolvedPath = (Resolve-Path -LiteralPath $BinaryPath).Path
    & $signing.SignTool sign /sha1 $signing.Thumbprint /fd SHA256 /tr $signing.TimestampUrl /td SHA256 /v $resolvedPath
    if ($LASTEXITCODE -ne 0) {
        throw "SignTool signing failed with exit code $LASTEXITCODE."
    }
    & $signing.SignTool verify /pa /all /v $resolvedPath
    if ($LASTEXITCODE -ne 0) {
        throw "SignTool Authenticode verification failed with exit code $LASTEXITCODE."
    }
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
