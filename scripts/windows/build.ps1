#requires -Version 7.2
<#
.SYNOPSIS
Builds the complete Vitis server package from the repository root.
#>
param([string]$Output = 'vitis.exe')

$ErrorActionPreference = 'Stop'
Push-Location (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
try {
    if (-not (Get-Command go -CommandType Application -ErrorAction SilentlyContinue)) {
        throw 'Go is required on PATH.'
    }
    & go build -trimpath -ldflags '-s -w' -o $Output ./cmd/vitis
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "Built $Output ($((Get-Item -LiteralPath $Output).Length) bytes)"
}
finally {
    Pop-Location
}
