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
Initialize-DaygoFrontend -DependencyMode ci -ForceBuild

Push-Location (Join-Path $DaygoRootDir 'cmd\daygo')
try {
    Write-Host 'Building Daygo for windows/amd64...'
    Invoke-DaygoNative go @(
        'run', $DaygoWailsPackage, 'build',
        '-platform', 'windows/amd64'
    )
}
finally {
    Pop-Location
}

$exe = Join-Path $DaygoRootDir 'build\bin\Daygo.exe'
$dll = Join-Path $DaygoRootDir 'build\bin\daygo_windows_native.dll'
if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
    throw "Windows build completed without the expected executable: $exe"
}
if (-not (Test-Path -LiteralPath $dll -PathType Leaf)) {
    throw "Windows build completed without the required capture helper: $dll"
}

Write-Host "Built $exe"
Write-Host "Bundled $dll"
