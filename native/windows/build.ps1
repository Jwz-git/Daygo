param(
    [switch]$RunSmoke
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RootDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Source = Join-Path $PSScriptRoot 'Sources\daygo_capture.cpp'
$IncludeDir = Join-Path $RootDir 'native\include'
$OutDir = Join-Path $RootDir 'build\native\windows\amd64'
$Object = Join-Path $OutDir 'daygo_capture.o'
$Archive = Join-Path $OutDir 'libdaygo_capture.a'

if (-not (Get-Command g++ -ErrorAction SilentlyContinue)) {
    throw 'MinGW-w64 g++ is required but was not found in PATH.'
}
if (-not (Get-Command ar -ErrorAction SilentlyContinue)) {
    throw 'MinGW-w64 ar is required but was not found in PATH.'
}

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

& g++ -std=c++17 -O2 -Wall -Wextra -Wpedantic -DDAYGO_CAPTURE_BUILD=1 -I $IncludeDir -c $Source -o $Object
if ($LASTEXITCODE -ne 0) {
    throw "C++ compilation failed ($LASTEXITCODE)."
}

& ar rcs $Archive $Object
if ($LASTEXITCODE -ne 0) {
    throw "Static archive creation failed ($LASTEXITCODE)."
}

Write-Host "Built $Archive"

if ($RunSmoke) {
    $SmokeSource = Join-Path $PSScriptRoot 'smoke.cpp'
    $SmokeBinary = Join-Path $OutDir 'daygo-capture-smoke.exe'
    $SmokeOutputDir = Join-Path $RootDir 'temp'
    $SmokeOutput = Join-Path $SmokeOutputDir 'windows-capture-smoke.jpg'
    New-Item -ItemType Directory -Path $SmokeOutputDir -Force | Out-Null
    & g++ -std=c++17 -O2 $SmokeSource $Archive -I $IncludeDir `
        -ld3d11 -ldxgi -ldxguid -lole32 -loleaut32 -lwindowscodecs `
        -luser32 -lgdi32 -ladvapi32 -static-libgcc -static-libstdc++ -o $SmokeBinary
    if ($LASTEXITCODE -ne 0) {
        throw "Smoke executable link failed ($LASTEXITCODE)."
    }
    Write-Host 'Running native capture smoke...'
    $env:DAYGO_SMOKE_OUTPUT = $SmokeOutput
    & $SmokeBinary
    Remove-Item Env:DAYGO_SMOKE_OUTPUT -ErrorAction SilentlyContinue
    if ($LASTEXITCODE -ne 0) {
        throw "Native capture smoke failed ($LASTEXITCODE)."
    }
}
