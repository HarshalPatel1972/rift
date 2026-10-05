package injector

import "github.com/HarshalPatel1972/rift/internal/protocol"

// The wire protocol speaks Windows virtual-key codes. The macOS injector
// translates them with these tables; they live in a platform-neutral file so
// they're unit-tested on every OS.

// macKeycode maps a Windows VK to a macOS virtual keycode (kVK_*).
var macKeycode = map[uint8]uint16{
	0x08: 51,             // Backspace → kVK_Delete
	0x09: 48,             // Tab
	0x0D: 36,             // Return
	0x1B: 53,             // Escape
	0x20: 49,             // Space
	0x21: 116, 0x22: 121, // Page Up / Down
	0x23: 119, 0x24: 115, // End / Home
	0x25: 123, 0x26: 126, 0x27: 124, 0x28: 125, // ← ↑ → ↓
	0x2D: 114, // Insert → Help
	0x2E: 117, // Delete → Forward Delete

	'0': 29, '1': 18, '2': 19, '3': 20, '4': 21, '5': 23, '6': 22, '7': 26, '8': 28, '9': 25,
	'A': 0, 'B': 11, 'C': 8, 'D': 2, 'E': 14, 'F': 3, 'G': 5, 'H': 4, 'I': 34, 'J': 38,
	'K': 40, 'L': 37, 'M': 46, 'N': 45, 'O': 31, 'P': 35, 'Q': 12, 'R': 15, 'S': 1,
	'T': 17, 'U': 32, 'V': 9, 'W': 13, 'X': 7, 'Y': 16, 'Z': 6,

	0x70: 122, 0x71: 120, 0x72: 99, 0x73: 118, 0x74: 96, 0x75: 97, // F1–F6
	0x76: 98, 0x77: 100, 0x78: 101, 0x79: 109, 0x7A: 103, 0x7B: 111, // F7–F12

	0xBA: 41, // ;
	0xBB: 24, // =
	0xBC: 43, // ,
	0xBD: 27, // -
	0xBE: 47, // .
	0xBF: 44, // /
	0xC0: 50, // `
	0xDB: 33, // [
	0xDC: 42, // \
	0xDD: 30, // ]
	0xDE: 39, // '
}

// macMediaKey maps Windows media VKs to NX_KEYTYPE_* codes, which macOS
// delivers as "system-defined" events rather than ordinary key presses.
var macMediaKey = map[uint8]int{
	0xAD: 7,  // mute → NX_KEYTYPE_MUTE
	0xAE: 1,  // volume down → NX_KEYTYPE_SOUND_DOWN
	0xAF: 0,  // volume up → NX_KEYTYPE_SOUND_UP
	0xB0: 17, // next → NX_KEYTYPE_NEXT
	0xB1: 18, // previous → NX_KEYTYPE_PREVIOUS
	0xB3: 16, // play/pause → NX_KEYTYPE_PLAY
}

// CGEventFlags masks.
const (
	cgFlagShift   = 0x00020000
	cgFlagControl = 0x00040000
	cgFlagOption  = 0x00080000
	cgFlagCommand = 0x00100000
)

// macFlags maps protocol modifiers to macOS ones so shortcuts keep their
// meaning: Ctrl+C on the phone is ⌘C on a Mac. The phone labels the chips
// ⌘ ⌥ ⇧ ⌃ when the host says it's a Mac, so "Win" becomes ⌃ (Control).
func macFlags(mods uint8) uint64 {
	var f uint64
	if mods&protocol.ModCtrl != 0 {
		f |= cgFlagCommand
	}
	if mods&protocol.ModShift != 0 {
		f |= cgFlagShift
	}
	if mods&protocol.ModAlt != 0 {
		f |= cgFlagOption
	}
	if mods&protocol.ModWin != 0 {
		f |= cgFlagControl
	}
	return f
}
