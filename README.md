# R I F T
> **The Zero-Friction Air Typing Bridge.**

![License](https://img.shields.io/badge/license-MIT-000000?style=flat-square)
![Platform](https://img.shields.io/badge/platform-Windows-000000?style=flat-square)
![Go](https://img.shields.io/badge/backend-Go-000000?style=flat-square)

RIFT turns your smartphone into a secure, high-performance input device for your PC. No mobile apps to install, no bluetooth pairing, no cloud servers. Just pure local-network speed.

---

## ✨ Features

### 🔌 Zero Friction
- **No Mobile App Required**: Connects via a web app on your phone.
- **Instant Pairing**: Just scan the QR code to connect.
- **Local Network**: All data stays within your WiFi. No internet required after loading.

### 🛡️ Secure by Design
- **Token Authentication**: Every session is protected by a unique cryptographic token.
- **Sandboxed**: Runs in a secure, isolated process.
- **Ephemeral**: One-time connections that expire when the session ends.

### 💎 Obsidian UI
- **Digital Nerve**: Visual feedback system that reacts to your keystrokes.
- **App Mode**: Launches as a clean, native window on your desktop.
- **Dark Mode First**: Crafted with a premium "Obsidian & Gold" aesthetic.

---

## 🚀 Getting Started

### Installation
1. Download the latest **RIFT_Setup.exe** from the [Releases](https://github.com/HarshalPatel1972/rift/releases) page.
2. Run the installer.
3. Launch **RIFT** from your desktop or start menu.

### Usage
1. Open RIFT on your PC.
2. Scan the **QR Code** with your phone's camera.
3. Start typing on your phone—text appears instantly on your PC.

---

## 🛠️ Architecture

RIFT is built on a high-performance **Golang** backend that orchestrates a lightweight native bridge.

- **Backend**: Go 1.21+ (HTTP/WebSocket)
- **Frontend**: Vanilla JS (Zero dependencies)
- **Native**: `win32` API calls for input injection
- **UI**: Chromium App Mode (Headless wrapper)

---

## 📦 Building from Source

Requirements: `Go 1.21+`, `NSIS` (for installer).

```powershell
# Clone the repository
git clone https://github.com/HarshalPatel1972/rift.git

# Build the binary
./build.bat

# Create installer (Requires NSIS)
./release.bat
```

---

## 📄 License

RIFT is open-source software licensed under the [MIT License](LICENSE).

<p align="center">
  <br>
  Designed & Engineered by Harshal Patel
</p>
