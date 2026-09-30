$ErrorActionPreference = 'Stop'

$temporaryParent = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$testRoot = Join-Path $temporaryParent ("polis-migration-manifest-test-" + [guid]::NewGuid().ToString('N'))
$resolvedTestRoot = [System.IO.Path]::GetFullPath($testRoot)
$parentPrefix = $temporaryParent.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
if (-not $resolvedTestRoot.StartsWith($parentPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'Temporary migration manifest test directory escaped the temp root.'
}

$updaterPath = Join-Path $PSScriptRoot 'update-migration-hash-manifest.ps1'
$migrationDirectory = Join-Path $resolvedTestRoot 'db/migrations'
$manifestPath = Join-Path $resolvedTestRoot 'db/migration_hashes.sha256'
New-Item -ItemType Directory -Path $migrationDirectory -Force | Out-Null
$firstPath = Join-Path $migrationDirectory '00037_first.sql'
[System.IO.File]::WriteAllText($firstPath, "SELECT 1;`n", [System.Text.Encoding]::UTF8)

try {
    & $updaterPath -RepositoryRoot $resolvedTestRoot -InitializeManifest | Out-Null
    $pinnedBytes = [System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes($manifestPath))

    [System.IO.File]::WriteAllText($firstPath, "SELECT 2;`n", [System.Text.Encoding]::UTF8)
    $changedMigrationRejected = $false
    try {
        & $updaterPath -RepositoryRoot $resolvedTestRoot | Out-Null
    }
    catch {
        $changedMigrationRejected = $true
    }
    if (-not $changedMigrationRejected) {
        throw 'Manifest updater accepted an edit to an already-pinned migration.'
    }
    if ([System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes($manifestPath)) -ne $pinnedBytes) {
        throw 'Manifest updater changed the pinned manifest after refusing migration drift.'
    }

    [System.IO.File]::WriteAllText($firstPath, "SELECT 1;`n", [System.Text.Encoding]::UTF8)
    [System.IO.File]::WriteAllText((Join-Path $migrationDirectory '00039_second.sql'), "SELECT 2;`n", [System.Text.Encoding]::UTF8)
    & $updaterPath -RepositoryRoot $resolvedTestRoot | Out-Null
    $entries = [System.IO.File]::ReadAllLines($manifestPath)
    if ($entries.Count -ne 2) {
        throw "Manifest updater added $($entries.Count) entries after a forward migration; expected 2."
    }
	$pinnedBytes = [System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes($manifestPath))

	$middleVersionRejected = $false
	try {
		[System.IO.File]::WriteAllText((Join-Path $migrationDirectory '00038_middle.sql'), "SELECT 38;`n", [System.Text.Encoding]::UTF8)
		& $updaterPath -RepositoryRoot $resolvedTestRoot | Out-Null
	}
	catch {
		$middleVersionRejected = $true
	}
	if (-not $middleVersionRejected -or [System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes($manifestPath)) -ne $pinnedBytes) {
		throw 'Manifest updater did not reject a migration inserted below the highest pinned version.'
	}
	Remove-Item -LiteralPath (Join-Path $migrationDirectory '00038_middle.sql') -Force

	$duplicateVersionRejected = $false
	try {
		[System.IO.File]::WriteAllText((Join-Path $migrationDirectory '00039_duplicate.sql'), "SELECT 39;`n", [System.Text.Encoding]::UTF8)
		& $updaterPath -RepositoryRoot $resolvedTestRoot | Out-Null
	}
	catch {
		$duplicateVersionRejected = $true
	}
	if (-not $duplicateVersionRejected -or [System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes($manifestPath)) -ne $pinnedBytes) {
		throw 'Manifest updater did not reject a duplicate Goose version.'
	}
	Remove-Item -LiteralPath (Join-Path $migrationDirectory '00039_duplicate.sql') -Force

	Remove-Item -LiteralPath $manifestPath -Force
	$missingManifestRejected = $false
	try {
		& $updaterPath -RepositoryRoot $resolvedTestRoot | Out-Null
	}
	catch {
		$missingManifestRejected = $true
	}
	if (-not $missingManifestRejected -or (Test-Path -LiteralPath $manifestPath)) {
		throw 'Manifest updater silently bootstrapped a missing manifest.'
	}

	& $updaterPath -RepositoryRoot $resolvedTestRoot -InitializeManifest | Out-Null
	$upperExtensionRejected = $false
	try {
		[System.IO.File]::WriteAllText((Join-Path $migrationDirectory '00040_upper.SQL'), "SELECT 40;`n", [System.Text.Encoding]::UTF8)
		& $updaterPath -RepositoryRoot $resolvedTestRoot | Out-Null
	}
	catch {
		$upperExtensionRejected = $true
	}
	if (-not $upperExtensionRejected) {
		throw 'Manifest updater accepted an uppercase SQL extension.'
	}

	$zeroVersionRoot = Join-Path $resolvedTestRoot 'zero-version-bootstrap'
	$zeroMigrationDirectory = Join-Path $zeroVersionRoot 'db/migrations'
	New-Item -ItemType Directory -Path $zeroMigrationDirectory -Force | Out-Null
	[System.IO.File]::WriteAllText((Join-Path $zeroMigrationDirectory '00000_zero.sql'), "SELECT 0;`n", [System.Text.Encoding]::UTF8)
	$zeroVersionRejected = $false
	try {
		& $updaterPath -RepositoryRoot $zeroVersionRoot -InitializeManifest | Out-Null
	}
	catch {
		$zeroVersionRejected = $true
	}
	if (-not $zeroVersionRejected -or (Test-Path -LiteralPath (Join-Path $zeroVersionRoot 'db/migration_hashes.sha256'))) {
		throw 'Manifest updater accepted Goose version zero during explicit initialization.'
	}

    Write-Output 'MIGRATION_HASH_UPDATER=PASSED (rejects pinned edits, backward/duplicate/zero versions, uppercase extensions and missing manifest; admits a new forward migration)'
}
finally {
    $recheckedRoot = [System.IO.Path]::GetFullPath($resolvedTestRoot)
    if ($recheckedRoot.StartsWith($parentPrefix, [System.StringComparison]::OrdinalIgnoreCase) -and
        [System.IO.Path]::GetFileName($recheckedRoot).StartsWith('polis-migration-manifest-test-', [System.StringComparison]::Ordinal)) {
        Remove-Item -LiteralPath $recheckedRoot -Recurse -Force
    }
    else {
        throw 'Refusing to remove unexpected migration-manifest test path.'
    }
}
