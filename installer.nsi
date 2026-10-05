!define APPNAME "RIFT"
!define COMPANYNAME "Harshal Patel"
!define DESCRIPTION "Phone keyboard & trackpad"
!define VERSIONMAJOR 2
!define VERSIONMINOR 0
!define VERSIONBUILD 0
!define HELPURL "http://github.com/HarshalPatel1972/rift"
!define UPDATEURL "http://github.com/HarshalPatel1972/rift/releases"
!define ABOUTURL "http://github.com/HarshalPatel1972/rift"
!define INSTALLSIZE 7233

RequestExecutionLevel admin ;Require admin rights on NT6+ (When UAC is turned on)

InstallDir "$PROGRAMFILES\${APPNAME}"

; rtf or txt file - remember if it is txt, it must be in the DOS text format (\r\n)
LicenseData "LICENSE"

; This will be in the installer/uninstaller's title bar
Name "${APPNAME}"
Icon "cmd\rift\app.ico"
OutFile "RIFT_Setup.exe"

!include LogicLib.nsh

Page license
Page directory
Page instfiles

!macro VerifyUserIsAdmin
UserInfo::GetAccountType
pop $0
${If} $0 != "admin" ;Require admin rights on NT4+
        messageBox mb_iconstop "Administrator rights required!"
        setErrorLevel 740 ;ERROR_ELEVATION_REQUIRED
        quit
${EndIf}
!macroend

function .onInit
	setShellVarContext all
	!insertmacro VerifyUserIsAdmin
functionEnd

section "install"
	# Files for the install directory - to build the installer, these file must be in the same directory as the install script (this file)
	setOutPath $INSTDIR
	
	# Stop a running copy (it lives in the tray) so the exe can be replaced
	nsExec::Exec 'taskkill /F /IM rift.exe'

	# Main Executable (web assets are embedded in it)
	file "rift.exe"

	# Remove leftovers from 1.x, which shipped the icon beside the exe
	delete "$INSTDIR\web\icon.png"
	rmDir "$INSTDIR\web"

	# Install App Icon (for Shortcuts)
	file /oname=rift.ico "cmd\rift\rift.ico"
	
	# Uninstaller - See function un.onInit and section "uninstall" for configuration
	writeUninstaller "$INSTDIR\uninstall.exe"

	# Start Menu
	createDirectory "$SMPROGRAMS\${APPNAME}"
	createShortCut "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk" "$INSTDIR\rift.exe" "" "$INSTDIR\rift.ico"

	# Desktop Shortcut
	createShortCut "$DESKTOP\${APPNAME}.lnk" "$INSTDIR\rift.exe" "" "$INSTDIR\rift.ico"

	# Registry information for add/remove programs
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "DisplayName" "${APPNAME} - ${DESCRIPTION}"
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "InstallLocation" "$\"$INSTDIR$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "DisplayIcon" "$\"$INSTDIR\rift.ico$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "Publisher" "$\"${COMPANYNAME}$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "HelpLink" "$\"${HELPURL}$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "URLUpdateInfo" "$\"${UPDATEURL}$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "URLInfoAbout" "$\"${ABOUTURL}$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "DisplayVersion" "$\"${VERSIONMAJOR}.${VERSIONMINOR}.${VERSIONBUILD}$\""
	WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "NoModify" 1
	WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "NoRepair" 1
	WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "EstimatedSize" ${INSTALLSIZE}

	# Let phones on trusted (private/domain) networks reach RIFT; public Wi-Fi stays blocked
	nsExec::Exec 'netsh advfirewall firewall delete rule name="${APPNAME}"'
	nsExec::Exec 'netsh advfirewall firewall add rule name="${APPNAME}" dir=in action=allow program="$INSTDIR\rift.exe" enable=yes profile=private,domain protocol=TCP'
sectionEnd

# Uninstaller

function un.onInit
	SetShellVarContext all
	!insertmacro VerifyUserIsAdmin
functionEnd

section "uninstall"
	nsExec::Exec 'taskkill /F /IM rift.exe'

	# Remove "Launch at startup" if it was enabled
	DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APPNAME}"

	# Remove Start Menu launcher
	delete "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk"
	# Try to remove the Start Menu folder - this will only happen if it is empty
	rmDir "$SMPROGRAMS\${APPNAME}"

	# Remove Desktop Shortcut
	delete "$DESKTOP\${APPNAME}.lnk"

	# Remove files
	delete $INSTDIR\rift.exe
	delete $INSTDIR\rift.ico

	# Always delete uninstaller as the last action
	delete $INSTDIR\uninstall.exe

	# Try to remove the install directory - this will only happen if it is empty
	rmDir $INSTDIR

	# Remove uninstaller information from the registry
	DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

	# Remove Windows Firewall Rule
	nsExec::Exec 'netsh advfirewall firewall delete rule name="${APPNAME}"'
sectionEnd
