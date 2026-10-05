# How to Release RIFT 🚀

## 1. Build and Package
Double-click `release.bat`. 
This will:
1. Compile `rift.exe` (with custom icon & app mode)
2. Generate `RIFT_Setup.exe` (Windows Installer)

## 2. Publish to GitHub
1. Go to your GitHub Repo: https://github.com/HarshalPatel1972/rift
2. Click **Releases** > **Draft a new release**.
3. **Tag version**: `v2.0.0` (or increment as needed). Keep `VERSIONMAJOR/MINOR/BUILD` in `installer.nsi` in sync.
4. **Release title**: "RIFT v2.0.0".
5. **Description**:
   ```markdown
   # RIFT v2.0.0

   - 🔒 **End-to-end encryption**: the key lives only in the QR code, and the dashboard is locked to this PC.
   - ⌨️ **Real phone typing**: autocorrect, swipe, emoji, dictation and IME all work.
   - 🖱️ **Trackpad gestures**: acceleration, right/middle-click taps, momentum scroll, tap-to-drag.
   - 🎛️ **Keys panel**: media, shortcuts, presentation remote, F-keys.
   - 💎 **Tray app**: live latency, pause, revoke, launch at startup, auto-reconnect.
   ```

   The CI workflow also builds `rift.exe` and `RIFT_Setup.exe` on every push; download them from the run's artifacts.
6. **Attach binaries**: Drag and drop `RIFT_Setup.exe` and `rift.exe` into the upload box.
7. Click **Publish release**.

## 3. Updates
Since we are using a manual release workflow:
- Users simply download and run the new `RIFT_Setup.exe` to update.
- The installer automatically overwrites the old version.
