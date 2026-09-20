Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'windows-common.ps1')

Initialize-DaygoWindowsNative
Initialize-DaygoFrontend -DependencyMode install

Push-Location (Join-Path $DaygoRootDir 'cmd\daygo')
try {
    Write-Host 'Starting wails dev (the first run may take a while to download the Wails CLI)...'
    # Go 1.25's Windows linker can emit malformed PE files for cgo debug
    # builds with DWARF 5 (golang/go#75077). Production builds are unaffected.
    $restoreExperiment = Install-DaygoDwarf5Workaround
    try {
        & go run $DaygoWailsPackage dev -s @args
        $wailsExitCode = $LASTEXITCODE
    }
    finally {
        Restore-DaygoExperiment -Previous $restoreExperiment
    }
    exit $wailsExitCode
}
finally {
    Pop-Location
}
