# How to Release RIFT 🚀

## 1. Build and Package
Double-click `release.bat`. 
This will:
1. Compile `rift.exe` (with custom icon & app mode)
2. Generate `RIFT_Setup.exe` (Windows Installer)

## 2. Publish to GitHub
1. Go to your GitHub Repo: https://github.com/HarshalPatel1972/rift
2. Click **Releases** > **Draft a new release**.
3. **Tag version**: `v1.0.0` (or increment as needed).
4. **Release title**: "RIFT v1.0.0 - Production Release".
5. **Description**:
   ```markdown
   # RIFT v1.0.0
   
   - ✨ **Professional App Mode**: Runs as a standalone window (no browser UI).
   - 🎨 **New Branding**: Cosmic RIFT icon.
   - ⚡ **Auto-Cleanup**: Clean shutdown when closing the window.
   - 🛠️ **Stability**: Removed legacy GUI dependencies.
   ```
6. **Attach binaries**: Drag and drop `RIFT_Setup.exe` and `rift.exe` into the upload box.
7. Click **Publish release**.

## 3. Updates
Since we are using a manual release workflow:
- Users simply download and run the new `RIFT_Setup.exe` to update.
- The installer automatically overwrites the old version.
