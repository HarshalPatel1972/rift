@echo off
echo === Building RIFT (Native Windows) ===

echo [1/4] Installing dependencies...
go get github.com/lxn/walk
go get github.com/skip2/go-qrcode
go get github.com/google/uuid
go install github.com/akavel/rsrc@latest

echo [2/4] Embedding manifest...
rsrc -manifest rift.manifest -o rsrc.syso

echo [3/4] Building executable...
set CGO_ENABLED=1
set PATH=C:\TDM-GCC-64\bin;%PATH%
go build -ldflags="-H windowsgui -s -w" -o rift.exe cmd\rift\main.go

echo [4/4] Done!
echo.
echo Binary: rift.exe
echo Run it by double-clicking or: .\rift.exe
