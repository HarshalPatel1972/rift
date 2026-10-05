//go:build windows

package screen

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
	"time"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

// Captures the real desktop (in memory only) to prove the GDI path and the
// Win32 struct layouts work. Skips on headless CI sessions with no desktop.
func TestGrabRealScreen(t *testing.T) {
	c := New()
	for _, tc := range []struct {
		name string
		req  Request
	}{
		{"screen", Request{Mode: protocol.PeekScreen, MaxWidth: 640}},
		{"cursor", Request{Mode: protocol.PeekCursor, MaxWidth: 640, Span: 800, Aspect: 600}},
		{"caret", Request{Mode: protocol.PeekCaret, MaxWidth: 640, Span: 800, Aspect: 600}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			f, err := c.Grab(tc.req)
			if err != nil {
				t.Skipf("no capturable desktop: %v", err)
			}
			if f.JPEG == nil {
				// Identical to the previous mode's frame; force a fresh encode.
				c.(*gdiCapturer).lastHash = 0
				if f, err = c.Grab(tc.req); err != nil || f.JPEG == nil {
					t.Fatalf("no frame: %v", err)
				}
			}
			img, err := jpeg.Decode(bytes.NewReader(f.JPEG))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != f.W || f.W > 640 || f.H <= 0 {
				t.Fatalf("decoded %v, frame %dx%d", img.Bounds(), f.W, f.H)
			}
			if f.Src.Empty() {
				t.Fatal("empty source rect")
			}
			t.Logf("%s: %dx%d from %v, %d KB, %v", tc.name, f.W, f.H, f.Src, len(f.JPEG)/1024, time.Since(start))
		})
	}
}

func TestFollowerKeepsWindowSteady(t *testing.T) {
	var f follower
	mon := image.Rect(0, 0, 1920, 1080)
	a := f.place(image.Pt(500, 500), 800, 400, mon)
	b := f.place(image.Pt(540, 510), 800, 400, mon) // small move: window stays
	if a != b {
		t.Fatalf("window jumped for a small move: %v -> %v", a, b)
	}
	c := f.place(image.Pt(1900, 1070), 800, 400, mon) // far corner: recenters, clamped
	if !c.In(mon) || c.Dx() != 800 || c.Dy() != 400 {
		t.Fatalf("bad window %v", c)
	}
}
