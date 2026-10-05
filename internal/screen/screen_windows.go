//go:build windows

package screen

import (
	"fmt"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetCursorInfo      = user32.NewProc("GetCursorInfo")
	procGetIconInfo        = user32.NewProc("GetIconInfo")
	procDrawIconEx         = user32.NewProc("DrawIconEx")
	procMonitorFromPoint   = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfo     = user32.NewProc("GetMonitorInfoW")
	procGetGUIThreadInfo   = user32.NewProc("GetGUIThreadInfo")
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procStretchBlt         = gdi32.NewProc("StretchBlt")
	procSetStretchBltMode  = gdi32.NewProc("SetStretchBltMode")
	procSetBrushOrgEx      = gdi32.NewProc("SetBrushOrgEx")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

const (
	srccopy      = 0x00CC0020
	captureblt   = 0x40000000 // include layered windows
	halftone     = 4
	diNormal     = 3
	cursorShown  = 1
	monitorNear  = 2
	dibRGBColors = 0
)

type rect struct{ Left, Top, Right, Bottom int32 }

func (r rect) image() image.Rectangle {
	return image.Rect(int(r.Left), int(r.Top), int(r.Right), int(r.Bottom))
}

type point struct{ X, Y int32 }

type cursorInfo struct {
	cbSize  uint32
	flags   uint32
	hCursor uintptr
	pt      point
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

type guiThreadInfo struct {
	cbSize        uint32
	flags         uint32
	hwndActive    uintptr
	hwndFocus     uintptr
	hwndCapture   uintptr
	hwndMenuOwner uintptr
	hwndMoveSize  uintptr
	hwndCaret     uintptr
	rcCaret       rect
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type gdiCapturer struct {
	follow   follower
	lastHash uint32
}

// New returns the GDI-based capturer.
func New() Capturer { return &gdiCapturer{} }

func cursor() (cursorInfo, bool) {
	ci := cursorInfo{cbSize: uint32(unsafe.Sizeof(cursorInfo{}))}
	r, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&ci)))
	return ci, r != 0
}

// monitorAt returns the bounds of the monitor containing p.
func monitorAt(p point) image.Rectangle {
	var h uintptr
	if unsafe.Sizeof(uintptr(0)) == 8 {
		// POINT is passed by value; on x64 it fits in one register.
		h, _, _ = procMonitorFromPoint.Call(uintptr(uint32(p.X))|uintptr(uint32(p.Y))<<32, monitorNear)
	} else {
		h, _, _ = procMonitorFromPoint.Call(uintptr(p.X), uintptr(p.Y), monitorNear)
	}
	mi := monitorInfo{cbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	procGetMonitorInfo.Call(h, uintptr(unsafe.Pointer(&mi)))
	return mi.rcMonitor.image()
}

// caret finds the focused text caret, if the app exposes a system caret.
func caret() (image.Point, bool) {
	gti := guiThreadInfo{cbSize: uint32(unsafe.Sizeof(guiThreadInfo{}))}
	if r, _, _ := procGetGUIThreadInfo.Call(0, uintptr(unsafe.Pointer(&gti))); r == 0 || gti.hwndCaret == 0 {
		return image.Point{}, false
	}
	p := point{gti.rcCaret.Left, gti.rcCaret.Bottom}
	if r, _, _ := procClientToScreen.Call(gti.hwndCaret, uintptr(unsafe.Pointer(&p))); r == 0 {
		return image.Point{}, false
	}
	return image.Pt(int(p.X), int(p.Y)), true
}

func (c *gdiCapturer) Grab(req Request) (*Frame, error) {
	ci, ok := cursor()
	if !ok {
		return nil, fmt.Errorf("screen: GetCursorInfo failed")
	}
	ptr := image.Pt(int(ci.pt.X), int(ci.pt.Y))
	mon := monitorAt(ci.pt)
	if mon.Empty() {
		return nil, fmt.Errorf("screen: no monitor")
	}

	src := mon
	quality := 70
	if req.Mode == protocol.PeekCursor || req.Mode == protocol.PeekCaret {
		focus := ptr
		if req.Mode == protocol.PeekCaret {
			if p, ok := caret(); ok && p.In(mon) {
				focus = p
			}
		}
		span := min(max(req.Span, 240), mon.Dx())
		aspect := min(max(req.Aspect, 300), 2500)
		src = c.follow.place(focus, span, span*aspect/1000, mon)
		quality = 82
	}
	dw, dh := fit(src.Dx(), src.Dy(), req.MaxWidth)

	img, err := capture(src, dw, dh, ci)
	if err != nil {
		return nil, err
	}
	cx := (ptr.X - src.Min.X) * dw / src.Dx()
	cy := (ptr.Y - src.Min.Y) * dh / src.Dy()
	f, err := encode(img, src, cx, cy, quality, c.lastHash)
	if err == nil && f.JPEG != nil {
		c.lastHash = f.Hash
	}
	return f, err
}

// capture copies src from the screen into a dw×dh RGBA image, scaling with
// GDI's HALFTONE filter and drawing the pointer on top.
func capture(src image.Rectangle, dw, dh int, ci cursorInfo) (*image.RGBA, error) {
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("screen: GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("screen: CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bmp, _, _ := procCreateCompatBitmap.Call(screenDC, uintptr(dw), uintptr(dh))
	if bmp == 0 {
		return nil, fmt.Errorf("screen: CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bmp)

	old, _, _ := procSelectObject.Call(memDC, bmp)
	var ok uintptr
	if dw == src.Dx() && dh == src.Dy() {
		ok, _, _ = procBitBlt.Call(memDC, 0, 0, uintptr(dw), uintptr(dh), screenDC,
			uintptr(src.Min.X), uintptr(src.Min.Y), srccopy|captureblt)
	} else {
		procSetStretchBltMode.Call(memDC, halftone)
		procSetBrushOrgEx.Call(memDC, 0, 0, 0)
		ok, _, _ = procStretchBlt.Call(memDC, 0, 0, uintptr(dw), uintptr(dh), screenDC,
			uintptr(src.Min.X), uintptr(src.Min.Y), uintptr(src.Dx()), uintptr(src.Dy()), srccopy|captureblt)
	}
	if ok == 0 {
		procSelectObject.Call(memDC, old)
		// Fails on the secure desktop (lock screen, UAC prompts).
		return nil, fmt.Errorf("screen: blit failed (secure desktop?)")
	}
	drawCursor(memDC, src, dw, ci)
	procSelectObject.Call(memDC, old) // GetDIBits needs the bitmap deselected

	bi := bitmapInfoHeader{
		biSize:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:    int32(dw),
		biHeight:   -int32(dh), // top-down rows
		biPlanes:   1,
		biBitCount: 32,
	}
	// BITMAPINFO = header + one RGBQUAD; reserve room for it.
	var info struct {
		h      bitmapInfoHeader
		colors [4]byte
	}
	info.h = bi
	img := image.NewRGBA(image.Rect(0, 0, dw, dh))
	if r, _, _ := procGetDIBits.Call(memDC, bmp, 0, uintptr(dh), uintptr(unsafe.Pointer(&img.Pix[0])),
		uintptr(unsafe.Pointer(&info)), dibRGBColors); r == 0 {
		return nil, fmt.Errorf("screen: GetDIBits failed")
	}
	// BGRA -> RGBA, opaque.
	p := img.Pix
	for i := 0; i < len(p); i += 4 {
		p[i], p[i+2], p[i+3] = p[i+2], p[i], 0xff
	}
	return img, nil
}

func drawCursor(dc uintptr, src image.Rectangle, dw int, ci cursorInfo) {
	if ci.flags&cursorShown == 0 || ci.hCursor == 0 {
		return
	}
	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(ci.hCursor, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return
	}
	if ii.hbmMask != 0 {
		procDeleteObject.Call(ii.hbmMask)
	}
	if ii.hbmColor != 0 {
		procDeleteObject.Call(ii.hbmColor)
	}
	scale := float64(dw) / float64(src.Dx())
	x := int(float64(int(ci.pt.X)-int(ii.xHotspot)-src.Min.X) * scale)
	y := int(float64(int(ci.pt.Y)-int(ii.yHotspot)-src.Min.Y) * scale)
	// Keep the pointer at least native size when the frame is scaled down, so
	// it stays easy to spot on a phone.
	procDrawIconEx.Call(dc, uintptr(x), uintptr(y), ci.hCursor, 0, 0, 0, 0, diNormal)
}
