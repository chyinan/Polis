param([string]$OutputDirectory = "evidence/development/r0.3a-t5")
$ErrorActionPreference = "Stop"
$utf8 = [System.Text.UTF8Encoding]::new($false)
$sha = [System.Security.Cryptography.SHA256]::Create()
function HashText([string]$s){([BitConverter]::ToString($sha.ComputeHash($utf8.GetBytes($s))).Replace('-','')).ToLowerInvariant()}
function C($v){$v|ConvertTo-Json -Compress -Depth 100}
function B([string]$s){$utf8.GetByteCount($s)}
function Protocol([string]$p){Get-Content -LiteralPath $p|Where-Object{$_}|ForEach-Object{$_|ConvertFrom-Json}}
function ThreadParams([string]$p){(Protocol $p|Where-Object{$_.direction -eq 'send' -and $_.data.method -eq 'thread/start'}|Select-Object -First 1).data.params}
function Prop($o,[string]$n){if($null -eq $o){return $null};$p=$o.PSObject.Properties[$n];if($null -eq $p){return $null};$p.Value}
function Depth($v,$d=0){if($null -eq $v){return $d};if($v -is [System.Collections.IDictionary]){return (($v.Values|ForEach-Object{Depth $_ ($d+1)}|Measure-Object -Maximum).Maximum)};if($v -is [System.Collections.IEnumerable] -and $v -isnot [string]){return (($v|ForEach-Object{Depth $_ ($d+1)}|Measure-Object -Maximum).Maximum)};return $d}
function SchemaCheck($tool){
  $s=Prop $tool 'inputSchema';$issues=@();$props=Prop $s 'properties';$required=@(Prop $s 'required');$stype=Prop $s 'type'
  if($stype -ne 'object'){$issues+='root_type_not_object'}
  if($null -eq $props){$props=[ordered]@{}}
  foreach($r in $required){if($null -eq $props.PSObject.Properties[$r]){$issues+="required_missing_property:$r"}}
  foreach($p in $props.PSObject.Properties){if($null -eq (Prop $p.Value 'type') -and $null -eq (Prop $p.Value 'enum')){$issues+="property_without_type_or_enum:$($p.Name)"}}
  foreach($bad in @('oneOf','anyOf','allOf','nullable')){if($null -ne (Prop $s $bad)){$issues+="keyword:$bad"}}
  $desc=[string](Prop $tool 'description');if($desc.IndexOf([char]0xFFFD)-ge 0){$issues+='description_replacement_character'}
  $depth=Depth $s;if($depth -gt 8){$issues+='nested_depth_gt_8'}
  [pscustomobject]@{Valid=($issues.Count -eq 0);Issues=$issues;RootType=$stype;Required=$required;PropertyNames=@($props.PSObject.Properties.Name);Depth=$depth}
}
function New-Run([string]$Name,[string]$Kind,[string]$Protocol,[string]$Session,[string]$Capability,[string]$Preflight){[pscustomobject]@{Name=$Name;Kind=$Kind;Protocol=$Protocol;Session=$Session;Capability=$Capability;Preflight=$Preflight}}
$goodPath='evidence/development/r0.2/luna-1/e9485489eeece81bcce3678779e3f91c/protocol.jsonl'
$badPath='evidence/development/r0.3a-real/luna-1/70a7fb6d45e09588eca30154d2789732/protocol.jsonl'
$good=ThreadParams $goodPath;$bad=ThreadParams $badPath
$baseNames=@($good.dynamicTools|ForEach-Object{[string](Prop $_ 'name')});$badNames=@($bad.dynamicTools|ForEach-Object{[string](Prop $_ 'name')});$newNames=@($badNames|Where-Object{$_ -notin $baseNames})
$baseLookupNames=@($baseNames|ForEach-Object{$_ -replace '^polis_',''})
$handlers=[ordered]@{work_current='EmployeeTools.call->Handover';context_read='EmployeeTools.call->Handover';workspace_read='EmployeeTools.call->Workspace';workspace_replace='EmployeeTools.call->TXReplace';workspace_check='EmployeeTools.call->workspace.check';work_checkpoint='EmployeeTools.call->TXCheckpoint';artifact_submit='EmployeeTools.call->TXSubmit';contract_propose='PeerEmployeeTools.call->TXProposePeerContract';contract_accept='PeerEmployeeTools.call->TXAcceptPeerContract';contract_read='PeerEmployeeTools.call->PeerContractRead';collab_send='PeerEmployeeTools.call->TXPeerSend'}
function ToolManifest($tools,$surface){$out=@();$ordinal=0;foreach($tool in @($tools)){$ordinal++;$name=[string](Prop $tool 'name');$lookup=$name -replace '^polis_','';$schema=Prop $tool 'inputSchema';$schemaText=C $schema;$desc=[string](Prop $tool 'description');$check=SchemaCheck $tool;$handler=$handlers[$lookup];if(($surface -eq 'peer_backend' -or $surface -eq 'peer_backend_subset') -and $lookup -in $baseLookupNames){$handler=$handler -replace '^EmployeeTools','PeerEmployeeTools'};$out += [pscustomobject]@{Name=$name;SchemaCanonicalDigest=HashText $schemaText;SchemaBytes=B $schemaText;DescriptionBytes=B $desc;DescriptionDigest=HashText $desc;Required=$check.Required;PropertyNames=$check.PropertyNames;NestedDepth=$check.Depth;SchemaValidation=$check;Handler=$handler;AuthorizationClass=$surface;CapabilityBinding=$surface;RegistrationOrdinal=$ordinal}};return $out}
function Variant($name,$tools,$developer){$schema=C @($tools);$params=[ordered]@{model=Prop $bad 'model';allowProviderModelFallback=Prop $bad 'allowProviderModelFallback';approvalPolicy=Prop $bad 'approvalPolicy';sandbox=Prop $bad 'sandbox';cwd=Prop $bad 'cwd';environments=Prop $bad 'environments';ephemeral=Prop $bad 'ephemeral';dynamicTools=$tools;config=Prop $bad 'config';developerInstructions=$developer};$request=C $params;$round=C ($request|ConvertFrom-Json);$without=[ordered]@{};foreach($p in $params.Keys){$without[$p]=if($p -eq 'dynamicTools'){@()}else{$params[$p]}};$nonTool=B (C $without);[pscustomobject]@{Name=$name;CanonicalRequestDigest=HashText $request;TotalBytes=B $request;ToolBytes=B $schema;NonToolBytes=$nonTool;ToolOrder=@($tools|ForEach-Object{Prop $_ 'name'});SerializationRoundTripDigest=HashText $round;RoundTripStable=((HashText $request) -eq (HashText $round))}}
$goodManifest=ToolManifest $good.dynamicTools 'employee_default';$badManifest=ToolManifest $bad.dynamicTools 'peer_backend'
$intersection=@();foreach($n in $baseNames){$g=$goodManifest|Where-Object Name -eq $n;$b=$badManifest|Where-Object Name -eq $n;$intersection += [pscustomobject]@{Name=$n;SchemaDigestSame=($g.SchemaCanonicalDigest -eq $b.SchemaCanonicalDigest);SchemaBytesDelta=($b.SchemaBytes-$g.SchemaBytes);DescriptionDigestSame=($g.DescriptionDigest -eq $b.DescriptionDigest);DescriptionBytesDelta=($b.DescriptionBytes-$g.DescriptionBytes);HandlerChanged=($g.Handler -ne $b.Handler);RegistrationOrdinalGood=$g.RegistrationOrdinal;RegistrationOrdinalFail=$b.RegistrationOrdinal;GoodHandler=$g.Handler;FailHandler=$b.Handler}}
$newManifest=@($badManifest|Where-Object Name -in $newNames)
$variants=@(Variant 'A' $good.dynamicTools (Prop $good 'developerInstructions');Variant 'B' $good.dynamicTools (Prop $bad 'developerInstructions');Variant 'C' $bad.dynamicTools (Prop $good 'developerInstructions');Variant 'D' $bad.dynamicTools (Prop $bad 'developerInstructions'))
$paddingTests=@();foreach($size in @(3072,4096,8192,16384)){$pad='x'*$size;$payload=[ordered]@{model=Prop $bad 'model';dynamicTools=$bad.dynamicTools;developerInstructions=Prop $bad 'developerInstructions';deterministicPadding=$pad};$text=C $payload;$round=C ($text|ConvertFrom-Json);$paddingTests += [pscustomobject]@{PaddingBytes=$size;SerializedBytes=B $text;Digest=HashText $text;RoundTripStable=((HashText $text) -eq (HashText $round));Truncated=((B $text) -lt $size)}}
$newObjects=@($bad.dynamicTools|Where-Object{(Prop $_ 'name') -in $newNames});$subset=@()
for($mask=0;$mask -lt 16;$mask++){
  $selected=@($baseNames|ForEach-Object{$n=$_;$bad.dynamicTools|Where-Object{(Prop $_ 'name') -eq $n}|Select-Object -First 1});$chosen=@();for($i=0;$i -lt $newObjects.Count;$i++){if(($mask -band (1 -shl $i)) -ne 0){$chosen+=$newObjects[$i]}}
  $sets=@([pscustomobject]@{Order='canonical';Tools=@($selected+$chosen)},[pscustomobject]@{Order='reversed';Tools=@($selected+$chosen|Sort-Object -Property name -Descending)},[pscustomobject]@{Order='alternate';Tools=@($selected+$chosen|Sort-Object -Property name)})
  foreach($set in $sets){$names=@($set.Tools|ForEach-Object{Prop $_ 'name'});$manifest=ToolManifest $set.Tools 'peer_backend_subset';$subset += [pscustomobject]@{Mask=$mask;Order=$set.Order;ToolCount=$names.Count;Names=$names;UniqueNames=(@($names|Select-Object -Unique).Count -eq $names.Count);AllSchemasValid=(@($manifest|Where-Object{-not $_.SchemaValidation.Valid}).Count -eq 0);AllHandlersBound=(@($names|Where-Object{$null -eq $handlers[($_ -replace '^polis_','')]}).Count -eq 0);Request=(Variant ("mask{0}-{1}" -f $mask,$set.Order) $set.Tools (Prop $bad 'developerInstructions'))}}
}
$manifest=[pscustomobject]@{GeneratedAt=(Get-Date).ToUniversalTime().ToString('o');SourceRuns=@{KnownGood=$goodPath;Failing=$badPath};KnownGoodManifest=$goodManifest;FailingBackendManifest=$badManifest;Intersection=$intersection;NewTools=$newManifest;Variants=$variants;SyntheticPaddingTests=$paddingTests;All16Subsets=$subset;CompatibilityStatement='local schema/serialization/handler validity does not prove provider compatibility for app-server 0.151.0'}
New-Item -ItemType Directory -Force -Path $OutputDirectory|Out-Null
$manifest|ConvertTo-Json -Depth 100|Set-Content -LiteralPath (Join-Path $OutputDirectory 'tool-manifest.json') -Encoding UTF8
$md=@('# R0.3A-T5 tool surface and native request qualification','',("Known-good R0.2 Medium tools: {0}; failing R0.3A Backend tools: {1}; exact additions: {2}." -f $goodManifest.Count,$badManifest.Count,($newNames -join ', ')),'','All results are local-only; provider compatibility is explicitly unproven.','','## Exact new tools')
foreach($x in $newManifest){$md += ('- {0}: schema={1}, bytes={2}, handler={3}, policy={4}, ordinal={5}.' -f $x.Name,$x.SchemaCanonicalDigest,$x.SchemaBytes,$x.Handler,$x.AuthorizationClass,$x.RegistrationOrdinal)}
$md += ''; $md += '## Intersection'; foreach($x in $intersection){$md += ('- ' + $x.Name + ': schema_same=' + $x.SchemaDigestSame + ', description_same=' + $x.DescriptionDigestSame + ', handler_changed=' + $x.HandlerChanged + ', ordinal=' + $x.RegistrationOrdinalGood + '->' + $x.RegistrationOrdinalFail + '.')}
$md += ''; $md += '## Subsets'; $badSub=@($subset|Where-Object{!$_.UniqueNames -or !$_.AllSchemasValid -or !$_.AllHandlersBound});$md += ('All 16 surface combinations × 3 orderings locally valid: {0}. Failing rows: {1}.' -f ($badSub.Count -eq 0),$badSub.Count)
$md += ''; $md += '## Synthetic request padding'; foreach($p in $paddingTests){$md += ('- {0} bytes: serialized={1}, round_trip_stable={2}, truncated={3}.' -f $p.PaddingBytes,$p.SerializedBytes,$p.RoundTripStable,$p.Truncated)}
$md|Set-Content -LiteralPath (Join-Path $OutputDirectory 'qualification.md') -Encoding UTF8
Write-Output "wrote $OutputDirectory"
