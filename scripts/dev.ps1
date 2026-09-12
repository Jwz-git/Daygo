Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RootDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$FrontendDir = Join-Path $RootDir 'frontend'
$WailsVersion = 'v2.15.0'
$WailsPackage = "github.com/wailsapp/wails/v2/cmd/wails@$WailsVersion"

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,

        [Parameter(Mandatory = $false)]
        [string[]]$ArgumentList = @()
    )

    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed ($LASTEXITCODE): $FilePath $($ArgumentList -join ' ')"
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is required but was not found in PATH.'
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw 'npm is required but was not found in PATH.'
}

Write-Host 'Syncing frontend dependencies...'
Push-Location $FrontendDir
try {
    # Run npm from the frontend directory. npm 10 on Windows can ignore
    # `--prefix` when invoked through npm.ps1.
    Invoke-Native npm @('install', '--no-audit', '--no-fund')

}
finally {
    Pop-Location
}

# frontend/dist/ must contain something or `go:embed all:dist` fails to compile
# and no Go command runs at all. Seed it ONLY when there is no entry point:
# writing one unconditionally would destroy a real bundle's index.html while
# leaving its assets/ in place, and the app then serves a blank page.
$FrontendDist = Join-Path $FrontendDir 'dist'
$FrontendEntry = Join-Path $FrontendDist 'index.html'
New-Item -ItemType Directory -Path $FrontendDist -Force | Out-Null
if (-not (Test-Path -LiteralPath $FrontendEntry -PathType Leaf)) {
    Set-Content -LiteralPath $FrontendEntry -Value '<!doctype html>' -NoNewline
}

# frontend/wailsjs/ is generated and not committed, but src/api/*.ts imports it,
# so vue-tsc and vite both fail without it. It drifts as soon as a binding is
# added, renamed or removed, and a stale copy produces errors that name the
# missing member rather than the stale file — so regenerate unconditionally
# instead of testing for existence. scripts/bootstrap-frontend.sh applies the
# same rule on macOS/Linux.
Write-Host 'Generating Wails frontend bindings...'
Push-Location (Join-Path $RootDir 'cmd\daygo')
try {
    Invoke-Native go @('run', $WailsPackage, 'generate', 'module')
}
finally {
    Pop-Location
}

# Build the real bundle. The test is the entry point's content: a real bundle's
# index.html references its hashed asset files and a seed does not, whereas
# assets/ can survive a seed write and would make this skip the rebuild that is
# needed. A marker file would work too, except that losing it leaves the seed in
# place permanently and the app blank.
$HasRealBundle = $false
if (Test-Path -LiteralPath $FrontendEntry -PathType Leaf) {
    if (Select-String -LiteralPath $FrontendEntry -Pattern 'assets/' -Quiet) {
        $HasRealBundle = $true
    }
}
if (-not $HasRealBundle) {
    Write-Host 'Building frontend bundle for go:embed...'
    Push-Location $FrontendDir
    try {
        Invoke-Native npm @('run', 'build')
    }
    finally {
        Pop-Location
    }
}

Push-Location (Join-Path $RootDir 'cmd\daygo')
try {
    Write-Host 'Starting wails dev (the first run may take a while to download the Wails CLI)...'
    # Go 1.25's Windows linker can emit malformed PE files for cgo debug
    # builds when DWARF v5 is enabled (golang/go#75077). Wails dev enables
    # debug symbols, and Daygo uses cgo for the native capture ABI, so retain
    # debuggability with the older DWARF layout until the toolchain fix lands.
    $PreviousGoExperiment = $env:GOEXPERIMENT
    $GoVersion = (& go env GOVERSION).Trim()
    if ($GoVersion -match '^go1\.25(?:\.|$)') {
        $ExperimentList = @(
            $PreviousGoExperiment -split ',' |
                Where-Object { $_ -and $_ -notin @('dwarf5', 'nodwarf5') }
        )
        $env:GOEXPERIMENT = (@($ExperimentList) + 'nodwarf5') -join ','
        Write-Host 'Applying Go 1.25 Windows cgo workaround: GOEXPERIMENT=nodwarf5'
    }
    try {
        & go run $WailsPackage dev -s @args
        $WailsExitCode = $LASTEXITCODE
    }
    finally {
        if ($null -eq $PreviousGoExperiment) {
            Remove-Item Env:GOEXPERIMENT -ErrorAction SilentlyContinue
        }
        else {
            $env:GOEXPERIMENT = $PreviousGoExperiment
        }
    }
    exit $WailsExitCode
}
finally {
    Pop-Location
}
