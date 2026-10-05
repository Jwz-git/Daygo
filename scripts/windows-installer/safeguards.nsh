# Owned by Daygo, not a patch to Wails' generated helper. Exit codes: 10 means
# payload inaccessible/in use; 20 means WebView2 could not be made available;
# 30 means an update could not locate its existing installation.
!ifndef DAYGO_UPDATE_WAIT_TICKS
  !define DAYGO_UPDATE_WAIT_TICKS 300
!endif
Var DaygoUpdateWaitTicks
!ifndef DAYGO_WEBVIEW_MACHINE_KEY
  !define DAYGO_WEBVIEW_MACHINE_KEY "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!endif
!ifndef DAYGO_WEBVIEW_USER_KEY
  !define DAYGO_WEBVIEW_USER_KEY "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!endif

Function DaygoUpdateInit
  StrCpy $DaygoUpdateWaitTicks 0
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/DAYGO_UPDATE" $1
  ${If} ${Errors}
    Return
  ${EndIf}
  StrCpy $DaygoUpdateWaitTicks ${DAYGO_UPDATE_WAIT_TICKS}
  # NSIS removes /D= from $CMDLINE. Inspect the original command only to
  # detect its presence; NSIS has already parsed the full path into $INSTDIR.
  System::Call 'kernel32::GetCommandLineW() w.r0'
  ClearErrors
  ${GetOptions} $0 "/D=" $1
  ${IfNot} ${Errors}
    Return
  ${EndIf}
  !insertmacro wails.setShellContext
  SetRegView 64
  ReadRegStr $1 SHELL_CONTEXT "${UNINST_KEY}" "InstallLocation"
  ${If} $1 == ""
    # Older Wails installers stored the unquoted EXE path as DisplayIcon.
    ReadRegStr $1 SHELL_CONTEXT "${UNINST_KEY}" "DisplayIcon"
    ${GetParent} "$1" $1
  ${EndIf}
  ${If} $1 != ""
  ${AndIf} ${FileExists} "$1\${PRODUCT_EXECUTABLE}"
    StrCpy $INSTDIR $1
    Return
  ${EndIf}
  SetErrorLevel 30
  IfSilent +2
    MessageBox MB_OK|MB_ICONSTOP "$(DaygoUpdateTargetMissing)"
  Abort
FunctionEnd

!macro DaygoCheckFile PATH PREFIX
  ${If} ${FileExists} "${PATH}"
    # OPEN_EXISTING + GENERIC_WRITE, no sharing, no truncation. An executable
    # or DLL mapped by a running agent cannot be replaced. Never force-kill it.
    ${Do}
      System::Call 'kernel32::CreateFileW(w "${PATH}", i 0x40000000, i 0, p 0, i 3, i 0, p 0) p.r0'
      ${If} $0 != -1
        System::Call 'kernel32::CloseHandle(p r0)'
        ${ExitDo}
      ${EndIf}
      !if "${PREFIX}" == ""
        # WinSparkle starts setup BEFORE asking its host to quit. Share one
        # bounded wait across all payload files; never terminate the host.
        ${If} $DaygoUpdateWaitTicks > 0
          IntOp $DaygoUpdateWaitTicks $DaygoUpdateWaitTicks - 1
          Sleep 100
          ${Continue}
        ${EndIf}
      !endif
      SetErrorLevel 10
      IfSilent +2
        MessageBox MB_OK|MB_ICONSTOP "$(DaygoFilesBusy)"
      Abort
    ${Loop}
  ${EndIf}
!macroend

!macro DaygoCheckFiles PREFIX
Function ${PREFIX}DaygoCheckFiles
  !insertmacro DaygoCheckFile "$INSTDIR\${PRODUCT_EXECUTABLE}" "${PREFIX}"
  !insertmacro DaygoCheckFile "$INSTDIR\daygo_windows_native.dll" "${PREFIX}"
  !insertmacro DaygoCheckFile "$INSTDIR\WinSparkle.dll" "${PREFIX}"
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
