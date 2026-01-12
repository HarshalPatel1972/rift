@echo off
echo [1/4] Cleaning...
go clean

echo [2/4] Embedding manifest...
rsrc -manifest rift.manifest -o rsrc.syso

echo [3/4] Building DEBUG Version (Console Visible)...
set CGO_ENABLED=1
set PATH=C:\TDM-GCC-64\bin;%PATH%
go build -o rift_debug.exe cmd\rift\main.go

echo [4/4] Launching...
echo WATCH FOR ERRORS!
.\rift_debug.exe
pause
