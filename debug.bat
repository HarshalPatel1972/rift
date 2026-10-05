@echo off
echo === RIFT debug build (console visible, logs to stderr) ===
set CGO_ENABLED=0
go build -o rift_debug.exe ./cmd/rift || (pause & exit /b 1)
echo Logs are also written to %%APPDATA%%\RIFT\rift.log
.\rift_debug.exe
pause
