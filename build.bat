@echo off
setlocal
echo === Building RIFT (pure Go, no C toolchain needed) ===

echo [1/4] Checking tools...
where rsrc >nul 2>nul || go install github.com/akavel/rsrc@latest

echo [2/4] Embedding manifest and icon...
pushd cmd\rift
rsrc -manifest ..\..\rift.manifest -ico rift.ico -o rsrc.syso || (popd & exit /b 1)
popd

echo [3/4] Testing...
go test ./... || exit /b 1

echo [4/4] Building executable...
set CGO_ENABLED=0
go build -trimpath -ldflags="-H windowsgui -s -w" -o rift.exe ./cmd/rift || exit /b 1

echo.
echo Done: rift.exe
endlocal
