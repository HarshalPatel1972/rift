package injector

import (
	"unsafe"

	"github.com/lxn/win"
)

// TypeStr types a string using pure Go Windows API calls
func TypeStr(text string) {
	for _, r := range text {
		sendKeyPress(r)
	}
}

// TapKey simulates pressing a special key
func TapKey(key string) {
	var vk uint16
	switch key {
	case "backspace":
		vk = win.VK_BACK
	case "enter":
		vk = win.VK_RETURN
	case "tab":
		vk = win.VK_TAB
	case "escape":
		vk = win.VK_ESCAPE
	default:
		return
	}

	sendVirtualKey(vk)
}

// sendKeyPress sends a Unicode character using SendInput
func sendKeyPress(char rune) {
	input := win.KEYBD_INPUT{
		Type: win.INPUT_KEYBOARD,
	}
	input.Ki.WScan = uint16(char)
	input.Ki.DwFlags = win.KEYEVENTF_UNICODE

	// Key down
	win.SendInput(1, unsafe.Pointer(&input), int32(unsafe.Sizeof(input)))

	// Key up
	input.Ki.DwFlags = win.KEYEVENTF_UNICODE | win.KEYEVENTF_KEYUP
	win.SendInput(1, unsafe.Pointer(&input), int32(unsafe.Sizeof(input)))
}

// sendVirtualKey sends a virtual key code (for special keys)
func sendVirtualKey(vk uint16) {
	input := win.KEYBD_INPUT{
		Type: win.INPUT_KEYBOARD,
	}
	input.Ki.WVk = vk

	// Key down
	win.SendInput(1, unsafe.Pointer(&input), int32(unsafe.Sizeof(input)))

	// Key up
	input.Ki.DwFlags = win.KEYEVENTF_KEYUP
	win.SendInput(1, unsafe.Pointer(&input), int32(unsafe.Sizeof(input)))
}
