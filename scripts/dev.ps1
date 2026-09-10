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

# The Go binding generator compiles the project with the `bindings` build tag,
# but frontend/embed.go is still compiled and requires at least one file under
# frontend/dist. Seed that directory for a fresh checkout; Vite replaces it
# with the real bundle below.
$FrontendEntry = Join-Path $FrontendDir 'dist\index.html'
$FrontendBootstrapMarker = Join-Path $FrontendDir 'dist\.daygo-embed-bootstrap'
$NeedsFrontendBuild =
    (-not (Test-Path -LiteralPath $FrontendEntry -PathType Leaf)) -or
    (Test-Path -LiteralPath $FrontendBootstrapMarker -PathType Leaf)
if ($NeedsFrontendBuild) {
    $FrontendDist = Split-Path -Parent $FrontendEntry
    New-Item -ItemType Directory -Path $FrontendDist -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $FrontendDir 'index.html') -Destination $FrontendEntry -Force
    New-Item -ItemType File -Path $FrontendBootstrapMarker -Force | Out-Null
}

if (-not (Test-Path -LiteralPath (Join-Path $FrontendDir 'wailsjs') -PathType Container)) {
    Write-Host 'Generating Wails frontend bindings...'
    Push-Location (Join-Path $RootDir 'cmd\daygo')
    try {
        Invoke-Native go @('run', $WailsPackage, 'generate', 'module')
    }
    finally {
        Pop-Location
    }
}

if ($NeedsFrontendBuild) {
    Write-Host 'Building frontend bundle once for go:embed...'
    Push-Location $FrontendDir
    try {
        Invoke-Native npm @('run', 'build')
        if (Test-Path -LiteralPath $FrontendBootstrapMarker -PathType Leaf) {
            Remove-Item -LiteralPath $FrontendBootstrapMarker -Force
        }
    }
    finally {
        Pop-Location
    }
}

Push-Location (Join-Path $RootDir 'cmd\daygo')
try {
    Write-Host 'Starting wails dev (the first run may take a while to download the Wails CLI)...'
    & go run $WailsPackage dev -s @args
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
