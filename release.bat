@echo off
echo === RIFT RELEASE BUILDER ===
echo.

echo 1. Building Executable...
call build.bat
if %errorlevel% neq 0 (
    echo BUILD FAILED!
    pause
    exit /b %errorlevel%
)

echo.
echo 2. Creating Installer...
"C:\Program Files (x86)\NSIS\makensis.exe" installer.nsi
if %errorlevel% neq 0 (
    echo INSTALLER BUILD FAILED!
    echo Ensure NSIS is installed at C:\Program Files (x86)\NSIS
    pause
    exit /b %errorlevel%
)

echo.
echo ===================================
echo DONE! 
echo Installer created: RIFT_Setup.exe
echo ===================================
pause
