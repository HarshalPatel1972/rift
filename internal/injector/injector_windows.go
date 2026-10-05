//go:build windows

package injector

import (
	"fmt"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procSendInput     = user32.NewProc("SendInput")
	procMapVirtualKey = user32.NewProc("MapVirtualKeyW")
	procSetCursorPos  = user32.NewProc("SetCursorPos")
	procSetDpiCtx     = user32.NewProc("SetProcessDpiAwarenessContext")
)

// Per-monitor DPI awareness makes pointer, capture and SetCursorPos all use
// physical pixels, so Screen Peek taps land exactly where they should on
// mixed-DPI setups. It must happen before any window is created.
func init() {
	const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT(-4)
	if procSetDpiCtx.Find() == nil {
		procSetDpiCtx.Call(perMonitorAwareV2)
	}
}

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002
	keyeventfUnicode     = 0x0004
	keyeventfScanCode    = 0x0008

	mouseeventfMove       = 0x0001
	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040
	mouseeventfWheel      = 0x0800
	mouseeventfHWheel     = 0x1000

	vkBack    = 0x08
	vkTab     = 0x09
	vkReturn  = 0x0D
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkLWin    = 0x5B

	mapvkVkToVsc = 0

	// SendInput is atomic per call, but very large batches can starve the
	// target's message queue; chunking keeps big pastes reliable.
	maxBatch = 512
)

// mouseInput is MOUSEINPUT, the largest member of INPUT's union, so the Go
// struct below has the same size and alignment as INPUT on both 386 and amd64.
type mouseInput struct {
	dx, dy      int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type input struct {
	typ uint32
	mi  mouseInput
}

func keyInput(vk, scan uint16, flags uint32) input {
	in := input{typ: inputKeyboard}
	ki := (*keybdInput)(unsafe.Pointer(&in.mi))
	ki.wVk, ki.wScan, ki.dwFlags = vk, scan, flags
	return in
}

func mouse(flags uint32, dx, dy int32, data uint32) input {
	return input{typ: inputMouse, mi: mouseInput{dx: dx, dy: dy, mouseData: data, dwFlags: flags}}
}

// Keys that live on the extended (E0-prefixed) part of the keyboard. Without
// the flag, apps that read scan codes see numpad keys instead of arrows etc.
var extended = map[uint16]bool{
	0x21: true, 0x22: true, 0x23: true, 0x24: true, // PgUp PgDn End Home
	0x25: true, 0x26: true, 0x27: true, 0x28: true, // arrows
	0x2D: true, 0x2E: true, // Insert Delete
	0x5B: true, 0x5C: true, 0x5D: true, // LWin RWin Apps
	0x6F: true, 0x90: true, // Divide NumLock
	0xA3: true, 0xA5: true, // RControl RMenu
	0xAD: true, 0xAE: true, 0xAF: true, // volume
	0xB0: true, 0xB1: true, 0xB2: true, 0xB3: true, // media
}

func vkInput(vk uint16, up bool) input {
	scan, _, _ := procMapVirtualKey.Call(uintptr(vk), mapvkVkToVsc)
	var flags uint32
	if extended[vk] {
		flags |= keyeventfExtendedKey
	}
	if up {
		flags |= keyeventfKeyUp
	}
	return keyInput(vk, uint16(scan), flags)
}

func tap(buf []input, vk uint16) []input {
	return append(buf, vkInput(vk, false), vkInput(vk, true))
}

type windowsInjector struct{}

// New returns the SendInput-backed injector.
func New() Injector { return windowsInjector{} }

func send(inputs []input) error {
	for len(inputs) > 0 {
		n := min(len(inputs), maxBatch)
		r, _, err := procSendInput.Call(
			uintptr(n),
			uintptr(unsafe.Pointer(&inputs[0])),
			unsafe.Sizeof(inputs[0]),
		)
		if int(r) != n {
			// Usually UIPI: the foreground window belongs to an elevated process.
			return fmt.Errorf("SendInput injected %d/%d events: %w", r, n, err)
		}
		inputs = inputs[n:]
	}
	return nil
}

func (windowsInjector) Edit(del int, insert string) error {
	buf := make([]input, 0, 2*del+4*len(insert))
	for range del {
		buf = tap(buf, vkBack)
	}
	for _, r := range insert {
		switch r {
		case '\r':
			continue
		case '\n':
			buf = tap(buf, vkReturn)
		case '\t':
			buf = tap(buf, vkTab)
		default:
			// KEYEVENTF_UNICODE takes one UTF-16 unit per event, so characters
			// outside the BMP (emoji, etc.) must go as a surrogate pair.
			units := []uint16{uint16(r)}
			if r1, r2 := utf16.EncodeRune(r); r1 != 0xFFFD {
				units = []uint16{uint16(r1), uint16(r2)}
			}
			for _, u := range units {
				buf = append(buf,
					keyInput(0, u, keyeventfUnicode),
					keyInput(0, u, keyeventfUnicode|keyeventfKeyUp))
			}
		}
	}
	return send(buf)
}

var modKeys = []struct {
	bit byte
	vk  uint16
}{
	{protocol.ModCtrl, vkControl},
	{protocol.ModShift, vkShift},
	{protocol.ModAlt, vkMenu},
	{protocol.ModWin, vkLWin},
}

func (windowsInjector) Key(vk uint8, mods uint8) error {
	if vk == 0 {
		return nil
	}
	var buf []input
	for _, m := range modKeys {
		if mods&m.bit != 0 {
			buf = append(buf, vkInput(m.vk, false))
		}
	}
	buf = tap(buf, uint16(vk))
	for i := len(modKeys) - 1; i >= 0; i-- {
		if mods&modKeys[i].bit != 0 {
			buf = append(buf, vkInput(modKeys[i].vk, true))
		}
	}
	return send(buf)
}

func (windowsInjector) Move(dx, dy int) error {
	return send([]input{mouse(mouseeventfMove, int32(dx), int32(dy), 0)})
}

func (windowsInjector) MoveTo(x, y int) error {
	if r, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y)); r == 0 {
		return fmt.Errorf("SetCursorPos: %w", err)
	}
	return nil
}

func (windowsInjector) Button(button, action uint8) error {
	var down, up uint32
	switch button {
	case protocol.ButtonLeft:
		down, up = mouseeventfLeftDown, mouseeventfLeftUp
	case protocol.ButtonRight:
		down, up = mouseeventfRightDown, mouseeventfRightUp
	case protocol.ButtonMiddle:
		down, up = mouseeventfMiddleDown, mouseeventfMiddleUp
	default:
		return nil
	}
	switch action {
	case protocol.ActionDown:
		return send([]input{mouse(down, 0, 0, 0)})
	case protocol.ActionUp:
		return send([]input{mouse(up, 0, 0, 0)})
	case protocol.ActionClick:
		return send([]input{mouse(down, 0, 0, 0), mouse(up, 0, 0, 0)})
	}
	return nil
}

func (windowsInjector) Scroll(dx, dy int) error {
	var buf []input
	if dy != 0 {
		buf = append(buf, mouse(mouseeventfWheel, 0, 0, uint32(int32(dy))))
	}
	if dx != 0 {
		buf = append(buf, mouse(mouseeventfHWheel, 0, 0, uint32(int32(dx))))
	}
	if len(buf) == 0 {
		return nil
	}
	return send(buf)
}
