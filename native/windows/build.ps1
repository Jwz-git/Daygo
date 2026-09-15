param(
    [switch]$RunSmoke,
    [switch]$RequirePrivacyAdapter
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RootDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Source = Join-Path $PSScriptRoot 'Sources\daygo_capture.cpp'
$ApplicationSource = Join-Path $PSScriptRoot 'Sources\daygo_application_proxy.cpp'
$NativeSource = Join-Path $PSScriptRoot 'Sources\daygo_windows_native.cpp'
$SystemSource = Join-Path $PSScriptRoot 'Sources\daygo_system.cpp'
$StatusItemSource = Join-Path $PSScriptRoot 'Sources\daygo_status_item.cpp'
$IncludeDir = Join-Path $RootDir 'native\include'
$OutDir = Join-Path $RootDir 'build\native\windows\amd64'
$Object = Join-Path $OutDir 'daygo_capture.o'
$ApplicationObject = Join-Path $OutDir 'daygo_application_proxy.o'
$SystemObject = Join-Path $OutDir 'daygo_system.o'
$StatusItemObject = Join-Path $OutDir 'daygo_status_item.o'
$Archive = Join-Path $OutDir 'libdaygo_capture.a'
$NativeDLL = Join-Path $OutDir 'daygo_windows_native.dll'
$NativeObject = Join-Path $OutDir 'daygo_windows_native.obj'
$NativeImportLibrary = Join-Path $OutDir 'daygo_windows_native.lib'
$BinDir = Join-Path $RootDir 'build\bin'

if (-not (Get-Command g++ -ErrorAction SilentlyContinue)) {
    throw 'MinGW-w64 g++ is required but was not found in PATH.'
}
if (-not (Get-Command ar -ErrorAction SilentlyContinue)) {
    throw 'MinGW-w64 ar is required but was not found in PATH.'
}

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

# Windows.Graphics.Capture's window-exclusion interface requires the 26100 SDK.
# Keep that privacy adapter optional so contributors with an older SDK can still
# run the Windows development shell and the baseline capture implementation.
$PrivacyHeader = Get-ChildItem -Path (Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\Include') `
    -Filter 'windows.ui.interop.h' -File -Recurse -ErrorAction SilentlyContinue |
    Sort-Object FullName -Descending |
    Select-Object -First 1
$InstalledNativeDLL = Join-Path $BinDir 'daygo_windows_native.dll'

if ($PrivacyHeader) {
    $VsWhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
    if (-not (Test-Path -LiteralPath $VsWhere -PathType Leaf)) {
        throw 'Visual Studio 2022 with the Desktop development with C++ workload is required.'
    }
    $VisualStudio = (& $VsWhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath).Trim()
    if (-not $VisualStudio) {
        throw 'Visual Studio 2022 C++ tools were not found.'
    }
    $VCVars = Join-Path $VisualStudio 'VC\Auxiliary\Build\vcvars64.bat'
    if (-not (Test-Path -LiteralPath $VCVars -PathType Leaf)) {
        throw 'vcvars64.bat was not found in the selected Visual Studio installation.'
    }
    $NativeCompile = '"{0}" >nul && cl.exe /nologo /std:c++20 /EHsc /O2 /MT /LD /DUNICODE /D_UNICODE /I"{1}" "{2}" /Fo:"{3}" /Fe:"{4}" d3d11.lib dxgi.lib windowsapp.lib runtimeobject.lib windowscodecs.lib ole32.lib user32.lib shell32.lib bcrypt.lib version.lib gdi32.lib onecoreuap.lib /link /IMPLIB:"{5}"' -f $VCVars, $IncludeDir, $NativeSource, $NativeObject, $NativeDLL, $NativeImportLibrary
    & $env:ComSpec /d /s /c $NativeCompile
    if ($LASTEXITCODE -ne 0) {
        throw "C++/WinRT native adapter compilation failed ($LASTEXITCODE)."
    }
    Copy-Item -LiteralPath $NativeDLL -Destination $InstalledNativeDLL -Force
} else {
    Remove-Item -LiteralPath $NativeDLL, $InstalledNativeDLL -Force -ErrorAction SilentlyContinue
    $Message = 'Windows SDK 26100 header windows.ui.interop.h was not found; building without per-application capture exclusion.'
    if ($RequirePrivacyAdapter) {
        throw $Message
    }
    Write-Warning $Message
}

& g++ -std=c++17 -O2 -Wall -Wextra -Wpedantic -DDAYGO_CAPTURE_BUILD=1 -I $IncludeDir -c $Source -o $Object
if ($LASTEXITCODE -ne 0) {
    throw "C++ compilation failed ($LASTEXITCODE)."
}

& g++ -std=c++17 -O2 -Wall -Wextra -Wpedantic -DDAYGO_APPLICATION_BUILD=1 -I $IncludeDir -c $ApplicationSource -o $ApplicationObject
if ($LASTEXITCODE -ne 0) {
    throw "C++ application ABI proxy compilation failed ($LASTEXITCODE)."
}

& g++ -std=c++17 -O2 -Wall -Wextra -Wpedantic -I $IncludeDir -c $SystemSource -o $SystemObject
if ($LASTEXITCODE -ne 0) {
    throw "C++ system event compilation failed ($LASTEXITCODE)."
}

& g++ -std=c++17 -O2 -Wall -Wextra -Wpedantic -I $IncludeDir -c $StatusItemSource -o $StatusItemObject
if ($LASTEXITCODE -ne 0) {
    throw "C++ status item compilation failed ($LASTEXITCODE)."
}

& ar rcs $Archive $Object $ApplicationObject $SystemObject $StatusItemObject
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

    $SystemSmokeSource = Join-Path $PSScriptRoot 'system_smoke.cpp'
    $SystemSmokeBinary = Join-Path $OutDir 'daygo-system-smoke.exe'
    & g++ -std=c++17 -O2 $SystemSmokeSource $Archive -I $IncludeDir `
        -lwtsapi32 -lshell32 -luser32 -static-libgcc -static-libstdc++ -o $SystemSmokeBinary
    if ($LASTEXITCODE -ne 0) {
        throw "System event smoke executable link failed ($LASTEXITCODE)."
    }
    Write-Host 'Running native system event smoke...'
    & $SystemSmokeBinary
    if ($LASTEXITCODE -ne 0) {
        throw "Native system event smoke failed ($LASTEXITCODE)."
    }
}
