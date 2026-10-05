#!/usr/bin/env bash
# Builds RIFT.app (universal: Apple Silicon + Intel) and RIFT.dmg.
#
#   scripts/macos/build-app.sh
#
# Optional environment:
#   VERSION         app version (default 2.0.0)
#   SIGN_IDENTITY   "Developer ID Application: …" — otherwise ad-hoc signed
#   NOTARY_PROFILE  notarytool keychain profile — notarizes and staples the DMG
#
# macOS grants Accessibility / Screen Recording to a signed app bundle, so
# RIFT must run as RIFT.app (not a bare binary) for permissions to stick.
# Ad-hoc signatures change every build, so users re-grant after each update;
# ship with a Developer ID for a smooth experience.
set -euo pipefail

cd "$(dirname "$0")/../.."
VERSION="${VERSION:-2.0.0}"
OUT=build/macos
APP="$OUT/RIFT.app"
export MACOSX_DEPLOYMENT_TARGET=12.3
export CGO_ENABLED=1

rm -rf "$OUT"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

echo "▸ Building universal binary"
for arch in arm64 amd64; do
  GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "$OUT/rift-$arch" ./cmd/rift
done
lipo -create -output "$APP/Contents/MacOS/rift" "$OUT/rift-arm64" "$OUT/rift-amd64"
rm "$OUT/rift-arm64" "$OUT/rift-amd64"

echo "▸ Icon"
ICONSET="$OUT/RIFT.iconset"
mkdir -p "$ICONSET"
SRC=web/phone/icon-512.png
for size in 16 32 128 256 512; do
  sips -z $size $size "$SRC" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  [ $double -le 512 ] && sips -z $double $double "$SRC" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/RIFT.icns"
rm -rf "$ICONSET"

echo "▸ Info.plist"
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>RIFT</string>
  <key>CFBundleDisplayName</key><string>RIFT</string>
  <key>CFBundleIdentifier</key><string>com.harshalpatel.rift</string>
  <key>CFBundleExecutable</key><string>rift</string>
  <key>CFBundleIconFile</key><string>RIFT</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>${VERSION}</string>
  <key>CFBundleVersion</key><string>${VERSION}</string>
  <key>LSMinimumSystemVersion</key><string>12.3</string>
  <!-- Menu-bar app: no Dock icon. -->
  <key>LSUIElement</key><true/>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSHumanReadableCopyright</key><string>MIT License · Harshal Patel</string>
</dict>
</plist>
PLIST

echo "▸ Signing"
if [ -n "${SIGN_IDENTITY:-}" ]; then
  codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" "$APP"
else
  codesign --force --sign - "$APP"
fi
codesign --verify --strict "$APP"

echo "▸ DMG"
STAGE="$OUT/dmg"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname "RIFT" -srcfolder "$STAGE" -ov -format UDZO "$OUT/RIFT.dmg" >/dev/null
rm -rf "$STAGE"

if [ -n "${NOTARY_PROFILE:-}" ]; then
  echo "▸ Notarizing"
  xcrun notarytool submit "$OUT/RIFT.dmg" --keychain-profile "$NOTARY_PROFILE" --wait
  xcrun stapler staple "$OUT/RIFT.dmg"
fi

echo "✓ $APP"
echo "✓ $OUT/RIFT.dmg"
