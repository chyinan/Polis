function Get-CanonicalJsonDigest([string]$Json) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($Json)))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Get-CanonicalToolRecordsFromJson([string]$Json) {
    if ([string]::IsNullOrEmpty($Json)) { throw 'empty tool registry JSON' }
    $parsed = ConvertFrom-Json -InputObject $Json
    if (-not ($parsed -is [System.Array])) { throw 'root JSON value is not an array' }
    $records = New-Object System.Collections.Generic.List[object]
    foreach ($record in $parsed) {
        if ($record -is [System.Array]) { throw 'nested JSON array is not a flat tool registry' }
        if ($null -eq $record -or $null -eq $record.name -or [string]::IsNullOrEmpty([string]$record.name)) { throw 'tool registry record is missing a name' }
        $null = $records.Add($record)
    }
    return $records.ToArray()
}

function Assert-CanonicalToolRecords {
    param(
        [object]$Records,
        [object]$ExpectedSurface,
        [string]$CanonicalJson,
        [string]$ExpectedAggregateDigest
    )
    $actualRecords = @($Records)
    $expectedTools = @($ExpectedSurface.tools)
    if ($null -eq $ExpectedSurface.tool_count -or [int]$ExpectedSurface.tool_count -ne $expectedTools.Count) { throw 'expected tool surface cardinality is invalid' }
    if ($actualRecords.Count -ne [int]$ExpectedSurface.tool_count) { throw 'tool registry count mismatch' }
    $actualAggregateDigest = Get-CanonicalJsonDigest $CanonicalJson
    if ($actualAggregateDigest -ne $ExpectedAggregateDigest -or $ExpectedAggregateDigest -ne [string]$ExpectedSurface.aggregate_manifest_digest) { throw 'tool registry aggregate manifest digest mismatch' }
    $names = New-Object System.Collections.Generic.List[string]
    $schemaDigests = New-Object System.Collections.Generic.List[string]
    $bindings = New-Object System.Collections.Generic.List[object]
    for ($index = 0; $index -lt $actualRecords.Count; $index++) {
        $record = $actualRecords[$index]
        if ($record -is [System.Array]) { throw 'nested System.Object[] was materialized as one tool record' }
        $expected = $expectedTools[$index]
        if ([string]$record.name -ne [string]$expected.name) { throw 'reordered or unexpected tool name' }
        $schemaJson = $record.inputSchema | ConvertTo-Json -Compress -Depth 50
        $schemaDigest = Get-CanonicalJsonDigest $schemaJson
        if ($schemaDigest -ne [string]$expected.schema_digest) { throw "per-tool schema digest mismatch: $($record.name)" }
        if ([string]::IsNullOrEmpty([string]$expected.binding_identity) -or [string]::IsNullOrEmpty([string]$expected.authorization_class)) { throw "missing callback/policy binding metadata: $($record.name)" }
        $null = $names.Add([string]$record.name)
        $null = $schemaDigests.Add($schemaDigest)
        $null = $bindings.Add([pscustomobject]@{ name = [string]$record.name; binding_identity = [string]$expected.binding_identity; authorization_class = [string]$expected.authorization_class; registration_ordinal = [int]$expected.registration_ordinal })
    }
    return [pscustomobject]@{ count = $actualRecords.Count; names = $names.ToArray(); per_tool_schema_digest = $schemaDigests.ToArray(); aggregate_manifest_digest = $actualAggregateDigest; binding_metadata = $bindings.ToArray() }
}
