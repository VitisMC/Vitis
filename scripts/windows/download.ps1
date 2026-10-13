#requires -Version 7.2
<#
.SYNOPSIS
Downloads bounded HTTPS artifacts and publishes only verified complete files.
#>

# Get-VitisDownload validates cached artifacts, follows HTTPS redirects and stages transfers.
function Get-VitisDownload {
    param(
        [System.Net.Http.HttpClient]$Client,
        [string]$Url,
        [string]$Destination,
        [string]$Algorithm = '',
        [string]$Expected = '',
        [long]$Size = 0,
        [long]$Limit = 33554432
    )
    if ($Algorithm -and (Test-Path -LiteralPath $Destination -PathType Leaf)) {
        Assert-VitisTree $Destination
        $entry = Get-Item -LiteralPath $Destination
        if (($Size -eq 0 -or $entry.Length -eq $Size) -and $entry.Length -le $Limit -and
            (Get-FileHash -LiteralPath $Destination -Algorithm $Algorithm).Hash -ieq $Expected) { return }
    }
    if ($Size -gt $Limit -or $Size -lt 0) { throw "Invalid download size: $Url" }
    $temporary = "$Destination.partial.$([guid]::NewGuid().ToString('N'))"
    try {
        for ($attempt = 0; $attempt -lt 4; $attempt++) {
            $response = $null
            $stream = $null
            $file = $null
            $deadline = [System.Threading.CancellationTokenSource]::new([TimeSpan]::FromMinutes(3))
            try {
                $location = [uri]$Url
                for ($redirect = 0; $redirect -le 8; $redirect++) {
                    if ($location.Scheme -ne 'https') { throw "Only HTTPS downloads are allowed: $location" }
                    $pending = $Client.GetAsync($location, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead, $deadline.Token)
                    while (-not $pending.IsCompleted) { Start-Sleep -Milliseconds 50 }
                    $response = $pending.GetAwaiter().GetResult()
                    if ([int]$response.StatusCode -notin @(301, 302, 303, 307, 308)) { break }
                    if ($redirect -eq 8 -or -not $response.Headers.Location) { throw "Invalid download redirect: $location" }
                    $location = [uri]::new($location, $response.Headers.Location)
                    $response.Dispose()
                    $response = $null
                }
                [void]$response.EnsureSuccessStatusCode()
                $stream = $response.Content.ReadAsStream()
                $file = [System.IO.File]::Open($temporary, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
                $buffer = [byte[]]::new(32768)
                $total = 0L
                while ($true) {
                    $read = $stream.ReadAsync($buffer, 0, $buffer.Length, $deadline.Token)
                    while (-not $read.IsCompleted) { Start-Sleep -Milliseconds 50 }
                    $count = $read.GetAwaiter().GetResult()
                    if ($count -eq 0) { break }
                    $total += $count
                    if ($total -gt $Limit) { throw "Download exceeds size limit: $Url" }
                    $file.Write($buffer, 0, $count)
                }
                $file.Flush($true)
                $file.Dispose()
                $file = $null
                if ($total -eq 0 -or ($Size -gt 0 -and $total -ne $Size)) { throw "Download size mismatch: $Url" }
                if ($Algorithm -and (Get-FileHash -LiteralPath $temporary -Algorithm $Algorithm).Hash -ine $Expected) {
                    throw "Download checksum mismatch: $Url"
                }
                [System.IO.File]::Move($temporary, $Destination, $true)
                return
            }
            catch {
                if ($attempt -eq 3) { throw }
                Start-Sleep -Seconds 1
            }
            finally {
                $deadline.Cancel()
                $deadline.Dispose()
                if ($file) { $file.Dispose() }
                if ($stream) { $stream.Dispose() }
                if ($response) { $response.Dispose() }
            }
        }
    }
    finally {
        if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Force }
    }
}
