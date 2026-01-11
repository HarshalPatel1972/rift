@echo off
echo Building RIFT for Windows...
if not exist "bin" mkdir bin
go build -ldflags="-s -w" -o bin/rift.exe cmd/rift/main.go
echo Build complete. Binary is in bin/rift.exe
