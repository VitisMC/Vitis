#requires -Version 7.2
<#
.SYNOPSIS
Tests download safety, dirty-file rollback and cancellation without Pester or network access.
#>
param([string]$Root)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'generation_transaction.ps1')
. (Join-Path $PSScriptRoot 'download.ps1')

# Assert-Test fails the test process when an invariant is not satisfied.
function Assert-Test {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

# Assert-Failure requires the supplied operation to fail explicitly.
function Assert-Failure {
    param([scriptblock]$Operation)
    $failed = $false
    try { & $Operation } catch { $failed = $true }
    Assert-Test $failed 'Expected an explicit failure'
}

foreach ($name in @('generated', 'fixtures', 'backup')) { [System.IO.Directory]::CreateDirectory((Join-Path $Root $name)) | Out-Null }
foreach ($name in @('generated/dirty.go', 'fixtures/reference.json', 'keep')) { Write-VitisText (Join-Path $Root $name) 'uncommitted original' }
$records = @(New-VitisSnapshot $Root @('generated', 'new.json', 'fixtures/reference.json') (Join-Path $Root 'backup'))
try {
    Write-VitisText (Join-Path $Root 'generated/dirty.go') 'changed'
    Write-VitisText (Join-Path $Root 'generated/new.go') 'created'
    Write-VitisText (Join-Path $Root 'new.json') 'created'
    Write-VitisText (Join-Path $Root 'fixtures/reference.json') 'changed'
    Assert-Failure { Invoke-VitisCommand pwsh @('-NoProfile', '-Command', 'exit 42') }
}
finally { Restore-VitisSnapshot $Root $records }
foreach ($name in @('generated/dirty.go', 'fixtures/reference.json', 'keep')) {
    Assert-Test ([System.IO.File]::ReadAllText((Join-Path $Root $name)) -ceq 'uncommitted original') "Lost original $name"
}
Assert-Test (-not (Test-Path -LiteralPath (Join-Path $Root 'new.json'))) 'Created output survived rollback'
Assert-Test (-not (Test-Path -LiteralPath (Join-Path $Root 'generated/new.go'))) 'Created nested output survived rollback'
Assert-Failure { Resolve-VitisOutput $Root '../outside' }
$link = Join-Path $Root 'unsafe-link'
$linkType = if ($IsWindows) { 'Junction' } else { 'SymbolicLink' }
New-Item -ItemType $linkType -Path $link -Target (Join-Path $Root 'generated') | Out-Null
Assert-Failure { Assert-VitisTree $link }
Assert-Failure { Resolve-VitisOutput $Root 'unsafe-link/dirty.go' }
Remove-Item -LiteralPath $link -Force

$inventory = Get-VitisInventory $Root @('generated')
Write-VitisText (Join-Path $Root 'generated/addition.go') 'unexpected addition'
Assert-Test ((Get-VitisInventory $Root @('generated')) -cne $inventory) 'Inventory did not detect an added file'
Remove-Item -LiteralPath (Join-Path $Root 'generated/addition.go')
Assert-Test ((Get-VitisInventory $Root @('generated')) -ceq $inventory) 'Inventory ordering changed'

Add-Type -TypeDefinition @'
using System;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
// DownloadHandler supplies deterministic responses without trusting external services.
public sealed class DownloadHandler : HttpMessageHandler {
    public byte[] Bytes = new byte[] {1, 2, 3, 4};
    public int Status = 200;
    public int Calls;
    public bool Redirect;
    public bool Delay;
    public Task<HttpResponseMessage> Pending;
    // SendAsync returns a controlled response or a cancellable pending request.
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken token) {
        Calls++;
        if (Delay) return Pending = Delayed(token);
        var response = new HttpResponseMessage((HttpStatusCode)Status);
        response.Content = new ByteArrayContent(Bytes);
        if (Redirect) response.Headers.Location = new Uri("http://unsafe.example/resource");
        return Task.FromResult(response);
    }
    // Delayed models a download that cannot complete before cancellation.
    private async Task<HttpResponseMessage> Delayed(CancellationToken token) {
        await Task.Delay(600000, token);
        return new HttpResponseMessage(HttpStatusCode.OK);
    }
}
'@
$handler = [DownloadHandler]::new()
$client = [System.Net.Http.HttpClient]::new($handler)
$destination = Join-Path $Root 'download.bin'
$expected = [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($handler.Bytes)).ToLowerInvariant()
try {
    Get-VitisDownload $client 'https://example.test/artifact' $destination SHA256 $expected 4
    Assert-Test ($handler.Calls -eq 1) 'Successful download failed'
    Get-VitisDownload $client 'https://example.test/artifact' $destination SHA256 $expected 4
    Assert-Test ($handler.Calls -eq 1) 'Verified cache was downloaded again'
    $original = [System.IO.File]::ReadAllBytes($destination)
    $handler.Bytes = [byte[]]@(5, 6, 7, 8)
    Assert-Failure { Get-VitisDownload $client 'https://example.test/artifact' $destination SHA256 ('0' * 64) 4 }
    Assert-Test ([Convert]::ToHexString([System.IO.File]::ReadAllBytes($destination)) -ceq [Convert]::ToHexString($original)) 'Failed checksum replaced existing artifact'
    $handler.Status = 500
    Assert-Failure { Get-VitisDownload $client 'https://example.test/artifact' (Join-Path $Root 'http-error') }
    $handler.Status = 302
    $handler.Redirect = $true
    Assert-Failure { Get-VitisDownload $client 'https://example.test/artifact' (Join-Path $Root 'redirect') }
    $handler.Status = 200
    $handler.Redirect = $false
    Assert-Failure { Get-VitisDownload $client 'https://example.test/artifact' (Join-Path $Root 'oversized') -Limit 2 }
    Assert-Test (@(Get-ChildItem -LiteralPath $Root -Filter '*.partial.*').Count -eq 0) 'Partial download survived failure'
    $handler.Delay = $true
    $handler.Calls = 0
    $downloadRunner = [powershell]::Create()
    try {
        [void]$downloadRunner.AddScript(@'
param($Helpers, $Download, $Client, $Destination)
. $Helpers
. $Download
Get-VitisDownload $Client 'https://example.test/artifact' $Destination
'@).AddArgument((Join-Path $PSScriptRoot 'generation_transaction.ps1')).AddArgument((Join-Path $PSScriptRoot 'download.ps1')).AddArgument($client).AddArgument((Join-Path $Root 'cancelled-download'))
        $pendingDownload = $downloadRunner.BeginInvoke()
        $deadline = [DateTime]::UtcNow.AddSeconds(15)
        while (-not $handler.Pending -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 50 }
        Assert-Test ($null -ne $handler.Pending) 'Cancellation download did not start'
        $downloadRunner.Stop()
        try { [void]$downloadRunner.EndInvoke($pendingDownload) } catch [System.Management.Automation.PipelineStoppedException] { }
        Assert-Test $handler.Pending.IsCanceled 'Stopping download did not cancel HTTP request'
        Assert-Test (-not (Test-Path -LiteralPath (Join-Path $Root 'cancelled-download'))) 'Cancelled download was published'
    }
    finally { $downloadRunner.Stop(); $downloadRunner.Dispose() }
}
finally { $client.Dispose() }

$ready = Join-Path $Root 'ready'
$childScript = Join-Path $Root 'child.ps1'
Write-VitisText $childScript 'param($Ready); [System.IO.File]::WriteAllText($Ready, [string]$PID); Start-Sleep -Seconds 600'
Write-VitisText (Join-Path $Root 'generated/dirty.go') 'changed before cancellation'
$runner = [powershell]::Create()
try {
    [void]$runner.AddScript(@'
param($Helper, $Root, $Records, $Child, $Ready)
. $Helper
try { Invoke-VitisCommand pwsh @('-NoProfile', '-File', $Child, $Ready) }
finally { Restore-VitisSnapshot $Root $Records }
'@).AddArgument((Join-Path $PSScriptRoot 'generation_transaction.ps1')).AddArgument($Root).AddArgument($records).AddArgument($childScript).AddArgument($ready)
    $pending = $runner.BeginInvoke()
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    while (-not (Test-Path -LiteralPath $ready) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 50 }
    Assert-Test (Test-Path -LiteralPath $ready) 'Cancellation child did not start'
    $childID = [int][System.IO.File]::ReadAllText($ready)
    $runner.Stop()
    try { [void]$runner.EndInvoke($pending) } catch [System.Management.Automation.PipelineStoppedException] { }
    Assert-Failure { [System.Diagnostics.Process]::GetProcessById($childID) }
    Assert-Test ([System.IO.File]::ReadAllText((Join-Path $Root 'generated/dirty.go')) -ceq 'uncommitted original') 'Cancellation did not restore dirty output'
}
finally { $runner.Stop(); $runner.Dispose() }
Write-Host 'PowerShell download, rollback and cancellation tests passed.'
