# ─────────────────────────────────────────────────────────────
# Daygo Windows Packager
#
# Builds a windows/amd64 release, wraps it in an NSIS installer
# (wails build -nsis) and optionally Authenticode-signs the artifacts.
# The bash counterpart for macOS is scripts/package-macos.sh; this script
# follows the same steps (build → verify → sign → installer → sign installer)
# but uses the Windows toolchain (wails build -nsis, makensis, signtool).
#
# Usage:
#   ./scripts/package-windows.ps1
#   ./scripts/package-windows.ps1 0.1.0
#
# Run the native capture smoke first:
#   ./scripts/package-windows.ps1 0.1.0 -RunSmoke
#
# Authenticode signing (certificate file):
#   $env:DAYGO_WIN_CERT_FILE = 'C:\path\daygo.pfx'
#   $env:DAYGO_WIN_CERT_PASSWORD = '...'          # avoid: exposed on the cmdline
#   ./scripts/package-windows.ps1 0.1.0
#
# Authenticode signing (installed cert by thumbprint — preferred, no password
# on the command line):
#   $env:DAYGO_WIN_CERT_THUMBPRINT = 'ABCD...EF'
#   ./scripts/package-windows.ps1 0.1.0
#
# Custom RFC3161 timestamp server (defaults to DigiCert):
#   $env:DAYGO_WIN_TIMESTAMP_URL = 'http://timestamp.sectigo.com'
# ─────────────────────────────────────────────────────────────

param(
    [string]$Version = 'dev',
    [switch]$RunSmoke,
    [ValidateSet('machine', 'user')]
    [string]$InstallScope = 'machine'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# Reuse Daygo's existing Windows helpers: frontend bootstrap, tool checks and
# the lockfile-strict npm ci production path (same as scripts/build.ps1).
. (Join-Path $PSScriptRoot 'windows-common.ps1')

$AppName = 'Daygo'
$Arch = 'amd64'

# ─────────────────────────────────────────────────────────────
# Signing configuration (all optional; unset means an unsigned build)
# ─────────────────────────────────────────────────────────────

$CertFile = $env:DAYGO_WIN_CERT_FILE
$CertPassword = $env:DAYGO_WIN_CERT_PASSWORD
$CertThumbprint = $env:DAYGO_WIN_CERT_THUMBPRINT
$TimestampUrl = if ($env:DAYGO_WIN_TIMESTAMP_URL) {
    $env:DAYGO_WIN_TIMESTAMP_URL
} else {
    'http://timestamp.digicert.com'
}

# A certificate file takes precedence over an installed-cert thumbprint.
$SignMode = 'none'
if ($CertFile) {
    $SignMode = 'file'
} elseif ($CertThumbprint) {
    $SignMode = 'store'
}

# ─────────────────────────────────────────────────────────────
# Paths
# ─────────────────────────────────────────────────────────────

$BinDir = Join-Path $DaygoRootDir 'build\bin'
$Exe = Join-Path $BinDir "$AppName.exe"
$Dll = Join-Path $BinDir 'daygo_windows_native.dll'
# wails build -nsis emits "<name>-<arch>-installer.exe" into build/bin.
$Installer = Join-Path $BinDir "$AppName-$Arch-installer.exe"

$DistDir = Join-Path $DaygoRootDir 'dist'
$InfoJson = Join-Path $DaygoRootDir 'build\windows\info.json'
$InstallerProject = Join-Path $DaygoRootDir 'build\windows\installer\project.nsi'
$InstallerProjectSource = Join-Path $DaygoRootDir 'scripts\windows-installer\project.nsi'
$AcceptanceManifest = Join-Path $DistDir 'windows-package.json'

if ($Version -ne 'dev' -and $Version -notmatch '^\d+\.\d+\.\d+$') {
    throw "Version must be 'dev' or three numeric components (for example 0.1.0): $Version"
}

if ($Version -eq 'dev') {
    $DistInstaller = Join-Path $DistDir "$AppName-$Arch-installer.exe"
} else {
    $DistInstaller = Join-Path $DistDir "$AppName-$Version-$Arch-installer.exe"
}

# ─────────────────────────────────────────────────────────────
# Pretty output
# ─────────────────────────────────────────────────────────────

function Write-Step {
    param([string]$Message)
    Write-Host ''
    Write-Host "━━━ $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "  ✓ $Message" -ForegroundColor Green
}

function Write-WarnLine {
    param([string]$Message)
    Write-Host "  ! $Message" -ForegroundColor Yellow
}

function Stop-WithError {
    param([string]$Message)
    Write-Host "  ✗ $Message" -ForegroundColor Red
    throw $Message
}

Write-Host ''
Write-Host '╭──────────────────────────────────────────────╮' -ForegroundColor Cyan
Write-Host '│              Daygo Packager                  │' -ForegroundColor Cyan
Write-Host '╰──────────────────────────────────────────────╯' -ForegroundColor Cyan
Write-Host ''
Write-Host "  Version       $Version"
Write-Host "  Architecture  windows/$Arch"
Write-Host "  Install scope $InstallScope"

switch ($SignMode) {
    'file' {
        Write-Host "  Signing       certificate file ($(Split-Path -Leaf $CertFile))" -ForegroundColor Green
        if (-not $CertPassword) {
            Write-WarnLine 'DAYGO_WIN_CERT_PASSWORD is empty; signtool will prompt or fail if the .pfx is protected.'
        }
    }
    'store' {
        Write-Host "  Signing       installed certificate ($CertThumbprint)" -ForegroundColor Green
    }
    default {
        Write-Host '  Signing       none (unsigned)' -ForegroundColor Yellow
    }
}

# ─────────────────────────────────────────────────────────────
# Signing helper (never echoes the certificate password)
# ─────────────────────────────────────────────────────────────

function Invoke-DaygoSigntool {
    param([Parameter(Mandatory = $true)][string]$Path)

    $signArgs = @('sign', '/fd', 'sha256', '/tr', $TimestampUrl, '/td', 'sha256')
    if ($SignMode -eq 'file') {
        $signArgs += @('/f', $CertFile)
        if ($CertPassword) {
            # /p exposes the password to the process list; prefer thumbprint mode.
            $signArgs += @('/p', $CertPassword)
        }
    } elseif ($SignMode -eq 'store') {
        $signArgs += @('/sha1', $CertThumbprint)
    }
    $signArgs += $Path

    Invoke-DaygoNative signtool $signArgs
}

# ─────────────────────────────────────────────────────────────
# Environment
# ─────────────────────────────────────────────────────────────

Write-Step 'Checking environment'

if ($env:OS -ne 'Windows_NT') {
    Stop-WithError 'Windows packages must be built on Windows (wails build -nsis, makensis and signtool are Windows-only).'
}

Assert-DaygoTool -Name go -Hint 'Install Go from https://go.dev/dl/ or via winget (winget install GoLang.Go).'
Assert-DaygoTool -Name npm -Hint 'Install Node.js 20.19+ (or 22.12+) from https://nodejs.org/.'

if (-not (Get-Command makensis -ErrorAction SilentlyContinue)) {
    $nsisCandidates = @(
        (Join-Path $env:ProgramFiles 'NSIS\makensis.exe'),
        (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\NSIS\makensis.exe')
    )
    $makensisPath = $nsisCandidates |
        Where-Object { $_ -and (Test-Path -LiteralPath $_ -PathType Leaf) } |
        Select-Object -First 1
    if ($makensisPath) {
        $env:PATH = "$(Split-Path -Parent $makensisPath);$env:PATH"
    }
}
Assert-DaygoTool -Name makensis -Hint 'Install NSIS (winget install NSIS.NSIS) and add it to PATH; wails build -nsis invokes it.'

if ($SignMode -ne 'none') {
    Assert-DaygoTool -Name signtool -Hint 'signtool ships with the Windows SDK; run from a Developer Command Prompt or add the SDK bin\x64 to PATH.'
    if ($SignMode -eq 'file' -and -not (Test-Path -LiteralPath $CertFile -PathType Leaf)) {
        Stop-WithError "Certificate file not found: $CertFile"
    }
}

Write-Success 'Build environment ready'

# ─────────────────────────────────────────────────────────────
# Bootstrap
# ─────────────────────────────────────────────────────────────

Write-Step 'Preparing frontend'

Initialize-DaygoFrontend -DependencyMode ci -ForceBuild

Write-Success 'Frontend and Wails bindings ready'

# ─────────────────────────────────────────────────────────────
# Native capture smoke (optional)
# ─────────────────────────────────────────────────────────────

if ($RunSmoke) {
    Write-Step 'Running native capture smoke'
    Invoke-DaygoNative powershell @(
        '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File',
        (Join-Path $DaygoRootDir 'native\windows\build.ps1'), '-RunSmoke'
    )
    Write-Success 'Native smoke passed'
}

# ─────────────────────────────────────────────────────────────
# Cleanup
# ─────────────────────────────────────────────────────────────

Write-Step 'Cleaning old builds'

if (Test-Path -LiteralPath $BinDir) {
    Remove-Item -LiteralPath $BinDir -Recurse -Force
}
if (Test-Path -LiteralPath $DistDir) {
    Remove-Item -LiteralPath $DistDir -Recurse -Force
}
New-Item -ItemType Directory -Path $DistDir -Force | Out-Null

Write-Success 'Workspace cleaned'

# ─────────────────────────────────────────────────────────────
# Build + installer templates
# ─────────────────────────────────────────────────────────────

Write-Step 'Building Daygo installer'

if (-not (Test-Path -LiteralPath $InstallerProjectSource -PathType Leaf)) {
    Stop-WithError "Tracked NSIS project is missing: $InstallerProjectSource"
}
New-Item -ItemType Directory -Path (Split-Path -Parent $InstallerProject) -Force | Out-Null
Copy-Item -LiteralPath $InstallerProjectSource -Destination $InstallerProject -Force

# build/windows/info.json is a Wails template; the versioninfo resource is
# compiled from it. Pin the concrete version for a release build, then restore
# the tracked template so the working tree is left unchanged.
$InfoJsonOriginal = $null
if ($Version -ne 'dev' -and (Test-Path -LiteralPath $InfoJson -PathType Leaf)) {
    $InfoJsonOriginal = Get-Content -LiteralPath $InfoJson -Raw
    $patched = $InfoJsonOriginal -replace [regex]::Escape('{{.Info.ProductVersion}}'), $Version
    Set-Content -LiteralPath $InfoJson -Value $patched -NoNewline
}

try {
    Push-Location (Join-Path $DaygoRootDir 'cmd\daygo')
    try {
        # -nsis generates build/windows/installer/ templates on first run and
        # wraps the freshly built EXE. The windows/* preBuildHook compiles the
        # native capture DLL (see cmd/daygo/wails.json).
        Invoke-DaygoNative go @(
            'run', $DaygoWailsPackage, 'build',
            '-platform', "windows/$Arch",
            '-nsis',
            '-installscope', $InstallScope,
            '-clean'
        )
    }
    finally {
        Pop-Location
    }
}
finally {
    if ($null -ne $InfoJsonOriginal) {
        Set-Content -LiteralPath $InfoJson -Value $InfoJsonOriginal -NoNewline
    }
}

Write-Success 'Wails build completed'

# ─────────────────────────────────────────────────────────────
# Verify artifacts
# ─────────────────────────────────────────────────────────────

Write-Step 'Inspecting build output'

if (-not (Test-Path -LiteralPath $Exe -PathType Leaf)) {
    if (Test-Path -LiteralPath $BinDir) {
        Get-ChildItem -LiteralPath $BinDir | Format-Table -AutoSize | Out-String | Write-Host
    }
    Stop-WithError "Daygo executable is missing: $Exe"
}
if (-not (Test-Path -LiteralPath $Dll -PathType Leaf)) {
    Stop-WithError "Required capture helper is missing: $Dll"
}
if (-not (Test-Path -LiteralPath $Installer -PathType Leaf)) {
    Stop-WithError "NSIS installer was not generated: $Installer"
}

Write-Success 'Executable, capture DLL and installer present'

# ─────────────────────────────────────────────────────────────
# Signing
# ─────────────────────────────────────────────────────────────

if ($SignMode -ne 'none') {
    Write-Step 'Signing application artifacts'

    Invoke-DaygoSigntool -Path $Exe
    Invoke-DaygoSigntool -Path $Dll
    Invoke-DaygoNative signtool @('verify', '/pa', $Exe)
    Invoke-DaygoNative signtool @('verify', '/pa', $Dll)
    Write-Success 'Application signatures applied and verified'
} else {
    Write-Step 'Skipping signing'
    Write-WarnLine 'Unsigned build. Suitable for local testing; SmartScreen will warn on other machines.'
}

# The first Wails NSIS pass materialises wails_tools.nsh and the WebView2
# bootstrapper. Rebuild after signing so the installer contains the final EXE
# and DLL bytes. The tracked project.nsi explicitly packages the native DLL;
# the stock Wails template only packages the EXE.
Write-Step 'Repacking final application artifacts'

if (-not (Test-Path -LiteralPath $InstallerProject -PathType Leaf)) {
    Stop-WithError "Tracked NSIS project is missing: $InstallerProject"
}

$makeNsisArgs = @("-DARG_WAILS_AMD64_BINARY=$Exe")
if ($Version -ne 'dev') {
    $makeNsisArgs += "-DINFO_PRODUCTVERSION=$Version"
}
if ($InstallScope -eq 'user') {
    $makeNsisArgs += @('-DWAILS_INSTALL_SCOPE=user', '-DREQUEST_EXECUTION_LEVEL=user')
}
$makeNsisArgs += $InstallerProject
Push-Location (Split-Path -Parent $InstallerProject)
try {
    Invoke-DaygoNative makensis $makeNsisArgs
}
finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $Installer -PathType Leaf)) {
    Stop-WithError "NSIS did not regenerate the installer: $Installer"
}

if ($SignMode -ne 'none') {
    Invoke-DaygoSigntool -Path $Installer
    Invoke-DaygoNative signtool @('verify', '/pa', $Installer)
    Write-Success 'Final installer signature applied and verified'
} else {
    Write-Success 'Final unsigned installer rebuilt with application DLL'
}

# ─────────────────────────────────────────────────────────────
# Publish to dist/
# ─────────────────────────────────────────────────────────────

Write-Step 'Publishing installer'

Copy-Item -LiteralPath $Installer -Destination $DistInstaller -Force

$artifactHash = (Get-FileHash -LiteralPath $DistInstaller -Algorithm SHA256).Hash.ToLowerInvariant()
$gitCommit = (& git -C $DaygoRootDir rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) {
    Stop-WithError 'Unable to resolve the source commit for the package manifest.'
}
$manifest = [ordered]@{
    schemaVersion = 1
    product = $AppName
    version = $Version
    platform = 'windows'
    architecture = $Arch
    installScope = $InstallScope
    sourceCommit = $gitCommit
    signed = ($SignMode -ne 'none')
    artifact = [ordered]@{
        file = (Split-Path -Leaf $DistInstaller)
        sha256 = $artifactHash
        sizeBytes = (Get-Item -LiteralPath $DistInstaller).Length
    }
}
$manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $AcceptanceManifest -Encoding utf8

Write-Success "Installer copied to $DistInstaller"
Write-Success "Acceptance manifest written to $AcceptanceManifest"

# ─────────────────────────────────────────────────────────────
# Finish
# ─────────────────────────────────────────────────────────────

$sizeMb = [math]::Round((Get-Item -LiteralPath $DistInstaller).Length / 1MB, 1)

Write-Host ''
Write-Host '╭──────────────────────────────────────────────╮' -ForegroundColor Green
Write-Host '│              Package complete ✓              │' -ForegroundColor Green
Write-Host '╰──────────────────────────────────────────────╯' -ForegroundColor Green
Write-Host ''
Write-Host "  Installer  $DistInstaller"
Write-Host "  Size       $sizeMb MB"
Write-Host ''
