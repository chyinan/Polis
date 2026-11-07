$ErrorActionPreference = 'Stop'

function Convert-WindowsPathToWsl([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { throw 'cannot convert an empty Windows path' }
    $previousErrorAction = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = @(& wsl.exe -e wslpath -a -u -- $Path 2>&1 | ForEach-Object { [string]$_ })
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorAction
    }
    if ($exitCode -ne 0) { throw "wslpath failed for Windows path: $Path" }
    $candidates = @($output | ForEach-Object { ([string]$_).Trim() } | Where-Object { $_ -match '^/' })
    if ($candidates.Count -ne 1) { throw "wslpath returned an ambiguous path for: $Path" }
    return $candidates[0]
}

function Convert-WslPathToWindows([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { throw 'cannot convert an empty WSL path' }
    $previousErrorAction = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = @(& wsl.exe -e wslpath -a -w -- $Path 2>&1 | ForEach-Object { [string]$_ })
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorAction
    }
    if ($exitCode -ne 0) { throw "wslpath failed for WSL path: $Path" }
    $candidates = @($output | ForEach-Object { ([string]$_).Trim() } | Where-Object { $_ -match '^(?:[A-Za-z]:\\|\\\\)' })
    if ($candidates.Count -ne 1) { throw "wslpath returned an ambiguous Windows path for: $Path" }
    return $candidates[0]
}

function Write-FrontendJson([string]$Path, $Value) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    [IO.File]::WriteAllText($Path, (($Value | ConvertTo-Json -Depth 30) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

function Convert-FrontendConfigToWsl($WindowsConfig) {
    $pathKeys = @(
        'execution_config_path', 'binary', 'code_mode_host', 'auth_file', 'selected_config_path',
        'execution_manifest_path', 'qualification_path', 'blob_durability_qualification_path',
        'behavioral_contract_path', 'checker_feedback_qualification_path', 'postgres_dump_path',
        'runtime_root', 'evidence_root', 'recovery_package_root', 'source_cas_root', 'runtime_artifact_manifest_path',
        'runtime_database_binding_path', 'database_access_view_path', 'authorization_binding_path', 'current_l1_evidence_path',
        'handover_boundary_evidence_path', 'handover_boundary_snapshot_path'
    )
    $wsl = [ordered]@{}
    foreach ($key in $WindowsConfig.Keys) {
        if ($key -eq 'path_encoding') {
            $wsl[$key] = 'wsl'
		} elseif ($key -eq 'runtime_cas_binding') {
			$binding = [ordered]@{}
			foreach ($bindingKey in $WindowsConfig[$key].Keys) { $binding[$bindingKey] = $WindowsConfig[$key][$bindingKey] }
			$binding['canonical_root'] = Convert-WindowsPathToWsl ([string]$WindowsConfig[$key]['canonical_root'])
			$wsl[$key] = $binding
        } elseif ($pathKeys -contains $key) {
            $wsl[$key] = Convert-WindowsPathToWsl ([string]$WindowsConfig[$key])
        } else {
            $wsl[$key] = $WindowsConfig[$key]
        }
    }
    if (-not $wsl.Contains('path_encoding')) { $wsl['path_encoding'] = 'wsl' }
    return $wsl
}

function Write-FrontendConfigPair($WindowsConfig, [string]$WindowsConfigPath, [string]$WslConfigPath) {
    $WindowsConfig.path_encoding = 'windows-native'
    $WindowsConfig.execution_config_path = $WindowsConfigPath
    $wsl = Convert-FrontendConfigToWsl $WindowsConfig
    $wsl.execution_config_path = Convert-WindowsPathToWsl $WslConfigPath
    Write-FrontendJson $WindowsConfigPath $WindowsConfig
    Write-FrontendJson $WslConfigPath $wsl
}

function Invoke-FrontendWslConfigProbe([string]$Repo, [string]$WslConfigPath, [string]$WslReportPath) {
    $repoUnix = Convert-WindowsPathToWsl $Repo
    $configUnix = Convert-WindowsPathToWsl $WslConfigPath
    $reportUnix = Convert-WindowsPathToWsl $WslReportPath
    & bash -lc "cd '$repoUnix' && bash scripts/go.sh run ./cmd/polis-r03a-frontend-config-probe -config '$configUnix' -output '$reportUnix'"
    if ($LASTEXITCODE -ne 0) {
        if (Test-Path -LiteralPath $WslReportPath) { Write-Warning (Get-Content -Raw -LiteralPath $WslReportPath) }
        $configForDiagnostics = $WslReportPath -replace '-report\.json$', '.json'
        if (Test-Path -LiteralPath $configForDiagnostics) {
            $diagnosticConfig = Get-Content -Raw -LiteralPath $configForDiagnostics | ConvertFrom-Json
            Write-Warning ("WSL probe path diagnostic: postgres_dump_path={0}" -f [string]$diagnosticConfig.postgres_dump_path)
        }
        throw 'FRONTEND_EXECUTION_CONFIG_INVALID: WSL config probe failed'
    }
    return (Get-Content -Raw -LiteralPath $WslReportPath | ConvertFrom-Json)
}
