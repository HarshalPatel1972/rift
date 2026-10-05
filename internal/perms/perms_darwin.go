//go:build darwin && cgo

package perms

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics -framework Foundation
#import <Foundation/Foundation.h>
#import <ApplicationServices/ApplicationServices.h>

static int riftInputOK(void) { return AXIsProcessTrusted() ? 1 : 0; }
static int riftScreenOK(void) { return CGPreflightScreenCaptureAccess() ? 1 : 0; }

// Shows the system prompt that adds RIFT to the Accessibility list.
static void riftAskInput(void) {
	NSDictionary *opts = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @YES};
	AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts);
}

// Adds RIFT to the Screen Recording list (prompts once per app version).
static void riftAskScreen(void) { CGRequestScreenCaptureAccess(); }
*/
import "C"

import "os/exec"

func Check() Status {
	return Status{Required: true, Input: C.riftInputOK() == 1, Screen: C.riftScreenOK() == 1}
}

// Request triggers the system prompt and opens the matching pane of
// System Settings, where the user flips RIFT's switch.
func Request(kind string) error {
	pane := "Privacy_Accessibility"
	switch kind {
	case Input:
		C.riftAskInput()
	case Screen:
		C.riftAskScreen()
		pane = "Privacy_ScreenCapture"
	default:
		return nil
	}
	return exec.Command("/usr/bin/open", "x-apple.systempreferences:com.apple.preference.security?"+pane).Start()
}
