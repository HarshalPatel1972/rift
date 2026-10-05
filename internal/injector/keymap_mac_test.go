package injector

import (
	"testing"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

func TestMacKeymapCoversEveryPhoneKey(t *testing.T) {
	// Every data-vk the phone UI can send (web/phone/index.html), except media
	// keys, which go through macMediaKey, and LWin, which the phone remaps.
	phone := []uint8{8, 9, 13, 27, 32, 33, 34, 35, 36, 37, 38, 39, 40, 46, 48, 65, 66, 67, 68, 69,
		70, 83, 84, 86, 87, 88, 89, 90, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123,
		187, 189, 190}
	// Mac-only remaps (data-mac="vk,mods" in the phone UI).
	phone = append(phone, 13, 32, 38, 56, 70, 77, 81, 82, 90, 122, 219)
	for _, vk := range phone {
		if _, ok := macKeycode[vk]; !ok {
			t.Errorf("VK %#02x has no macOS keycode", vk)
		}
	}
	for _, vk := range []uint8{0xAD, 0xAE, 0xAF, 0xB0, 0xB1, 0xB3} {
		if _, ok := macMediaKey[vk]; !ok {
			t.Errorf("media VK %#02x unmapped", vk)
		}
	}
}

func TestMacKeycodesAreUnique(t *testing.T) {
	seen := map[uint16]uint8{}
	for vk, kc := range macKeycode {
		if prev, dup := seen[kc]; dup {
			t.Errorf("keycode %d used by VK %#02x and %#02x", kc, prev, vk)
		}
		seen[kc] = vk
	}
}

func TestMacShortcutsKeepTheirMeaning(t *testing.T) {
	if got := macFlags(protocol.ModCtrl); got != cgFlagCommand {
		t.Errorf("Ctrl → %#x, want Command", got)
	}
	if got := macFlags(protocol.ModCtrl | protocol.ModShift); got != cgFlagCommand|cgFlagShift {
		t.Errorf("Ctrl+Shift → %#x", got)
	}
	if got := macFlags(protocol.ModWin); got != cgFlagControl {
		t.Errorf("Win → %#x, want Control", got)
	}
	if got := macFlags(protocol.ModAlt); got != cgFlagOption {
		t.Errorf("Alt → %#x, want Option", got)
	}
}
