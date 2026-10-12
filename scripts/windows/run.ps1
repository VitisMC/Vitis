#requires -Version 7.2
<#
.SYNOPSIS
Builds Vitis and forwards server arguments and console input to its binary.
#>

$ErrorActionPreference = 'Stop'
$ServerArguments = $args
Push-Location (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
try {
    if (-not (Get-Command go -CommandType Application -ErrorAction SilentlyContinue)) {
        throw 'Go is required on PATH.'
    }
    & go build -o vitis.exe ./cmd/vitis
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & (Join-Path (Get-Location) 'vitis.exe') @ServerArguments
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
