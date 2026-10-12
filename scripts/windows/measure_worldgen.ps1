#requires -Version 7.2
<#
.SYNOPSIS
Records child CPU time, peak working set and elapsed time as JSON on Windows.
.PARAMETER Output
Destination JSON file, relative to the caller's working directory.
.PARAMETER Program
Benchmark executable; remaining arguments are forwarded unchanged.
#>
param([string]$Output, [string]$Program)

$ErrorActionPreference = 'Stop'
if (-not $Output -or -not $Program) { throw 'Usage: measure_worldgen.ps1 output.json program [arguments...]' }
$application = Get-Command $Program -CommandType Application -ErrorAction Stop | Select-Object -First 1
$start = [System.Diagnostics.ProcessStartInfo]::new()
$start.FileName = $application.Source
$start.WorkingDirectory = (Get-Location).Path
$start.UseShellExecute = $false
foreach ($argument in $args) { $start.ArgumentList.Add($argument) }
$child = [System.Diagnostics.Process]::new()
$child.StartInfo = $start
$clock = [System.Diagnostics.Stopwatch]::StartNew()
$samples = [System.Collections.Generic.List[object]]::new()
$started = $false
$peak = 0L
$userSeconds = 0.0
$systemSeconds = 0.0
$nextSample = 10.0
try {
    $started = $child.Start()
    if (-not $started) { throw "Could not start $Program" }
    while (-not $child.WaitForExit(200)) {
        $child.Refresh()
        if ($child.HasExited) { break }
        $peak = [Math]::Max($peak, $child.PeakWorkingSet64)
        $userSeconds = $child.UserProcessorTime.TotalSeconds
        $systemSeconds = $child.PrivilegedProcessorTime.TotalSeconds
        if ($clock.Elapsed.TotalSeconds -ge $nextSample -and $samples.Count -lt 10000) {
            $samples.Add(@{ elapsed_seconds = $clock.Elapsed.TotalSeconds; rss_kib = [long]($child.WorkingSet64 / 1024); mean_cpu_khz = 0; cpu_milli_celsius = 0 })
            $nextSample += 10
        }
    }
    if ($IsWindows) {
        $peak = [Math]::Max($peak, $child.PeakWorkingSet64)
        $userSeconds = $child.UserProcessorTime.TotalSeconds
        $systemSeconds = $child.PrivilegedProcessorTime.TotalSeconds
    }
    $metrics = @{
        user_seconds = $userSeconds; system_seconds = $systemSeconds
        peak_rss_kib = [long]($peak / 1024); wall_seconds = $clock.Elapsed.TotalSeconds
        success = $child.ExitCode -eq 0; samples = $samples.ToArray()
    }
    [System.IO.File]::WriteAllText([System.IO.Path]::GetFullPath($Output), ($metrics | ConvertTo-Json -Depth 4) + "`n", [System.Text.UTF8Encoding]::new($false))
    exit $child.ExitCode
}
finally {
    try {
        if ($started -and -not $child.HasExited) { $child.Kill($true); $child.WaitForExit() }
    }
    finally { $clock.Stop(); $child.Dispose() }
}
