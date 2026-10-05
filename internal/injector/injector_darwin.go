//go:build darwin && cgo

package injector

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework ApplicationServices -framework AppKit -framework CoreGraphics

#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>

static CGEventSourceRef riftSource(void) {
	static CGEventSourceRef src;
	if (!src) src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
	return src;
}

static int riftTrusted(void) { return AXIsProcessTrusted() ? 1 : 0; }

static CGPoint riftCursor(void) {
	CGEventRef e = CGEventCreate(NULL);
	CGPoint p = CGEventGetLocation(e);
	CFRelease(e);
	return p;
}

// Union of all active displays, in global points (origin top-left).
static CGRect riftDesktop(void) {
	CGDirectDisplayID ids[16];
	uint32_t n = 0;
	CGGetActiveDisplayList(16, ids, &n);
	CGRect r = CGRectNull;
	for (uint32_t i = 0; i < n; i++) r = CGRectUnion(r, CGDisplayBounds(ids[i]));
	return r;
}

static void riftKey(CGKeyCode kc, CGEventFlags flags, bool down) {
	CGEventRef e = CGEventCreateKeyboardEvent(riftSource(), kc, down);
	CGEventSetFlags(e, flags);
	CGEventPost(kCGHIDEventTap, e);
	CFRelease(e);
}

// Types UTF-16 text; macOS caps each event's string at 20 units.
static void riftUnicode(const UniChar *s, int n) {
	int i = 0;
	while (i < n) {
		int len = n - i < 20 ? n - i : 20;
		// Don't split a surrogate pair across events.
		if (len == 20 && CFStringIsSurrogateHighCharacter(s[i + len - 1])) len--;
		for (int down = 1; down >= 0; down--) {
			CGEventRef e = CGEventCreateKeyboardEvent(riftSource(), 0, down);
			CGEventKeyboardSetUnicodeString(e, len, s + i);
			CGEventPost(kCGHIDEventTap, e);
			CFRelease(e);
		}
		i += len;
	}
}

// Media keys are "system-defined" NSEvents, not ordinary key presses.
static void riftMedia(int key) {
	for (int down = 1; down >= 0; down--) {
		NSEvent *ev = [NSEvent otherEventWithType:NSEventTypeSystemDefined
		                                 location:NSZeroPoint
		                            modifierFlags:(down ? 0xa00 : 0xb00)
		                                timestamp:0
		                             windowNumber:0
		                                  context:nil
		                                  subtype:8
		                                    data1:((key << 16) | ((down ? 0xa : 0xb) << 8))
		                                    data2:-1];
		CGEventPost(kCGHIDEventTap, [ev CGEvent]);
	}
}

static void riftMouse(CGEventType type, CGPoint p, CGMouseButton b, int clicks) {
	CGEventRef e = CGEventCreateMouseEvent(riftSource(), type, p, b);
	if (clicks > 0) CGEventSetIntegerValueField(e, kCGMouseEventClickState, clicks);
	CGEventPost(kCGHIDEventTap, e);
	CFRelease(e);
}

static void riftScroll(int dx, int dy) {
	CGEventRef e = CGEventCreateScrollWheelEvent2(riftSource(), kCGScrollEventUnitPixel, 2, dy, dx, 0);
	CGEventPost(kCGHIDEventTap, e);
	CFRelease(e);
}

static double riftDoubleClickInterval(void) { return [NSEvent doubleClickInterval]; }
*/
import "C"

import (
	"errors"
	"math"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

// ErrNoAccessibility means macOS hasn't granted RIFT Accessibility access,
// without which posted events are silently dropped.
var ErrNoAccessibility = errors.New("injector: RIFT needs Accessibility permission")

const (
	kcDelete = 51
	kcReturn = 36
	kcTab    = 48
	// Wheel units (120 = one notch) → pixels. ~48 px per notch feels native.
	pixelsPerWheel = 0.4
)

type macInjector struct {
	mu        sync.Mutex
	held      [3]bool // buttons currently down
	lastClick struct {
		at     time.Time
		button uint8
		x, y   float64
		count  int
	}
	trustedAt time.Time
	trusted   bool
}

// New returns the CoreGraphics-backed injector.
func New() Injector { return &macInjector{} }

func (m *macInjector) check() error {
	if time.Since(m.trustedAt) > time.Second {
		m.trusted = C.riftTrusted() == 1
		m.trustedAt = time.Now()
	}
	if !m.trusted {
		return ErrNoAccessibility
	}
	return nil
}

func (m *macInjector) Edit(del int, insert string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(); err != nil {
		return err
	}
	for range del {
		C.riftKey(kcDelete, 0, true)
		C.riftKey(kcDelete, 0, false)
	}
	var run []rune
	flush := func() {
		if len(run) == 0 {
			return
		}
		u := utf16.Encode(run)
		C.riftUnicode((*C.UniChar)(&u[0]), C.int(len(u)))
		run = run[:0]
	}
	for _, r := range insert {
		switch r {
		case '\r':
		case '\n', '\t':
			flush()
			kc := C.CGKeyCode(kcReturn)
			if r == '\t' {
				kc = kcTab
			}
			C.riftKey(kc, 0, true)
			C.riftKey(kc, 0, false)
		default:
			run = append(run, r)
		}
	}
	flush()
	return nil
}

func (m *macInjector) Key(vk uint8, mods uint8) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(); err != nil {
		return err
	}
	if media, ok := macMediaKey[vk]; ok {
		C.riftMedia(C.int(media))
		return nil
	}
	kc, ok := macKeycode[vk]
	if !ok {
		return nil
	}
	flags := C.CGEventFlags(macFlags(mods))
	C.riftKey(C.CGKeyCode(kc), flags, true)
	C.riftKey(C.CGKeyCode(kc), flags, false)
	return nil
}

func (m *macInjector) moveTo(x, y float64) {
	d := C.riftDesktop()
	x = math.Min(math.Max(x, float64(d.origin.x)), float64(d.origin.x+d.size.width)-1)
	y = math.Min(math.Max(y, float64(d.origin.y)), float64(d.origin.y+d.size.height)-1)
	typ, btn := C.CGEventType(C.kCGEventMouseMoved), C.CGMouseButton(C.kCGMouseButtonLeft)
	switch {
	case m.held[protocol.ButtonLeft]:
		typ = C.CGEventType(C.kCGEventLeftMouseDragged)
	case m.held[protocol.ButtonRight]:
		typ, btn = C.CGEventType(C.kCGEventRightMouseDragged), C.CGMouseButton(C.kCGMouseButtonRight)
	case m.held[protocol.ButtonMiddle]:
		typ, btn = C.CGEventType(C.kCGEventOtherMouseDragged), C.CGMouseButton(C.kCGMouseButtonCenter)
	}
	C.riftMouse(typ, C.CGPoint{x: C.CGFloat(x), y: C.CGFloat(y)}, btn, 0)
}

func (m *macInjector) Move(dx, dy int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(); err != nil {
		return err
	}
	p := C.riftCursor()
	m.moveTo(float64(p.x)+float64(dx), float64(p.y)+float64(dy))
	return nil
}

// MoveTo takes global display coordinates in points, the same space Screen
// Peek reports its source rectangle in.
func (m *macInjector) MoveTo(x, y int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(); err != nil {
		return err
	}
	m.moveTo(float64(x), float64(y))
	return nil
}

var macButtons = [3]struct {
	down, up C.CGEventType
	button   C.CGMouseButton
}{
	{C.CGEventType(C.kCGEventLeftMouseDown), C.CGEventType(C.kCGEventLeftMouseUp), C.CGMouseButton(C.kCGMouseButtonLeft)},
	{C.CGEventType(C.kCGEventRightMouseDown), C.CGEventType(C.kCGEventRightMouseUp), C.CGMouseButton(C.kCGMouseButtonRight)},
	{C.CGEventType(C.kCGEventOtherMouseDown), C.CGEventType(C.kCGEventOtherMouseUp), C.CGMouseButton(C.kCGMouseButtonCenter)},
}

// clickCount tracks consecutive clicks: macOS only treats a click as a
// double-click if the event itself says so.
func (m *macInjector) clickCount(b uint8, p C.CGPoint) int {
	lc := &m.lastClick
	x, y := float64(p.x), float64(p.y)
	interval := time.Duration(float64(C.riftDoubleClickInterval()) * float64(time.Second))
	if lc.button == b && time.Since(lc.at) < interval && math.Hypot(x-lc.x, y-lc.y) < 6 {
		lc.count++
	} else {
		lc.count = 1
	}
	lc.at, lc.button, lc.x, lc.y = time.Now(), b, x, y
	return lc.count
}

func (m *macInjector) Button(button, action uint8) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(); err != nil {
		return err
	}
	if int(button) >= len(macButtons) {
		return nil
	}
	ev := macButtons[button]
	p := C.riftCursor()
	switch action {
	case protocol.ActionDown:
		n := m.clickCount(button, p)
		m.held[button] = true
		C.riftMouse(ev.down, p, ev.button, C.int(n))
	case protocol.ActionUp:
		m.held[button] = false
		C.riftMouse(ev.up, p, ev.button, C.int(max(m.lastClick.count, 1)))
	case protocol.ActionClick:
		n := m.clickCount(button, p)
		C.riftMouse(ev.down, p, ev.button, C.int(n))
		C.riftMouse(ev.up, p, ev.button, C.int(n))
	}
	return nil
}

func (m *macInjector) Scroll(dx, dy int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(); err != nil {
		return err
	}
	px := int(math.Round(float64(dx) * pixelsPerWheel))
	py := int(math.Round(float64(dy) * pixelsPerWheel))
	if px != 0 || py != 0 {
		C.riftScroll(C.int(px), C.int(py))
	}
	return nil
}
