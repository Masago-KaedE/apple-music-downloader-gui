Unicode True
RequestExecutionLevel admin

!include "MUI2.nsh"

!define PRODUCT_NAME "Apple Music Downloader GUI"
!define PRODUCT_VERSION "1.0.0"
!define PRODUCT_PUBLISHER "Apple Music Downloader Community"
!define PRODUCT_EXE "AppleMusicDownloaderGUI.exe"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\AppleMusicDownloaderGUI"

Name "${PRODUCT_NAME}"
OutFile "..\build\bin\AppleMusicDownloaderGUI-Setup.exe"
InstallDir "$PROGRAMFILES64\${PRODUCT_NAME}"
InstallDirRegKey HKLM "${UNINSTALL_KEY}" "InstallLocation"

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Section "安装" SEC_MAIN
  SetRegView 64
  SetShellVarContext all
  SetOutPath "$INSTDIR"
  File "..\build\bin\${PRODUCT_EXE}"
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\卸载.lnk" "$INSTDIR\Uninstall.exe"

  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_EXE}"
  WriteRegStr HKLM "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  SetRegView 64
  SetShellVarContext all
  Delete "$INSTDIR\${PRODUCT_EXE}"
  Delete "$INSTDIR\Uninstall.exe"
  Delete "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  Delete "$SMPROGRAMS\${PRODUCT_NAME}\卸载.lnk"
  RMDir "$SMPROGRAMS\${PRODUCT_NAME}"
  RMDir "$INSTDIR"
  DeleteRegKey HKLM "${UNINSTALL_KEY}"
  ; Intentionally preserve C:\Apple-Music-Downloader, downloaded media,
  ; WSL, Wrapper, config.yaml and per-user safe GUI settings.
SectionEnd
