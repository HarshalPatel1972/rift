// Package screen captures what's on the PC so the phone can "peek" at it
// from across the room.
package screen

import (
	"bytes"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"sync"
)

// ErrUnsupported is returned on hosts without a capture backend.
var ErrUnsupported = errors.New("screen: capture not supported on this platform")

// Request describes what the phone wants to see.
type Request struct {
	Mode     byte // protocol.Peek*
	MaxWidth int  // output width cap, in pixels
	Span     int  // follow modes: how many screen pixels wide the window is
	Aspect   int  // follow modes: height/width × 1000 of the phone's viewport
}

// Frame is one encoded capture.
type Frame struct {
	JPEG             []byte
	W, H             int
	CursorX, CursorY int             // pointer position in frame pixels (may be off-frame)
	Src              image.Rectangle // captured area, in screen pixels
	Hash             uint32          // of the raw pixels, to skip unchanged frames
}

// Capturer grabs frames. Implementations keep per-viewer state (the follow
// window), so use one per session.
type Capturer interface {
	Grab(req Request) (*Frame, error)
}

var jpegBuf = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// encode turns a capture into a Frame. Text-heavy follow modes get higher
// quality so small type stays legible.
func encode(img *image.RGBA, src image.Rectangle, cx, cy int, quality int, prevHash uint32) (*Frame, error) {
	h := crc32.ChecksumIEEE(img.Pix)
	f := &Frame{W: img.Rect.Dx(), H: img.Rect.Dy(), CursorX: cx, CursorY: cy, Src: src, Hash: h}
	if h == prevHash {
		return f, nil // unchanged; caller decides whether to resend
	}
	buf := jpegBuf.Get().(*bytes.Buffer)
	defer jpegBuf.Put(buf)
	buf.Reset()
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	f.JPEG = bytes.Clone(buf.Bytes())
	return f, nil
}

// follower keeps a reading window steady: it only moves once the point of
// interest leaves the middle of the window, so text doesn't jitter as you type.
type follower struct {
	win image.Rectangle
}

func (f *follower) place(focus image.Point, w, h int, bounds image.Rectangle) image.Rectangle {
	// Shrink to fit the monitor while keeping the phone's aspect ratio, so the
	// view fills the phone instead of letterboxing.
	if w > bounds.Dx() {
		w, h = bounds.Dx(), h*bounds.Dx()/w
	}
	if h > bounds.Dy() {
		w, h = w*bounds.Dy()/h, bounds.Dy()
	}
	w, h = max(w, 1), max(h, 1)
	inner := f.win.Inset(min(f.win.Dx(), f.win.Dy()) / 5)
	if f.win.Dx() != w || f.win.Dy() != h || !focus.In(inner) || !f.win.In(bounds) {
		r := image.Rect(focus.X-w/2, focus.Y-h/2, focus.X-w/2+w, focus.Y-h/2+h)
		// Clamp inside the monitor.
		if r.Min.X < bounds.Min.X {
			r = r.Add(image.Pt(bounds.Min.X-r.Min.X, 0))
		}
		if r.Min.Y < bounds.Min.Y {
			r = r.Add(image.Pt(0, bounds.Min.Y-r.Min.Y))
		}
		if r.Max.X > bounds.Max.X {
			r = r.Add(image.Pt(bounds.Max.X-r.Max.X, 0))
		}
		if r.Max.Y > bounds.Max.Y {
			r = r.Add(image.Pt(0, bounds.Max.Y-r.Max.Y))
		}
		f.win = r
	}
	return f.win
}

// fit scales (w, h) down to at most maxW wide, keeping the aspect ratio.
func fit(w, h, maxW int) (int, int) {
	if maxW <= 0 || w <= maxW {
		return w, h
	}
	return maxW, max(1, h*maxW/w)
}
