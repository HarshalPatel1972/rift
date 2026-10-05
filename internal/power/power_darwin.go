//go:build darwin && cgo

package power

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

// ⌃⌘Q is the system "Lock Screen" shortcut.
static void riftLock(void) {
	for (int down = 1; down >= 0; down--) {
		CGEventRef e = CGEventCreateKeyboardEvent(NULL, 12, down); // kVK_ANSI_Q
		CGEventSetFlags(e, kCGEventFlagMaskCommand | kCGEventFlagMaskControl);
		CGEventPost(kCGHIDEventTap, e);
		CFRelease(e);
	}
}
*/
import "C"

import (
	"fmt"
	"os/exec"
)

type macPower struct{}

func New() Controller { return macPower{} }

func pmset(arg string) error {
	if out, err := exec.Command("/usr/bin/pmset", arg).CombinedOutput(); err != nil {
		return fmt.Errorf("pmset %s: %v: %s", arg, err, out)
	}
	return nil
}

func (macPower) Lock() error {
	C.riftLock()
	return nil
}

func (macPower) DisplayOff() error { return pmset("displaysleepnow") }

func (macPower) Sleep() error { return pmset("sleepnow") }
