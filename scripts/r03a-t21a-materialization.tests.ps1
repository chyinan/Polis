param(
    [string]$Evidence = 'D:\Programs\Polis\evidence\development\r0.3a-t21'
)

$ErrorActionPreference = 'Stop'
$helper = Join-Path $PSScriptRoot 'r03a-t21a-tool-materialization.ps1'
if (-not (Test-Path -LiteralPath $helper -PathType Leaf)) { throw 'materialization helper is missing' }
. $helper

function Assert-Throws([scriptblock]$Action, [string]$Name) {
    try { & $Action; throw "$Name was accepted" }
    catch {
        if ($_.Exception.Message -eq "$Name was accepted") { throw }
    }
}

$root = '[{"name":"polis_a","inputSchema":{"type":"object"}},{"name":"polis_b","inputSchema":{"type":"object"}}]'
$records = Get-CanonicalToolRecordsFromJson $root
if ($records.Count -ne 2 -or $records[0].name -ne 'polis_a' -or $records[1].name -ne 'polis_b') { throw 'root JSON array did not materialize as two ordered records' }

Assert-Throws { Get-CanonicalToolRecordsFromJson '[[{"name":"polis_a"},{"name":"polis_b"}]]' } 'nested array'

$schemaDigest = Get-CanonicalJsonDigest ($records[0].inputSchema | ConvertTo-Json -Compress -Depth 20)
$schemaDigestB = Get-CanonicalJsonDigest ($records[1].inputSchema | ConvertTo-Json -Compress -Depth 20)
$aggregateDigest = Get-CanonicalJsonDigest $root
$surface = [pscustomobject]@{
    tool_count = 2
    aggregate_manifest_digest = $aggregateDigest
    tools = @(
        [pscustomobject]@{ name = 'polis_a'; schema_digest = $schemaDigest; binding_identity = 'binding-a'; authorization_class = 'peer_backend'; registration_ordinal = 1 },
        [pscustomobject]@{ name = 'polis_b'; schema_digest = $schemaDigestB; binding_identity = 'binding-b'; authorization_class = 'peer_backend'; registration_ordinal = 2 }
    )
}
$validated = Assert-CanonicalToolRecords $records $surface $root $aggregateDigest
if ($validated.count -ne 2 -or $validated.binding_metadata.Count -ne 2) { throw 'binding metadata was not returned for both records' }

Assert-Throws { Assert-CanonicalToolRecords @($records[0]) $surface $root $aggregateDigest } 'count mismatch'
$reordered = @($records[1], $records[0])
Assert-Throws { Assert-CanonicalToolRecords $reordered $surface $root $aggregateDigest } 'reordered tool'
$badDigestSurface = [pscustomobject]@{ tool_count = 2; aggregate_manifest_digest = $aggregateDigest; tools = @([pscustomobject]@{ name = 'polis_a'; schema_digest = ('0' * 64); binding_identity = 'binding-a'; authorization_class = 'peer_backend'; registration_ordinal = 1 }, $surface.tools[1]) }
Assert-Throws { Assert-CanonicalToolRecords $records $badDigestSurface $root $aggregateDigest } 'schema digest mismatch'
Assert-Throws { Assert-CanonicalToolRecords $records $surface $root ('1' * 64) } 'aggregate digest mismatch'
$badBindingSurface = [pscustomobject]@{ tool_count = 2; aggregate_manifest_digest = $aggregateDigest; tools = @([pscustomobject]@{ name = 'polis_a'; schema_digest = $schemaDigest; binding_identity = ''; authorization_class = 'peer_backend'; registration_ordinal = 1 }, $surface.tools[1]) }
Assert-Throws { Assert-CanonicalToolRecords $records $badBindingSurface $root $aggregateDigest } 'missing binding metadata'

$registryPath = Join-Path $Evidence 'tool-registry.json'
$surfacePath = Join-Path $Evidence 'tool-surface.json'
$registryCanonical = (Get-Content -Raw $registryPath).TrimEnd("`r", "`n")
$registryRecords = Get-CanonicalToolRecordsFromJson $registryCanonical
$toolEvidence = Get-Content -Raw $surfacePath | ConvertFrom-Json
$registryValidated = Assert-CanonicalToolRecords $registryRecords $toolEvidence.surface $registryCanonical $toolEvidence.surface.aggregate_manifest_digest
if ($registryValidated.count -ne 11 -or ($registryValidated.names -join "`n") -ne ($toolEvidence.tool_names -join "`n")) { throw 'formal T21 registry exact names/order regression failed' }
Write-Output 'T21A materialization tests passed'
