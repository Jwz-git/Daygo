Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$DaygoRootDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$DaygoFrontendDir = Join-Path $DaygoRootDir 'frontend'
$DaygoFrontendDist = Join-Path $DaygoFrontendDir 'dist'
$DaygoWailsVersion = 'v3.1.0'
$DaygoWailsPackage = "github.com/wailsapp/wails/v3/cmd/wails3@$DaygoWailsVersion"

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

function Assert-DaygoPortableExecutable {
    param([Parameter(Mandatory = $true)][string]$Path)

    Assert-DaygoTool -Name objdump -Hint 'Install MinGW-w64 binutils alongside g++.'
    $imports = & objdump -p $Path
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to inspect PE imports: $Path"
    }
    $runtime = @($imports | Select-String -Pattern '^\s*DLL Name:\s*(libstdc\+\+-6|libgcc_s_[^\s]+|libwinpthread-1)\.dll\s*$' -CaseSensitive:$false)
    if ($runtime.Count -gt 0) {
        $names = @($runtime | ForEach-Object { $_.Matches[0].Groups[1].Value + '.dll' }) -join ', '
        throw "Unbundled MinGW runtime dependency in ${Path}: $names"
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
    Push-Location $DaygoRootDir
    try {
        Invoke-DaygoNative go @('run', $DaygoWailsPackage, 'generate', 'bindings', '-ts', '-i', '-names', '-f', '-tags=daygo_desktop,server', '-d', 'frontend/bindings', './cmd/daygo')
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

# Compile directly with Go; Wails v3 no longer accepts v2 build/NSIS flags.
function Build-DaygoWindowsDesktop {
    param([string]$Version = '0.0.0')
    $temporary = Join-Path $DaygoRootDir '.tmp\windows-build'
    New-Item -ItemType Directory -Path $temporary -Force | Out-Null
    $info = Join-Path $temporary 'info.json'
    $metadata = Get-Content -LiteralPath (Join-Path $DaygoRootDir 'build\windows\info.json') -Raw | ConvertFrom-Json
    $metadata.fixed.file_version = $Version
    $metadata.info.'0000'.ProductVersion = $Version
    [System.IO.File]::WriteAllText($info, ($metadata | ConvertTo-Json -Depth 5), (New-Object System.Text.UTF8Encoding($false)))
    $resource = Join-Path $DaygoRootDir 'cmd\daygo\daygo_windows_amd64.syso'
    $previousCgo = $env:CGO_ENABLED
    $previousArch = $env:GOARCH
    $previousExperiment = Install-DaygoDwarf5Workaround
    Push-Location $DaygoRootDir
    try {
        Invoke-DaygoNative go @('run', $DaygoWailsPackage, 'generate', 'syso', '-arch', 'amd64',
            '-icon', 'build/windows/icon.ico', '-manifest', 'build/windows/wails.exe.manifest',
            '-info', $info, '-out', $resource)
        $env:CGO_ENABLED = '1'
        $env:GOARCH = 'amd64'
        $tags = 'daygo_desktop'
        $linkerFlags = '-H windowsgui'
        if ($env:DAYGO_DEV -ne '1') {
            $tags += ',production'
            $linkerFlags += ' -s -w'
        }
        if ($env:EXTRA_TAGS) { $tags += ',' + $env:EXTRA_TAGS }
        Invoke-DaygoNative go @('build', '-tags', $tags, '-ldflags', $linkerFlags, '-o',
            (Join-Path $DaygoRootDir 'build\bin\Daygo.exe'), './cmd/daygo')
    }
    finally {
        Pop-Location
        Remove-Item -LiteralPath $resource -Force -ErrorAction SilentlyContinue
        if ($null -eq $previousCgo) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue }
        else { $env:CGO_ENABLED = $previousCgo }
        if ($null -eq $previousArch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue }
        else { $env:GOARCH = $previousArch }
        Restore-DaygoExperiment -Previous $previousExperiment
    }
}
