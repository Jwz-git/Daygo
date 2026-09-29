Unicode true

# Daygo keeps this Wails-compatible project file under source control because
# the stock v2.15 template only installs the application EXE. Daygo also needs
# daygo_windows_native.dll beside that EXE for capture, application inspection,
# system events and the notification-area adapter.
!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright" "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName" "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI2.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING

!define MUI_WELCOMEPAGE_TITLE "$(DaygoWelcomeTitle)"
!define MUI_WELCOMEPAGE_TEXT "$(DaygoWelcomeText)"
!insertmacro MUI_PAGE_WELCOME
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    !define MUI_PAGE_HEADER_SUBTEXT "$(DaygoDirectoryUser)"
  !else
    !define MUI_PAGE_HEADER_SUBTEXT "$(DaygoDirectoryMachine)"
  !endif
!else
  !define MUI_PAGE_HEADER_SUBTEXT "$(DaygoDirectoryMachine)"
!endif
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_TITLE "$(DaygoFinishTitle)"
!define MUI_FINISHPAGE_TEXT "$(DaygoFinishText)"
# A machine installer is elevated (possibly as another Windows user). Never
# launch the resident agent with that token or create its data for that user.
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    !define DAYGO_USER_INSTALL
    !define MUI_FINISHPAGE_RUN
    !define MUI_FINISHPAGE_RUN_TEXT "$(DaygoLaunch)"
    !define MUI_FINISHPAGE_RUN_FUNCTION DaygoLaunch
    !define MUI_PAGE_CUSTOMFUNCTION_SHOW DaygoFinishShow
  !endif
!endif
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(DaygoDesktop)"
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION DaygoDesktop
!insertmacro MUI_PAGE_FINISH
!define MUI_UNCONFIRMPAGE_TEXT_TOP "$(DaygoUninstallText)"
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!define MUI_FINISHPAGE_TITLE "$(DaygoUninstallTitle)"
!define MUI_FINISHPAGE_TEXT "$(DaygoUninstallFinish)"
!insertmacro MUI_UNPAGE_FINISH
!include "languages.nsh"
!include "safeguards.nsh"

!define WAILS_WIN10_REQUIRED "$(DaygoWindowsRequired)"
!define WAILS_ARCHITECTURE_NOT_SUPPORTED "$(DaygoArchitectureRequired)"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"

!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif

BrandingText "Daygo ${INFO_PRODUCTVERSION}"
ShowInstDetails hide
ShowUninstDetails hide

Function .onInit
  !insertmacro wails.checkArchitecture
FunctionEnd

Function un.onInit
  !insertmacro wails.setShellContext
  ReadRegDWORD $0 SHELL_CONTEXT "${UNINST_KEY}" "InstallerLanguage"
  ${IfNot} ${Errors}
    StrCpy $LANGUAGE $0
  ${EndIf}
FunctionEnd

Function DaygoDesktop
  !insertmacro wails.setShellContext
  ClearErrors
  CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
  ${If} ${Errors}
    MessageBox MB_OK|MB_ICONEXCLAMATION "$(DaygoShortcutFailed)"
  ${EndIf}
FunctionEnd

!ifdef DAYGO_USER_INSTALL
Function DaygoFinishShow
  System::Call 'shell32::IsUserAnAdmin() i.r0'
  ${If} $0 != 0
    # Even a user-scope installer can be started explicitly with Run as admin.
    SendMessage $mui.FinishPage.Run ${BM_SETCHECK} ${BST_UNCHECKED} 0
    ShowWindow $mui.FinishPage.Run ${SW_HIDE}
  ${EndIf}
FunctionEnd

Function DaygoLaunch
  System::Call 'shell32::IsUserAnAdmin() i.r0'
  ${If} $0 == 0
    SetOutPath $INSTDIR
    ClearErrors
    Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
    ${If} ${Errors}
      MessageBox MB_OK|MB_ICONEXCLAMATION "$(DaygoLaunchFailed)"
    ${EndIf}
  ${EndIf}
FunctionEnd
!endif

Section
  !insertmacro wails.setShellContext
  Call DaygoCheckFiles
  Call DaygoEnsureWebView

  SetDetailsPrint textonly
  DetailPrint "$(DaygoInstalling)"
  SetDetailsPrint listonly
  # Do not offer Ignore for a missing/locked required DLL; fail the install.
  SetOverwrite try
  ClearErrors
  SetOutPath $INSTDIR
  !insertmacro wails.files
  File "/oname=daygo_windows_native.dll" "..\..\bin\daygo_windows_native.dll"
  File "/oname=WinSparkle.dll" "..\..\bin\WinSparkle.dll"
  ${If} ${Errors}
    SetErrorLevel 10
    IfSilent +2
      MessageBox MB_OK|MB_ICONSTOP "$(DaygoFilesBusy)"
    Abort
  ${EndIf}

  CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

  !insertmacro wails.associateFiles
  !insertmacro wails.associateCustomProtocols
  !insertmacro wails.writeUninstaller
  WriteRegDWORD SHELL_CONTEXT "${UNINST_KEY}" "InstallerLanguage" $LANGUAGE
SectionEnd

Section "uninstall"
  !insertmacro wails.setShellContext
  Call un.DaygoCheckFiles

  # Remove only files this package owns. Never recurse into a chosen directory
  # (it may contain unrelated files), AppData, recordings or credentials.
  ClearErrors
  Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
  Delete "$INSTDIR\daygo_windows_native.dll"
  Delete "$INSTDIR\WinSparkle.dll"
  ${If} ${Errors}
    SetErrorLevel 10
    IfSilent +2
      MessageBox MB_OK|MB_ICONSTOP "$(DaygoFilesBusy)"
    Abort
  ${EndIf}

  Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
  Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

  !insertmacro wails.unassociateFiles
  !insertmacro wails.unassociateCustomProtocols
  !insertmacro wails.deleteUninstaller
  RMDir "$INSTDIR"
SectionEnd
