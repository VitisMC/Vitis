#requires -Version 7.2
<#
.SYNOPSIS
Runs the same mandatory checks as make verify without requiring Make or Bash.
#>

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'generation_transaction.ps1')
Push-Location (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
try {
    Invoke-VitisCommand go @('test', './...', '-count=1')
    Invoke-VitisCommand go @('vet', './...')
    Invoke-VitisCommand go @('test', '-race', './...', '-count=1')
    Invoke-VitisCommand go @('test', './internal/protocol', '-run=^$', '-fuzz=FuzzBufferBoundedReads', '-fuzztime=1s')
    Invoke-VitisCommand go @('test', './internal/nbt', '-run=^$', '-fuzz=FuzzDecodeRootCompound', '-fuzztime=1s')
    Invoke-VitisCommand go @('test', './internal/network', '-run=^$', '-fuzz=FuzzDecompressFrame', '-fuzztime=1s')
    Invoke-VitisCommand go @('run', './cmd/vitis-doccheck')
}
finally { Pop-Location }
