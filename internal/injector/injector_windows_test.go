//go:build windows

package injector

import (
	"testing"
	"unsafe"
)

// SendInput rejects every event if cbSize doesn't match the C INPUT struct.
func TestInputMatchesWin32Layout(t *testing.T) {
	want := map[uintptr]uintptr{8: 40, 4: 28}[unsafe.Sizeof(uintptr(0))]
	if got := unsafe.Sizeof(input{}); got != want {
		t.Fatalf("sizeof(INPUT) = %d, want %d", got, want)
	}
	if unsafe.Sizeof(keybdInput{}) > unsafe.Sizeof(mouseInput{}) {
		t.Fatal("KEYBDINPUT must fit in the union")
	}
}
