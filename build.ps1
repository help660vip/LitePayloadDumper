[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Go120,

    [Parameter(Mandatory = $true)]
    [string]$GoModern
)

$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$distDir = Join-Path $projectRoot 'dist'
New-Item -ItemType Directory -Force -Path $distDir | Out-Null

function Build-Target {
    param(
        [string]$GoExecutable,
        [string]$OutputName
    )

    if (-not (Test-Path -LiteralPath $GoExecutable -PathType Leaf)) {
        throw "Go executable does not exist: $GoExecutable"
    }

    $outputPath = Join-Path $distDir $OutputName
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & $GoExecutable build -trimpath -buildvcs=false -ldflags '-s -w -H=windowsgui' -o $outputPath .
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed: $OutputName"
    }
    Write-Host "Built $outputPath"
}

Push-Location $projectRoot
try {
    Build-Target -GoExecutable $Go120 -OutputName 'LitePayloadDumper_Win7.exe'
    Build-Target -GoExecutable $GoModern -OutputName 'LitePayloadDumper_Win10-11.exe'
}
finally {
    Pop-Location
}

