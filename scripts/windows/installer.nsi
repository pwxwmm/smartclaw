; SmartClaw Windows Installer (NSIS)
; Build: makensis installer.nsi
; Prerequisites: Place smartclaw_windows_amd64.exe in the same directory as smartclaw.exe

!define PRODUCT_NAME "SmartClaw"
!define PRODUCT_VERSION "1.0.0"
!define PRODUCT_PUBLISHER "SmartClaw"
!define PRODUCT_WEB_SITE "https://github.com/instructkr/smartclaw"
!define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\smartclaw.exe"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"
!define PRODUCT_UNINST_ROOT_KEY "HKLM"

!include "MUI.nsh"

!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "../LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_INSTFILES

Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "SmartClaw-${PRODUCT_VERSION}-setup.exe"
InstallDir "$PROGRAMFILES\SmartClaw"
InstallDirRegKey HKLM "${PRODUCT_DIR_REGKEY}" ""
ShowInstDetails show
ShowUnInstDetails show

Section "MainSection" SEC01
  SetOutPath "$INSTDIR"
  SetOverwrite on
  File "smartclaw.exe"
  
  ; Add to PATH
  Push "$INSTDIR"
  Call AddToPath
SectionEnd

Section -AdditionalIcons
  CreateDirectory "$SMPROGRAMS\SmartClaw"
  CreateShortCut "$SMPROGRAMS\SmartClaw\SmartClaw.lnk" "$INSTDIR\smartclaw.exe"
  CreateShortCut "$SMPROGRAMS\SmartClaw\Uninstall.lnk" "$INSTDIR\uninst.exe"
SectionEnd

Section -Post
  WriteUninstaller "$INSTDIR\uninst.exe"
  WriteRegStr HKLM "${PRODUCT_DIR_REGKEY}" "" "$INSTDIR\smartclaw.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayName" "$(^Name)"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninst.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
SectionEnd

Section Uninstall
  ; Remove from PATH
  Push "$INSTDIR"
  Call un.RemoveFromPath
  
  Delete "$INSTDIR\smartclaw.exe"
  Delete "$INSTDIR\uninst.exe"
  Delete "$SMPROGRAMS\SmartClaw\SmartClaw.lnk"
  Delete "$SMPROGRAMS\SmartClaw\Uninstall.lnk"
  RMDir "$SMPROGRAMS\SmartClaw"
  RMDir "$INSTDIR"
  
  DeleteRegKey ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKLM "${PRODUCT_DIR_REGKEY}"
SectionEnd

; --- PATH manipulation functions ---

!include "WinMessages.nsh"

Function AddToPath
  Exch $0
  Push $1
  Push $2
  Push $3
  
  ReadRegStr $1 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path"
  StrCpy $2 $1 1 -1
  StrCmp $2 ";" 0 +2
    StrCpy $1 $1 -1
  
  StrCmp $1 "" AddToPath_DoAdd AddToPath_CheckExists
AddToPath_CheckExists:
  StrCpy $2 ";$1;"
  StrCpy $3 ";$0;"
  StrCmp $2 $3 AddToPath_Done
  
AddToPath_DoAdd:
  StrCpy $2 "$1;$0"
  WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path" $2
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment"
  
AddToPath_Done:
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd

Function un.RemoveFromPath
  Exch $0
  Push $1
  Push $2
  Push $3
  Push $4
  
  ReadRegStr $1 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path"
  StrCpy $2 $1 1 -1
  StrCmp $2 ";" 0 +2
    StrCpy $1 $1 -1
  
  StrCpy $2 ";$1;"
  StrCpy $3 ";$0;"
  
  StrCpy $4 $2 "" $3
  StrCmp $4 "" un.RemoveFromPath_Done
  
  StrCpy $5 $2 0 -$3
  StrCpy $4 "$5$4"
  StrCpy $4 $4 "" 1
  StrLen $5 $4
  IntOp $5 $5 - 1
  StrCpy $4 $4 $5 0
  
  WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path" $4
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment"
  
un.RemoveFromPath_Done:
  Pop $4
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd
