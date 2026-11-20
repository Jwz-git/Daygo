# Exercise the production build function with anonymous metadata and tool doubles.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-common.ps1')
$fixture = Join-Path ([System.IO.Path]::GetTempPath()) ('daygo-flags-' + [guid]::NewGuid())
$previousDev = $env:DAYGO_DEV
try {
    New-Item -ItemType Directory -Path (Join-Path $fixture 'build\windows') -Force | Out-Null
    Copy-Item (Join-Path $DaygoRootDir 'build\windows\info.json') (Join-Path $fixture 'build\windows\info.json')
    $DaygoRootDir = $fixture
    function Install-DaygoDwarf5Workaround { return $env:GOEXPERIMENT }
    function Invoke-DaygoNative {
        param([string]$FilePath, [string[]]$ArgumentList)
        if ($ArgumentList[0] -eq 'build') { $script:buildArguments = $ArgumentList }
    }
    foreach ($development in @($false, $true)) {
        $env:DAYGO_DEV = if ($development) { '1' } else { '0' }
        Build-DaygoWindowsDesktop
        $flags = $buildArguments[[array]::IndexOf($buildArguments, '-ldflags') + 1] -split ' '
        if (($flags -join ' ') -notmatch '-H windowsgui') { throw 'GUI subsystem flag missing' }
        foreach ($flag in @('-s', '-w')) {
            if (($flags -contains $flag) -eq $development) { throw "Incorrect dev=$development linker flags: $flags" }
        }
        $tags = $buildArguments[[array]::IndexOf($buildArguments, '-tags') + 1] -split ','
        if (($tags -contains 'production') -eq $development) { throw 'Incorrect production tag' }
    }
    Write-Host 'Windows production/dev build arguments passed'
} finally {
    if ($null -eq $previousDev) { Remove-Item Env:DAYGO_DEV -ErrorAction SilentlyContinue }
    else { $env:DAYGO_DEV = $previousDev }
    Remove-Item -LiteralPath $fixture -Recurse -Force -ErrorAction SilentlyContinue
}
