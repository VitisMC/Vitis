#requires -Version 7.2
<#
.SYNOPSIS
Prepares official inputs and regenerates Minecraft data natively in PowerShell.
.PARAMETER Version
Minecraft version; defaults to MC_VERSION from the environment, otherwise 1.21.4.
#>
param([string]$Version = '1.21.4')

$ErrorActionPreference = 'Stop'
if (-not $PSBoundParameters.ContainsKey('Version') -and (Test-Path Env:MC_VERSION)) { $Version = $env:MC_VERSION }
if ($args.Count -or $Version -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$' -or $Version.Contains('..')) {
    throw 'Usage: update_version.ps1 [-Version <minecraft-version>]'
}
foreach ($tool in @('go', 'gofmt', 'git', 'java', 'javac')) {
    if (-not (Get-Command $tool -CommandType Application -ErrorAction SilentlyContinue)) { throw "Missing required command: $tool" }
}
$javaInfo = (& java -version 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $javaInfo -notmatch 'version "(21(?:\.[^"]*)?)"') { throw 'Generation requires JDK 21 on PATH.' }
$javaVersion = $Matches[1]
$compilerInfo = (& javac -version 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $compilerInfo -notmatch '(?m)^javac 21(?:\.|\s|$)') { throw 'Generation requires javac from JDK 21 on PATH.' }
if ($IsWindows) {
    $goInfo = & go env -json CGO_ENABLED GOOS GOARCH CC | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $goInfo.CGO_ENABLED -ne '1' -or $goInfo.GOOS -ne 'windows' -or $goInfo.GOARCH -ne 'amd64') {
        throw 'Mandatory race checks require native windows/amd64 Go with CGO_ENABLED=1 and a C compiler.'
    }
    $compiler = Get-Command $goInfo.CC -CommandType Application -ErrorAction Stop | Select-Object -First 1
    $synchronization = (& $compiler.Source '--print-file-name' 'libsynchronization.a' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $synchronization -PathType Leaf)) {
        throw 'Mandatory race checks require a C compiler with mingw-w64 runtime libraries version 8 or newer.'
    }
}

. (Join-Path $PSScriptRoot 'generation_transaction.ps1')
. (Join-Path $PSScriptRoot 'download.ps1')
$manifest = Get-Content -LiteralPath (Join-Path (Split-Path -Parent $PSScriptRoot) 'generation_manifest.json') -Raw | ConvertFrom-Json -AsHashtable
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$cacheRoot = Join-Path $root '.mc-decompiled'
$cache = Join-Path $cacheRoot "downloads/$Version"
$tools = Join-Path $cacheRoot 'downloads/tools'
$data = Join-Path $root ".mcdata/$Version"
foreach ($directory in @($cache, $tools, (Join-Path $root '.mcdata'))) {
    $relative = [System.IO.Path]::GetRelativePath($root, $directory)
    $null = Resolve-VitisOutput $root $relative
    [System.IO.Directory]::CreateDirectory($directory) | Out-Null
}
$lock = Join-Path $root '.mcdata/.generate-lock'
# Directory.CreateDirectory alone would allow two simultaneous generators.
try { New-Item -Path $lock -ItemType Directory -ErrorAction Stop | Out-Null }
catch { throw "Another generation is active. After an ungraceful stop, inspect .generation-backup.* before removing $lock" }
$token = [guid]::NewGuid().ToString('N')
$stage = Join-Path $root ".mcdata/.$Version.stage.$token"
$sourceStage = Join-Path $cacheRoot ".$Version.sources.$token"
$scratch = Join-Path ([System.IO.Path]::GetTempPath()) "vitis-generation-$token"
$backup = Join-Path $root ".generation-backup.$token"
$records = @()
$publishing = $false
$success = $false
$restoreFailed = $false
$handler = [System.Net.Http.HttpClientHandler]::new()
$handler.AllowAutoRedirect = $false
$client = [System.Net.Http.HttpClient]::new($handler)
$client.Timeout = [TimeSpan]::FromMinutes(3)
$client.DefaultRequestHeaders.UserAgent.ParseAdd('Vitis-Generator/1.0')
Push-Location $root
try {
    foreach ($directory in @($stage, $sourceStage, $scratch)) { [System.IO.Directory]::CreateDirectory($directory) | Out-Null }
    $metadataPath = Join-Path $stage 'mojang-version.json'
    Get-VitisDownload $client 'https://piston-meta.mojang.com/mc/game/version_manifest_v2.json' (Join-Path $scratch 'manifest.json') -Limit 4194304
    $versions = Get-Content -LiteralPath (Join-Path $scratch 'manifest.json') -Raw | ConvertFrom-Json
    $selected = @($versions.versions | Where-Object id -CEQ $Version)
    if ($selected.Count -ne 1) { throw "Minecraft version not found: $Version" }
    Get-VitisDownload $client $selected[0].url $metadataPath -Limit 4194304
    $metadata = Get-Content -LiteralPath $metadataPath -Raw | ConvertFrom-Json
    if ($metadata.id -cne $Version) { throw 'Official Minecraft version mismatch' }
    $serverSHA = $metadata.downloads.server.sha1
    $mappingsSHA = $metadata.downloads.server_mappings.sha1
    if ($serverSHA -cnotmatch '^[0-9a-f]{40}$' -or $mappingsSHA -cnotmatch '^[0-9a-f]{40}$') { throw 'Invalid official checksums' }
    if ($metadata.downloads.server.size -le 0 -or $metadata.downloads.server_mappings.size -le 0) { throw 'Invalid official download sizes' }
    $server = Join-Path $cache 'server.jar'
    $mappings = Join-Path $cache 'server_mappings.txt'
    Get-VitisDownload $client $metadata.downloads.server.url $server SHA1 $serverSHA $metadata.downloads.server.size -Limit 134217728
    Get-VitisDownload $client $metadata.downloads.server_mappings.url $mappings SHA1 $mappingsSHA $metadata.downloads.server_mappings.size
    $decompiler = Join-Path $tools "MinecraftDecompiler-$($manifest.decompiler.version).jar"
    $vineflower = Join-Path $tools "vineflower-$($manifest.vineflower.version).jar"
    Get-VitisDownload $client $manifest.decompiler.url $decompiler SHA256 $manifest.decompiler.sha256 -Limit 134217728
    Get-VitisDownload $client $manifest.vineflower.url $vineflower SHA256 $manifest.vineflower.sha256 -Limit 134217728

    $fingerprint = "minecraft_version=$Version`nserver_sha1=$serverSHA`nmappings_sha1=$mappingsSHA`ndecompiler_sha256=$($manifest.decompiler.sha256)`nvineflower_sha256=$($manifest.vineflower.sha256)`njdk_version=$javaVersion`noptions=$($manifest.source_options)`n"
    Write-VitisText (Join-Path $sourceStage "$Version.sources.lock") $fingerprint
    $sourceFiles = @("$Version-remapped.jar", "$Version-decompiled")
    $sourceArtifacts = $sourceFiles + @("$Version.sources.lock", "$Version.sources.sha256")
    $sourceCached = $false
    if ((Test-Path -LiteralPath (Join-Path $cacheRoot "$Version.sources.lock")) -and
        (Test-Path -LiteralPath (Join-Path $cacheRoot "$Version.sources.sha256"))) {
        try {
            $sourceCached = [System.IO.File]::ReadAllText((Join-Path $cacheRoot "$Version.sources.lock")) -ceq $fingerprint -and
                [System.IO.File]::ReadAllText((Join-Path $cacheRoot "$Version.sources.sha256")) -ceq (Get-VitisInventory $cacheRoot $sourceFiles)
        }
        catch { $sourceCached = $false }
    }
    if ($sourceCached) { Write-Host "Using verified named sources and remapped JAR for $Version" }
    else {
        Invoke-VitisCommand go @('run', './cmd/vitis-data', '-unpack', '-version', $Version, '-jar', $server, '-output', (Join-Path $sourceStage 'bundle'))
        $work = Join-Path $sourceStage 'work'
        [System.IO.Directory]::CreateDirectory($work) | Out-Null
        $libraries = @(Get-ChildItem -LiteralPath (Join-Path $sourceStage 'bundle/libraries') -Recurse -File -Filter '*.jar' | Sort-Object FullName | ForEach-Object { "-e=$($_.FullName)" })
        Push-Location $work
        try {
            Invoke-VitisCommand java @('-Xmx2G', '-XX:ActiveProcessorCount=4', '-jar', $decompiler, '--input', (Join-Path $sourceStage 'bundle/server.jar'), '--map', $mappings, '--output', (Join-Path $sourceStage "$Version-remapped.jar"), '--temp', (Join-Path $work 'temporary'))
            Invoke-VitisCommand java (@('-Xmx2G', '-XX:ActiveProcessorCount=4', '-jar', $vineflower, '--folder', '-thr=4', '-jrt=1', '-sef=1', '-log=WARN') + $libraries + @((Join-Path $sourceStage "$Version-remapped.jar"), (Join-Path $sourceStage "$Version-decompiled")))
        }
        finally { Pop-Location }
        foreach ($required in $manifest.required_sources) {
            if (-not (Test-Path -LiteralPath (Join-Path $sourceStage "$Version-decompiled/$required") -PathType Leaf)) { throw "Missing named source: $required" }
        }
        Write-VitisText (Join-Path $sourceStage "$Version.sources.sha256") (Get-VitisInventory $sourceStage $sourceFiles)
    }

    $commit = ''
    $sourceLock = Join-Path $data 'source.lock'
    if (Test-Path -LiteralPath $sourceLock) {
        foreach ($line in Get-Content -LiteralPath $sourceLock) { if ($line -match '^minecraft_data_commit=(.*)$') { $commit = $Matches[1] } }
    }
    if (-not $commit -and $manifest.minecraft_data_commits.ContainsKey($Version)) { $commit = $manifest.minecraft_data_commits[$Version] }
    if (-not $commit) {
        Get-VitisDownload $client "https://api.github.com/repos/PrismarineJS/minecraft-data/commits?path=data/pc/$Version&per_page=1" (Join-Path $scratch 'commits.json') -Limit 4194304
        $commits = @(Get-Content -LiteralPath (Join-Path $scratch 'commits.json') -Raw | ConvertFrom-Json)
        $commit = $commits[0].sha
    }
    if ($commit -cnotmatch '^[0-9a-f]{40}$') { throw 'Invalid minecraft-data commit' }
    $sourceURL = "https://raw.githubusercontent.com/PrismarineJS/minecraft-data/$commit/data"
    Get-VitisDownload $client "$sourceURL/dataPaths.json" (Join-Path $stage 'dataPaths.json')
    $paths = Get-Content -LiteralPath (Join-Path $stage 'dataPaths.json') -Raw | ConvertFrom-Json -AsHashtable
    foreach ($name in $manifest.prismarine_files) {
        $sourcePath = $paths.pc[$Version][$name]
        if ($sourcePath -notmatch '^pc/[A-Za-z0-9._-]+$') { throw "Invalid minecraft-data path: $name" }
        $destination = Join-Path $stage "$name.json"
        Get-VitisDownload $client "$sourceURL/$sourcePath/$name.json" $destination
        $null = Get-Content -LiteralPath $destination -Raw | ConvertFrom-Json
    }
    $versionMetadata = Get-Content -LiteralPath (Join-Path $stage 'version.json') -Raw | ConvertFrom-Json
    if ($versionMetadata.minecraftVersion -cne $Version) { throw 'minecraft-data version mismatch' }
    Write-VitisText (Join-Path $stage 'source.lock') "minecraft_data_commit=$commit`nserver_sha1=$serverSHA`nmappings_sha1=$mappingsSHA`ndecompiler_sha256=$($manifest.decompiler.sha256)`nvineflower_sha256=$($manifest.vineflower.sha256)`n"
    Invoke-VitisCommand go @('run', './cmd/vitis-data', '-version', $Version, '-jar', $server, '-output', (Join-Path $stage 'vanilla'))
    $identity = Get-Content -LiteralPath (Join-Path $stage 'vanilla/version.json') -Raw | ConvertFrom-Json
    if ($identity.protocol_version -ne $versionMetadata.version) { throw 'minecraft-data protocol mismatch' }
    Copy-Item -LiteralPath (Join-Path $stage 'vanilla/reports/registries.json') -Destination (Join-Path $stage 'registries.json')
    foreach ($pair in @(@('data/minecraft', 'datapacks'), @('data/minecraft/tags', 'tags'), @('data/minecraft/loot_table', 'loot_tables'), @('data/minecraft/recipe', 'recipes'))) {
        Copy-Item -LiteralPath (Join-Path $stage "vanilla/$($pair[0])") -Destination (Join-Path $stage $pair[1]) -Recurse
    }
    $counts = @{}
    foreach ($pair in @(@('vanilla/data/minecraft', 5000), @('recipes', 1000), @('loot_tables', 1000), @('vanilla/data/minecraft/advancement', 1000), @('vanilla/data/minecraft/structure', 1000))) {
        $counts[$pair[0]] = @(Get-ChildItem -LiteralPath (Join-Path $stage $pair[0]) -Recurse -File).Count
        if ($counts[$pair[0]] -lt $pair[1]) { throw "Incomplete generated resources: $($pair[0])" }
    }
    Write-VitisText (Join-Path $stage 'checksums.sha256') (Get-VitisInventory $stage @('.') @('checksums.sha256'))
    $actual = @(Get-ChildItem -LiteralPath (Join-Path $root 'internal/data/generator') -Recurse -Filter main.go -File)
    if ($actual.Count -ne $manifest.generators.Count) { throw 'Generator inventory changed; update generation_manifest.json.' }
    foreach ($generator in $manifest.generators) {
        if (-not (Test-Path -LiteralPath (Join-Path $root "internal/data/generator/$generator/main.go"))) { throw "Missing generator: $generator" }
    }
    [System.IO.Directory]::CreateDirectory($backup) | Out-Null
    $outputs = @(".mcdata/$Version", 'internal/data/generated') + $manifest.generated_go + $manifest.fixtures
    if (-not $sourceCached) { $outputs += @($sourceArtifacts | ForEach-Object { ".mc-decompiled/$_" }) }
    $records = @(New-VitisSnapshot $root $outputs $backup)
    $publishing = $true
    if (-not $sourceCached) {
        foreach ($artifact in $sourceArtifacts) {
            $destination = Resolve-VitisOutput $root ".mc-decompiled/$artifact"
            if (Test-Path -LiteralPath $destination) { Remove-Item -LiteralPath $destination -Recurse -Force }
            Move-Item -LiteralPath (Join-Path $sourceStage $artifact) -Destination $destination
        }
    }
    if (Test-Path -LiteralPath $data) { Remove-Item -LiteralPath $data -Recurse -Force }
    Move-Item -LiteralPath $stage -Destination $data
    foreach ($generator in $manifest.generators) {
        if ($generator -eq 'worldgen') {
            Invoke-VitisCommand go @('run', './internal/data/generator/worldgen', '-version', $Version, '-resources', (Join-Path $data 'vanilla/data/minecraft/worldgen'))
        }
        else { Invoke-VitisCommand go @('run', "internal/data/generator/$generator/main.go", '-version', $Version) }
    }
    $generatedFiles = @(Get-ChildItem -LiteralPath (Join-Path $root 'internal/data/generated') -Recurse -File -Filter '*.go' | ForEach-Object FullName)
    Invoke-VitisCommand gofmt (@('-w') + $generatedFiles + $manifest.generated_go)
    Invoke-VitisCommand pwsh @('-NoProfile', '-File', (Join-Path $PSScriptRoot 'verify.ps1'))
    Invoke-VitisCommand git @('diff', '--check')
    $success = $true
    Write-Host "Data generation completed for ${Version}: resources=$($counts['vanilla/data/minecraft']) recipes=$($counts.recipes) loot=$($counts.loot_tables)"
}
finally {
    try {
        if ($publishing -and -not $success) {
            Write-Warning 'Generation failed; restoring previous data and generated outputs.'
            try { Restore-VitisSnapshot $root $records }
            catch { $restoreFailed = $true; Write-Warning "Restoration incomplete; recovery snapshot retained at $backup. $_" }
        }
        foreach ($temporary in @($stage, $sourceStage, $scratch)) {
            if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
        }
        if (-not $restoreFailed -and (Test-Path -LiteralPath $backup)) { Remove-Item -LiteralPath $backup -Recurse -Force }
        Remove-Item -LiteralPath $lock -Force
        if ($restoreFailed) { throw "Restoration incomplete; recover files from $backup" }
    }
    finally { $client.Dispose(); Pop-Location }
}
