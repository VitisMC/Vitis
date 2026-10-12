#requires -Version 7.2
<#
.SYNOPSIS
Provides bounded child-process execution and transactional generation snapshots.
#>

# Invoke-VitisCommand preserves argument boundaries and stops descendants before rollback.
function Invoke-VitisCommand {
    param([string]$Program, [string[]]$Arguments = @())
    $application = Get-Command $Program -CommandType Application -ErrorAction Stop | Select-Object -First 1
    $start = [System.Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $application.Source
    $start.WorkingDirectory = (Get-Location).Path
    $start.UseShellExecute = $false
    foreach ($argument in $Arguments) { $start.ArgumentList.Add($argument) }
    $child = [System.Diagnostics.Process]::new()
    $child.StartInfo = $start
    $started = $false
    try {
        $started = $child.Start()
        if (-not $started) { throw "Could not start $Program" }
        while (-not $child.WaitForExit(200)) { }
        if ($child.ExitCode -ne 0) { throw "$Program exited with code $($child.ExitCode)" }
    }
    finally {
        try {
            if ($started -and -not $child.HasExited) {
                $child.Kill($true)
                if (-not $child.WaitForExit(10000)) { throw "Could not stop $Program before rollback" }
            }
        }
        finally { $child.Dispose() }
    }
}

# Assert-VitisTree rejects symlinks and reparse points before copying or hashing a tree.
function Assert-VitisTree {
    param([string]$Path)
    $entry = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    $entries = @($entry)
    if ($entry.PSIsContainer) { $entries += @(Get-ChildItem -LiteralPath $Path -Force -Recurse) }
    foreach ($item in $entries) {
        if ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) {
            throw "Unsafe symlink or reparse point: $($item.FullName)"
        }
    }
}

# Resolve-VitisOutput validates every parent of a declared repository-relative output.
function Resolve-VitisOutput {
    param([string]$Root, [string]$Relative)
    if ([System.IO.Path]::IsPathRooted($Relative) -or $Relative -match '(^|[/\\])\.\.?([/\\]|$)|:') {
        throw "Unsafe generation output: $Relative"
    }
    $current = $Root
    foreach ($component in $Relative -split '[/\\]') {
        $current = Join-Path $current $component
        if (Test-Path -LiteralPath $current) {
            $entry = Get-Item -LiteralPath $current -Force
            if ($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint) {
                throw "Unsafe generation output: $Relative"
            }
        }
    }
    return $current
}

# New-VitisSnapshot records uncommitted contents and originally absent outputs.
function New-VitisSnapshot {
    param([string]$Root, [string[]]$Paths, [string]$Backup)
    $records = @()
    foreach ($relative in $Paths) {
        $target = Resolve-VitisOutput $Root $relative
        $saved = Join-Path $Backup ([string]$records.Count)
        $present = Test-Path -LiteralPath $target
        if ($present) {
            Assert-VitisTree $target
            Copy-Item -LiteralPath $target -Destination $saved -Recurse -Force
        }
        $records += [pscustomobject]@{ Relative = $relative; Saved = $saved; Present = $present }
    }
    return $records
}

# Restore-VitisSnapshot removes only declared outputs and restores their original contents.
function Restore-VitisSnapshot {
    param([string]$Root, [object[]]$Records)
    $failures = @()
    foreach ($record in $Records) {
        try {
            $target = Resolve-VitisOutput $Root $record.Relative
            if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Recurse -Force }
            if ($record.Present) {
                [System.IO.Directory]::CreateDirectory((Split-Path -Parent $target)) | Out-Null
                Copy-Item -LiteralPath $record.Saved -Destination $target -Recurse -Force
            }
        }
        catch { $failures += $_.Exception.Message }
    }
    if ($failures.Count) { throw ($failures -join [Environment]::NewLine) }
}

# Write-VitisText produces UTF-8 without a BOM and stable LF line endings.
function Write-VitisText {
    param([string]$Path, [string]$Text)
    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

# Get-VitisInventory hashes a sorted complete inventory in the Bash updater's format.
function Get-VitisInventory {
    param([string]$Root, [string[]]$Paths, [string[]]$Exclude = @())
    $files = [System.Collections.Generic.Dictionary[string,string]]::new([System.StringComparer]::Ordinal)
    foreach ($relative in $Paths) {
        $path = Join-Path $Root $relative
        Assert-VitisTree $path
        $entry = Get-Item -LiteralPath $path -Force
        $entries = if ($entry.PSIsContainer) { @(Get-ChildItem -LiteralPath $path -Recurse -Force -File) } else { @($entry) }
        foreach ($item in $entries) {
            $name = [System.IO.Path]::GetRelativePath($Root, $item.FullName).Replace('\', '/')
            if ($name -notin $Exclude) {
                if ($relative -eq '.') { $name = "./$name" }
                $files[$name] = $item.FullName
            }
        }
    }
    $names = [string[]]@($files.Keys)
    [Array]::Sort($names, [System.StringComparer]::Ordinal)
    $text = [System.Text.StringBuilder]::new()
    foreach ($name in $names) {
        $hash = (Get-FileHash -LiteralPath $files[$name] -Algorithm SHA256).Hash.ToLowerInvariant()
        [void]$text.Append("$hash  $name`n")
    }
    return $text.ToString()
}
