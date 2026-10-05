//go:build darwin && cgo

package screen

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=12.3
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework Foundation -framework ScreenCaptureKit

#import <Foundation/Foundation.h>
#import <ApplicationServices/ApplicationServices.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>

typedef struct { double x, y, w, h, scale; } RiftDisplay;

static void riftCursor(double *x, double *y) {
	CGEventRef e = CGEventCreate(NULL);
	CGPoint p = CGEventGetLocation(e);
	CFRelease(e);
	*x = p.x;
	*y = p.y;
}

// The display under (x, y), in global points, plus its pixels-per-point.
static RiftDisplay riftDisplayAt(double x, double y, uint32_t *idOut) {
	CGDirectDisplayID id = CGMainDisplayID();
	uint32_t n = 0;
	CGGetDisplaysWithPoint(CGPointMake(x, y), 1, &id, &n);
	if (n == 0) id = CGMainDisplayID();
	CGRect b = CGDisplayBounds(id);
	double scale = 1;
	CGDisplayModeRef mode = CGDisplayCopyDisplayMode(id);
	if (mode) {
		if (CGDisplayModeGetWidth(mode) > 0)
			scale = (double)CGDisplayModeGetPixelWidth(mode) / (double)CGDisplayModeGetWidth(mode);
		CGDisplayModeRelease(mode);
	}
	*idOut = id;
	return (RiftDisplay){b.origin.x, b.origin.y, b.size.width, b.size.height, scale};
}

// The focused text caret via Accessibility, in global points.
static int riftCaret(double *x, double *y) {
	int ok = 0;
	AXUIElementRef sys = AXUIElementCreateSystemWide();
	CFTypeRef focused = NULL, range = NULL, bounds = NULL;
	if (AXUIElementCopyAttributeValue(sys, kAXFocusedUIElementAttribute, &focused) == kAXErrorSuccess &&
	    AXUIElementCopyAttributeValue(focused, kAXSelectedTextRangeAttribute, &range) == kAXErrorSuccess &&
	    AXUIElementCopyParameterizedAttributeValue(focused, kAXBoundsForRangeParameterizedAttribute, range, &bounds) == kAXErrorSuccess) {
		CGRect r;
		if (AXValueGetValue(bounds, kAXValueCGRectType, &r) && r.size.height > 0) {
			*x = r.origin.x;
			*y = r.origin.y + r.size.height;
			ok = 1;
		}
	}
	if (bounds) CFRelease(bounds);
	if (range) CFRelease(range);
	if (focused) CFRelease(focused);
	CFRelease(sys);
	return ok;
}

static int riftHasScreenAccess(void) { return CGPreflightScreenCaptureAccess() ? 1 : 0; }

// Captures a rectangle (global points) of one display into an RGBA buffer
// of dw×dh pixels. Returns 0 on success, 1 on failure, 2 without Screen
// Recording permission, 3 on macOS < 14.
static int riftCapture(uint32_t displayID, double gx, double gy, double gw, double gh,
                       int dw, int dh, uint8_t *out) {
	if (!CGPreflightScreenCaptureAccess()) return 2;
	if (@available(macOS 14.0, *)) {
		// Shareable content is slow to fetch; reuse it for a few seconds.
		static SCShareableContent *content;
		static CFAbsoluteTime fetched;
		if (!content || CFAbsoluteTimeGetCurrent() - fetched > 5) {
			dispatch_semaphore_t sem = dispatch_semaphore_create(0);
			__block SCShareableContent *got = nil;
			[SCShareableContent getShareableContentExcludingDesktopWindows:NO
			                                           onScreenWindowsOnly:YES
			                                             completionHandler:^(SCShareableContent *c, NSError *e) {
				got = c;
				dispatch_semaphore_signal(sem);
			}];
			dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC));
			if (!got) return 1;
			content = got;
			fetched = CFAbsoluteTimeGetCurrent();
		}
		SCDisplay *display = nil;
		for (SCDisplay *d in content.displays) {
			if (d.displayID == displayID) { display = d; break; }
		}
		if (!display) { content = nil; return 1; }

		CGRect b = CGDisplayBounds(displayID);
		SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
		SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
		cfg.width = dw;
		cfg.height = dh;
		cfg.sourceRect = CGRectMake(gx - b.origin.x, gy - b.origin.y, gw, gh);
		cfg.showsCursor = YES;

		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		__block CGImageRef img = NULL;
		[SCScreenshotManager captureImageWithFilter:filter configuration:cfg completionHandler:^(CGImageRef i, NSError *e) {
			if (i) img = CGImageRetain(i);
			dispatch_semaphore_signal(sem);
		}];
		dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC));
		if (!img) return 1;

		CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
		CGContextRef ctx = CGBitmapContextCreate(out, dw, dh, 8, dw * 4, cs,
			kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
		CGContextDrawImage(ctx, CGRectMake(0, 0, dw, dh), img);
		CGContextRelease(ctx);
		CGColorSpaceRelease(cs);
		CGImageRelease(img);
		return 0;
	}
	return 3;
}
*/
import "C"

import (
	"errors"
	"image"
	"math"
	"unsafe"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

var (
	// ErrNoScreenAccess means macOS hasn't granted Screen Recording permission.
	ErrNoScreenAccess = errors.New("screen: RIFT needs Screen Recording permission")
	errOldMacOS       = errors.New("screen: Screen Peek needs macOS 14 or later")
	errCapture        = errors.New("screen: capture failed")
)

type sckCapturer struct {
	follow   follower
	lastHash uint32
}

// New returns the ScreenCaptureKit capturer. Coordinates are global display
// points, matching the injector's MoveTo.
func New() Capturer { return &sckCapturer{} }

func (c *sckCapturer) Grab(req Request) (*Frame, error) {
	var cx, cy C.double
	C.riftCursor(&cx, &cy)
	ptr := image.Pt(int(cx), int(cy))

	var id C.uint32_t
	d := C.riftDisplayAt(cx, cy, &id)
	mon := image.Rect(int(d.x), int(d.y), int(d.x+d.w), int(d.y+d.h))
	scale := float64(d.scale)

	src := mon
	quality := 70
	if req.Mode == protocol.PeekCursor || req.Mode == protocol.PeekCaret {
		focus := ptr
		if req.Mode == protocol.PeekCaret {
			var kx, ky C.double
			if C.riftCaret(&kx, &ky) == 1 {
				if p := image.Pt(int(kx), int(ky)); p.In(mon) {
					focus = p
				}
			}
		}
		// The phone's span is in pixels; convert to points so zoom feels the
		// same on Retina and non-Retina displays.
		span := min(max(int(float64(req.Span)/scale), 160), mon.Dx())
		aspect := min(max(req.Aspect, 300), 2500)
		src = c.follow.place(focus, span, span*aspect/1000, mon)
		quality = 82
	}
	dw, dh := fit(int(math.Round(float64(src.Dx())*scale)), int(math.Round(float64(src.Dy())*scale)), req.MaxWidth)

	img := image.NewRGBA(image.Rect(0, 0, dw, dh))
	switch C.riftCapture(id, C.double(src.Min.X), C.double(src.Min.Y), C.double(src.Dx()), C.double(src.Dy()),
		C.int(dw), C.int(dh), (*C.uint8_t)(unsafe.Pointer(&img.Pix[0]))) {
	case 0:
	case 2:
		return nil, ErrNoScreenAccess
	case 3:
		return nil, errOldMacOS
	default:
		return nil, errCapture
	}

	fx := (ptr.X - src.Min.X) * dw / src.Dx()
	fy := (ptr.Y - src.Min.Y) * dh / src.Dy()
	f, err := encode(img, src, fx, fy, quality, c.lastHash)
	if err == nil && f.JPEG != nil {
		c.lastHash = f.Hash
	}
	return f, err
}

// HasAccess reports whether Screen Recording permission is granted.
func HasAccess() bool { return C.riftHasScreenAccess() == 1 }
