Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$DaygoRootDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$DaygoFrontendDir = Join-Path $DaygoRootDir 'frontend'
$DaygoFrontendDist = Join-Path $DaygoFrontendDir 'dist'
$DaygoWailsVersion = 'v2.15.0'
$DaygoWailsPackage = "github.com/wailsapp/wails/v2/cmd/wails@$DaygoWailsVersion"

function Invoke-DaygoNative {
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

function Assert-DaygoTool {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $false)]
        [string]$Hint = ''
    )

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        $message = "$Name is required but was not found in PATH."
        if ($Hint) { $message += "`n       $Hint" }
        throw $message
    }
}

function Initialize-DaygoWindowsNative {
    param([switch]$RunSmoke)

    $arguments = @(
        '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File',
        (Join-Path $DaygoRootDir 'native\windows\build.ps1')
    )
    if ($RunSmoke) {
        $arguments += '-RunSmoke'
    }
    Invoke-DaygoNative powershell $arguments
}

function Test-DaygoFrontendBundle {
    $entry = Join-Path $DaygoFrontendDist 'index.html'
    if (-not (Test-Path -LiteralPath $entry -PathType Leaf)) {
        return $false
    }
    return [bool] (Select-String -LiteralPath $entry -Pattern 'assets/' -Quiet)
}

function Initialize-DaygoFrontend {
    param(
        [Parameter(Mandatory = $true)]
        [ValidateSet('install', 'ci')]
        [string]$DependencyMode,

        [switch]$ForceBuild
    )

    Assert-DaygoTool -Name go -Hint 'Install Go from https://go.dev/dl/ or via winget (winget install GoLang.Go).'
    Assert-DaygoTool -Name npm -Hint 'Install Node.js 20.19+ (or 22.12+) from https://nodejs.org/.'

    Write-Host 'Syncing frontend dependencies...'
    Push-Location $DaygoFrontendDir
    try {
        Invoke-DaygoNative npm @($DependencyMode, '--no-audit', '--no-fund')
    }
    finally {
        Pop-Location
    }

    $entry = Join-Path $DaygoFrontendDist 'index.html'
    New-Item -ItemType Directory -Path $DaygoFrontendDist -Force | Out-Null
    if (-not (Test-Path -LiteralPath $entry -PathType Leaf)) {
        Set-Content -LiteralPath $entry -Value '<!doctype html>' -NoNewline
    }

    Write-Host 'Generating Wails frontend bindings...'
    Push-Location (Join-Path $DaygoRootDir 'cmd\daygo')
    try {
        Invoke-DaygoNative go @('run', $DaygoWailsPackage, 'generate', 'module')
    }
    finally {
        Pop-Location
    }

    if ($ForceBuild -or -not (Test-DaygoFrontendBundle)) {
        Write-Host 'Building frontend bundle for go:embed...'
        Push-Location $DaygoFrontendDir
        try {
            Invoke-DaygoNative npm @('run', 'build')
        }
        finally {
            Pop-Location
        }
    }
}

function Install-DaygoDwarf5Workaround {
    $previous = $env:GOEXPERIMENT
    $goVersion = (& go env GOVERSION).Trim()
    if ($goVersion -notmatch '^go1\.25(?:\.|$)') {
        return $previous
    }

    Write-Host 'Applying Go 1.25 Windows cgo workaround: GOEXPERIMENT=nodwarf5'
    $previousItems = @(
        $previous -split ',' |
            Where-Object { $_ -and $_ -notin @('dwarf5', 'nodwarf5') }
    )
    $env:GOEXPERIMENT = (@($previousItems) + 'nodwarf5') -join ','
    return $previous
}

function Restore-DaygoExperiment {
    param([AllowNull()][string]$Previous)

    if ($null -eq $Previous) {
        Remove-Item Env:GOEXPERIMENT -ErrorAction SilentlyContinue
    }
    else {
        $env:GOEXPERIMENT = $Previous
    }
}
