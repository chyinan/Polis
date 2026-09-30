param(
    [string]$RepositoryRoot,
    [switch]$InitializeManifest
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($RepositoryRoot)) {
    $RepositoryRoot = Split-Path -Parent $PSScriptRoot
}
$repositoryRoot = [System.IO.Path]::GetFullPath($RepositoryRoot)
$migrationDirectory = Join-Path $repositoryRoot 'db/migrations'
$manifestPath = Join-Path $repositoryRoot 'db/migration_hashes.sha256'
if (-not (Test-Path -LiteralPath $migrationDirectory -PathType Container)) {
    throw "Migration directory does not exist: $migrationDirectory"
}

$pinnedHashes = @{}
$pinnedVersions = @{}
if (Test-Path -LiteralPath $manifestPath -PathType Leaf) {
    $existingLines = [System.IO.File]::ReadAllLines($manifestPath)
    if ($existingLines.Count -eq 0) {
        throw 'Existing migration hash manifest is empty; refusing to replace it.'
    }
    $lastName = ''
    foreach ($line in $existingLines) {
        $match = [System.Text.RegularExpressions.Regex]::Match($line, '^(?<hash>[0-9a-f]{64})  (?<name>[0-9]{5}_[A-Za-z0-9._-]+\.sql)$')
        if (-not $match.Success) {
            throw 'Existing migration hash manifest is malformed; refusing to replace it.'
        }
        $name = $match.Groups['name'].Value
        if ($name -le $lastName -or $pinnedHashes.ContainsKey($name)) {
            throw 'Existing migration hash manifest is unsorted or contains duplicate names; refusing to replace it.'
        }
        $version = [int]$name.Substring(0, 5)
        if ($version -le 0) {
            throw "Existing manifest has nonpositive Goose version $version."
        }
        if ($pinnedVersions.ContainsKey($version)) {
            throw "Existing migration hash manifest contains duplicate Goose version $version."
        }
        $lastName = $name
        $pinnedHashes[$name] = $match.Groups['hash'].Value
        $pinnedVersions[$version] = $true
    }
}
elseif (-not $InitializeManifest) {
    throw 'Migration hash manifest is missing. Refusing to bootstrap pins; use -InitializeManifest only for an intentional initial baseline.'
}

$migrationFiles = @(Get-ChildItem -LiteralPath $migrationDirectory -File | Where-Object { $_.Name -match '\.sql$' } | Sort-Object -Property Name)
if ($migrationFiles.Count -eq 0) {
    throw 'No Goose migration SQL files were found.'
}
$currentNames = @{}
$currentVersions = @{}
$highestPinnedVersion = 0
if ($pinnedVersions.Count -gt 0) {
    $highestPinnedVersion = [int](($pinnedVersions.Keys | Measure-Object -Maximum).Maximum)
}
foreach ($migrationFile in $migrationFiles) {
    if ($migrationFile.Name -cnotmatch '^[0-9]{5}_[A-Za-z0-9._-]+\.sql$') {
        throw "Migration file name is not canonical: $($migrationFile.Name)"
    }
    $version = [int]$migrationFile.Name.Substring(0, 5)
    if ($version -le 0) {
        throw "Migration file uses nonpositive Goose version $version."
    }
    if ($currentVersions.ContainsKey($version)) {
        throw "Multiple migration files use Goose version $version."
    }
    if (-not $pinnedHashes.ContainsKey($migrationFile.Name) -and $pinnedHashes.Count -gt 0 -and $version -le $highestPinnedVersion) {
        throw "New migration '$($migrationFile.Name)' is not forward of pinned Goose version $highestPinnedVersion."
    }
    $currentVersions[$version] = $true
    $currentNames[$migrationFile.Name] = $true
}
foreach ($pinnedName in $pinnedHashes.Keys) {
    if (-not $currentNames.ContainsKey($pinnedName)) {
        throw "Pinned migration '$pinnedName' was removed or renamed; add a forward migration instead."
    }
}

$sha256Algorithm = [System.Security.Cryptography.SHA256]::Create()
try {
    $manifestEntries = @(
        foreach ($migrationFile in $migrationFiles) {
            $stream = [System.IO.File]::OpenRead($migrationFile.FullName)
            try {
                $digest = $sha256Algorithm.ComputeHash($stream)
            }
            finally {
                $stream.Dispose()
            }
            $hash = [System.BitConverter]::ToString($digest).Replace('-', '').ToLowerInvariant()
            if ($pinnedHashes.ContainsKey($migrationFile.Name) -and $pinnedHashes[$migrationFile.Name] -ne $hash) {
                throw "Pinned migration '$($migrationFile.Name)' has changed; add a new forward migration instead of rehashing history."
            }
            "$hash  $($migrationFile.Name)"
        }
    )

    $content = ($manifestEntries -join "`n") + "`n"
    [System.IO.File]::WriteAllBytes($manifestPath, [System.Text.Encoding]::UTF8.GetBytes($content))
    $newEntries = $manifestEntries.Count - $pinnedHashes.Count
    Write-Output "Verified $($pinnedHashes.Count) existing pins and added $newEntries forward migration hashes. Review the manifest diff before migration."
}
finally {
    $sha256Algorithm.Dispose()
}
