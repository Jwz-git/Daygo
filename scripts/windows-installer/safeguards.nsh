# Owned by Daygo, not a patch to Wails' generated helper. Exit codes: 10 means
# payload inaccessible/in use; 20 means WebView2 could not be made available.
!ifndef DAYGO_WEBVIEW_MACHINE_KEY
  !define DAYGO_WEBVIEW_MACHINE_KEY "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!endif
!ifndef DAYGO_WEBVIEW_USER_KEY
  !define DAYGO_WEBVIEW_USER_KEY "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!endif

!macro DaygoCheckFile PATH
  ${If} ${FileExists} "${PATH}"
    # OPEN_EXISTING + GENERIC_WRITE, no sharing, no truncation. An executable
    # or DLL mapped by a running agent cannot be replaced. Never force-kill it.
    System::Call 'kernel32::CreateFileW(w "${PATH}", i 0x40000000, i 0, p 0, i 3, i 0, p 0) p.r0'
    ${If} $0 == -1
      SetErrorLevel 10
      IfSilent +2
        MessageBox MB_OK|MB_ICONSTOP "$(DaygoFilesBusy)"
      Abort
    ${EndIf}
    System::Call 'kernel32::CloseHandle(p r0)'
  ${EndIf}
!macroend

!macro DaygoCheckFiles PREFIX
Function ${PREFIX}DaygoCheckFiles
  !insertmacro DaygoCheckFile "$INSTDIR\${PRODUCT_EXECUTABLE}"
  !insertmacro DaygoCheckFile "$INSTDIR\daygo_windows_native.dll"
  !insertmacro DaygoCheckFile "$INSTDIR\WinSparkle.dll"
FunctionEnd
!macroend
!insertmacro DaygoCheckFiles ""
!insertmacro DaygoCheckFiles "un."

Function DaygoWebViewPresent
  StrCpy $1 0
  SetRegView 64
  ReadRegStr $0 HKLM "${DAYGO_WEBVIEW_MACHINE_KEY}" "pv"
  ${If} $0 != ""
  ${AndIf} $0 != "0.0.0.0"
    StrCpy $1 1
    Return
  ${EndIf}
  # A machine installer must not accept the elevated account's per-user
  # Runtime: it may be a different account from the person using Daygo.
  !ifdef DAYGO_USER_INSTALL
    ReadRegStr $0 HKCU "${DAYGO_WEBVIEW_USER_KEY}" "pv"
    ${If} $0 != ""
    ${AndIf} $0 != "0.0.0.0"
      StrCpy $1 1
    ${EndIf}
  !endif
FunctionEnd

Function DaygoEnsureWebView
  Call DaygoWebViewPresent
  ${If} $1 == 1
    Return
  ${EndIf}
  SetDetailsPrint textonly
  DetailPrint "$(DaygoWebViewInstalling)"
  SetDetailsPrint listonly
  InitPluginsDir
  SetOutPath "$PLUGINSDIR\webview2bootstrapper"
  File "tmp\MicrosoftEdgeWebview2Setup.exe"
  ClearErrors
  ExecWait '"$PLUGINSDIR\webview2bootstrapper\MicrosoftEdgeWebview2Setup.exe" /silent /install' $2
  ${IfNot} ${Errors}
    # Presence is the postcondition, not a successful process exit alone.
    # Another installer may have concurrently supplied the Runtime.
    Call DaygoWebViewPresent
    ${If} $1 == 1
      Return
    ${EndIf}
  ${EndIf}
  SetDetailsPrint both
  DetailPrint "$(DaygoWebViewFailed)"
  SetErrorLevel 20
  IfSilent +2
    MessageBox MB_OK|MB_ICONSTOP "$(DaygoWebViewFailed)"
  Abort
FunctionEnd
