; RIFT Installer Script
; Requires: rift.exe, app.ico

!include "MUI2.nsh"

Name "RIFT"
OutFile "RIFT_Setup.exe"
Unicode True

; Default Installation Directory
InstallDir "$PROGRAMFILES64\RIFT"

; Registry key to check for directory (so updates overwrite old one)
InstallDirRegKey HKCU "Software\RIFT" ""

; Request Admin privileges for Program Files
RequestExecutionLevel admin

; UI Interface Settings
!define MUI_ABORTWARNING
!define MUI_ICON "app.ico" 
!define MUI_UNICON "app.ico"
!define MUI_HEADERIMAGE
!define MUI_WELCOMEFINISHPAGE_BITMAP_NOSTRETCH

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; --- INSTALLATION SECTION ---
Section "RIFT (Required)" SecRift
  SetOutPath "$INSTDIR"
  
  ; Files to Install
  File "rift.exe"
  File "app.ico" 
  
  ; Shortcuts
  CreateDirectory "$SMPROGRAMS\RIFT"
  CreateShortcut "$SMPROGRAMS\RIFT\RIFT.lnk" "$INSTDIR\rift.exe" "" "$INSTDIR\app.ico"
  CreateShortcut "$DESKTOP\RIFT.lnk" "$INSTDIR\rift.exe" "" "$INSTDIR\app.ico"
  
  ; Store install path in Registry
  WriteRegStr HKCU "Software\RIFT" "" $INSTDIR
  
  ; Uninstaller
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  
  ; Add to Windows "Add/Remove Programs"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RIFT" "DisplayName" "RIFT - Air Typing Host"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RIFT" "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RIFT" "DisplayIcon" "$INSTDIR\rift.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RIFT" "Publisher" "Harshal Patel"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RIFT" "DisplayVersion" "1.0.0"
SectionEnd

; --- UNINSTALL SECTION ---
Section "Uninstall"
  Delete "$INSTDIR\rift.exe"
  Delete "$INSTDIR\app.ico"
  Delete "$INSTDIR\Uninstall.exe"
  
  Delete "$SMPROGRAMS\RIFT\RIFT.lnk"
  Delete "$DESKTOP\RIFT.lnk"
  RMDir "$SMPROGRAMS\RIFT"
  RMDir "$INSTDIR"
  
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\RIFT"
  DeleteRegKey HKCU "Software\RIFT"
SectionEnd
