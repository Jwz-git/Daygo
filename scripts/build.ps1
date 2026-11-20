param(
    [switch]$RunSmoke
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'windows-common.ps1')

$binDir = Join-Path $DaygoRootDir 'build\bin'
if (Test-Path -LiteralPath $binDir) {
    Remove-Item -LiteralPath $binDir -Recurse -Force
}
Initialize-DaygoWindowsNative -RunSmoke:$RunSmoke
if ($env:DAYGO_DEV -eq '1') {
    Push-Location $DaygoRootDir
    try { Invoke-DaygoNative go @('run', $DaygoWailsPackage, 'generate', 'bindings', '-ts', '-i', '-names', '-f', '-tags=daygo_desktop,server', '-d', 'frontend/bindings', './cmd/daygo') }
    finally { Pop-Location }
} else { Initialize-DaygoFrontend -DependencyMode ci -ForceBuild }

Build-DaygoWindowsDesktop

$exe = Join-Path $DaygoRootDir 'build\bin\Daygo.exe'
$dll = Join-Path $DaygoRootDir 'build\bin\daygo_windows_native.dll'
if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
    throw "Windows build completed without the expected executable: $exe"
}
if (-not (Test-Path -LiteralPath $dll -PathType Leaf)) {
    throw "Windows build completed without the required capture helper: $dll"
}
Assert-DaygoPortableExecutable -Path $exe

Write-Host "Built $exe"
Write-Host "Bundled $dll"
