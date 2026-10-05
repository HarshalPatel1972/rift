<div align="center">

# rift

### Your desk can wait.

Type, click, scroll and peek at your PC from the couch or bed, using the phone already in your hand.<br>
No phone app. No account. No cloud. End-to-end encrypted.

![License](https://img.shields.io/badge/license-MIT-8b6cff?style=flat-square)
![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS-4cc9f0?style=flat-square)
![Go](https://img.shields.io/badge/backend-Go-2ee6a8?style=flat-square)
![E2E](https://img.shields.io/badge/encryption-end--to--end-ff6b6b?style=flat-square)

</div>

---

## Why RIFT exists

Sitting at a desk all day is exhausting. RIFT lets you keep working, browsing and watching **while lying back**: your phone becomes your PC's keyboard, trackpad, screen and remote. Everything is designed for one thumb, a dim room and a screen that's too far away to read.

## ✨ What you can do

| | |
|---|---|
| ⌨️ **Type** | Your phone's keyboard, on your PC: autocorrect, swipe, emoji, every language and **voice dictation**. A **live preview** follows your text caret so you can see where your words land. Sticky Ctrl/Alt/Shift/Win and a key row with arrows. |
| 🖐️ **Touch** | A laptop-grade trackpad: acceleration, tap to click, two- and three-finger taps, two-finger scroll with momentum, double-tap-and-hold to drag, a **thumb scroll strip**, and hold-able buttons. |
| 👀 **Peek** | Too far to read the screen? See it on your phone: the whole screen, or zoomed around your **pointer** or **text caret**. **Tap on it to click there**, hold to right-click, drag to point. |
| 🌙 **Chill** | Big media controls, seek and fullscreen, volume, and a **bedtime card**: sleep timer, screen off, lock and sleep now. |
| ✨ **Keys** | Everyday shortcuts, browser and Windows shortcuts, a presentation remote and F1–F12. |

Plus: a story-driven first run, day and night themes, a left-handed mode, haptics, auto-reconnect, and add-to-home-screen support.

## 🛡️ Private by design

- **End-to-end encrypted** (NaCl secretbox, XSalsa20-Poly1305) with a separate key for each direction. The key lives only in the QR code's URL *fragment*, which browsers never send over the network.
- **Replay-proof**: every connection answers a fresh challenge, and every message carries an increasing sequence number.
- **Peek is visible and switchable**: the PC shows *"Your phone is viewing this screen"* while Peek is on, with a one-click off switch (also in the tray tooltip).
- **The dashboard is loopback-only**, with a per-launch admin key and DNS-rebinding protection. The firewall rule covers **private networks only**.
- One phone at a time; **New pairing code** revokes every phone paired before.

## 🚀 Getting started

1. Download from [Releases](https://github.com/HarshalPatel1972/rift/releases): **RIFT_Setup.exe** (Windows) or **RIFT.dmg** (macOS 12.3+, drag RIFT to Applications).
2. Open RIFT. Scan the QR code with your phone's camera (same Wi-Fi).
3. Lean back. 🛋️

> **macOS:** the first time, RIFT asks for two switches in System Settings → Privacy & Security: **Accessibility** (to type and click) and **Screen Recording** (only for Peek, which needs macOS 14+). The RIFT window walks you through both and updates live. RIFT lives in the menu bar. On a Mac, your phone shows ⌘ ⌥ ⌃ and Mac shortcuts (Spotlight, Mission Control…).

> **Windows admin apps:** Windows blocks input to apps running as administrator. RIFT warns you when this happens; run RIFT as administrator to control them.

## 🛠️ Architecture

```
 Phone (browser, ES modules)              PC (rift.exe, pure Go)
┌───────────────────────┐  WebSocket/LAN  ┌────────────────────────────────┐
│ typing differ         │  E2E-encrypted  │ internal/server   :8080        │
│ gesture engine        │ ──────────────► │  handshake · sessions · ping   │
│ peek viewer (acks)    │ ◄────────────── │  peek stream (flow-controlled) │
│ tweetnacl             │  JPEG frames    │  sleep timer · actions         │
└───────────────────────┘                 │ internal/injector  SendInput   │
                                          │ internal/screen    GDI capture │
 Dashboard (Edge app window)              │ internal/power     lock/sleep  │
┌───────────────────────┐  loopback only  │ cmd/rift  127.0.0.1:8081       │
│ web/desktop           │ ◄─────────────► │  QR · SSE status · tray        │
└───────────────────────┘  admin key+SSE  └────────────────────────────────┘
```

The wire protocol is documented in [`internal/protocol/protocol.go`](internal/protocol/protocol.go).

## 📦 Building from source

Requirements: **Go 1.26+**.

**Windows** (pure Go, no C toolchain; [NSIS](https://nsis.sourceforge.io/) only for the installer):
```powershell
./build.bat      # tests + rift.exe
./release.bat    # + RIFT_Setup.exe
```

**macOS** (needs Xcode Command Line Tools, since the host uses cgo):
```bash
scripts/macos/build-app.sh                     # universal RIFT.app + RIFT.dmg, ad-hoc signed
SIGN_IDENTITY="Developer ID Application: …" NOTARY_PROFILE=rift scripts/macos/build-app.sh # signed + notarized, for distribution
```
Permissions are tied to the app's signature, so ship with a Developer ID; ad-hoc builds need permissions re-granted after every rebuild.

**Develop the phone UI without touching your PC:**
```powershell
go run ./cmd/riftdev
```
This serves the phone app against a *pretend* PC: input is only logged, and Peek shows a synthetic desktop.

**Tests:** `go test ./...` covers the crypto (against vectors from the phone's own library), the handshake, replay and tamper rejection, key rotation, pausing, takeover by a second phone, Peek flow control, tap-to-point mapping, power actions and the sleep timer, plus real GDI capture and the Win32 struct layouts.

**Landing page:** `site/` is a static page, deployed to GitHub Pages by `.github/workflows/pages.yml`.

## 🗺️ Roadmap

- 🌍 Translations and right-to-left layouts
- 🔒 HTTPS on the LAN, which unlocks wake-lock, an installable app and the gyroscope "air mouse"

## 📄 License

MIT. Bundles [TweetNaCl.js](https://github.com/dchest/tweetnacl-js) (public domain) and [Nunito](https://github.com/googlefonts/nunito) (SIL OFL 1.1).

<p align="center"><br>Made with 💜 by Harshal Patel</p>
