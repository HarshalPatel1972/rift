// Command riftdev serves the phone app against a pretend PC: input is only
// logged and Screen Peek shows a synthetic desktop, so the UI and protocol can
// be developed without typing into (or exposing) your real machine.
//
//	go run ./cmd/riftdev
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"sync"

	"github.com/HarshalPatel1972/rift/internal/config"
	"github.com/HarshalPatel1972/rift/internal/netinfo"
	"github.com/HarshalPatel1972/rift/internal/protocol"
	"github.com/HarshalPatel1972/rift/internal/screen"
	"github.com/HarshalPatel1972/rift/internal/server"
	"github.com/HarshalPatel1972/rift/web"
)

// fakePC is a 1920×1080 desktop whose pointer follows the injected input.
type fakePC struct {
	mu     sync.Mutex
	x, y   int
	typed  int // grows a "text line" as you type
	clicks int
}

var desk = image.Rect(0, 0, 1920, 1080)

func (p *fakePC) Edit(del int, s string) error {
	log.Printf("EDIT   del=%d insert=%q", del, s)
	p.mu.Lock()
	p.typed = max(0, p.typed-del+len([]rune(s)))
	p.mu.Unlock()
	return nil
}
func (p *fakePC) Key(vk, mods uint8) error {
	log.Printf("KEY    vk=%#02x mods=%04b", vk, mods)
	return nil
}
func (p *fakePC) Move(dx, dy int) error {
	log.Printf("MOVE   %d,%d", dx, dy)
	p.mu.Lock()
	p.x = min(max(p.x+dx, 0), desk.Dx()-1)
	p.y = min(max(p.y+dy, 0), desk.Dy()-1)
	p.mu.Unlock()
	return nil
}
func (p *fakePC) MoveTo(x, y int) error {
	log.Printf("MOVETO %d,%d", x, y)
	p.mu.Lock()
	p.x, p.y = x, y
	p.mu.Unlock()
	return nil
}
func (p *fakePC) Button(b, a uint8) error {
	log.Printf("BUTTON %d action=%d", b, a)
	p.mu.Lock()
	p.clicks++
	p.mu.Unlock()
	return nil
}
func (p *fakePC) Scroll(dx, dy int) error { log.Printf("SCROLL %d,%d", dx, dy); return nil }

func (p *fakePC) Lock() error       { log.Print("POWER  lock"); return nil }
func (p *fakePC) DisplayOff() error { log.Print("POWER  display off"); return nil }
func (p *fakePC) Sleep() error      { log.Print("POWER  sleep"); return nil }

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r.Intersect(img.Rect), &image.Uniform{c}, image.Point{}, draw.Src)
}

// render paints a toy desktop: wallpaper, two windows, a typed line and the pointer.
func (p *fakePC) render() *image.RGBA {
	p.mu.Lock()
	x, y, typed, clicks := p.x, p.y, p.typed, p.clicks
	p.mu.Unlock()
	img := image.NewRGBA(desk)
	for row := 0; row < desk.Dy(); row++ {
		c := color.RGBA{uint8(40 + row/12), 30, uint8(90 + row/10), 255}
		fill(img, image.Rect(0, row, desk.Dx(), row+1), c)
	}
	fill(img, image.Rect(120, 100, 1100, 760), color.RGBA{245, 243, 238, 255}) // editor window
	fill(img, image.Rect(120, 100, 1100, 140), color.RGBA{60, 60, 80, 255})
	for i := range 8 { // fake text lines
		fill(img, image.Rect(160, 180+i*44, 160+300+(i*97)%500, 196+i*44), color.RGBA{70, 70, 90, 255})
	}
	fill(img, image.Rect(160, 560, 160+min(typed*14, 900), 580), color.RGBA{124, 92, 255, 255})
	fill(img, image.Rect(1200, 300, 1800, 900), color.RGBA{30, 30, 40, 255}) // media window
	fill(img, image.Rect(1240, 340, 1760, 640), color.RGBA{255, 107, 107, 255})
	fill(img, image.Rect(1240, 700, 1240+(clicks*40)%520, 716), color.RGBA{6, 214, 160, 255})
	fill(img, image.Rect(x-2, y-2, x+16, y+22), color.RGBA{0, 0, 0, 255}) // pointer
	fill(img, image.Rect(x, y, x+12, y+18), color.RGBA{255, 255, 255, 255})
	return img
}

type fakeCapturer struct {
	pc   *fakePC
	last uint32
	win  image.Rectangle
}

func (c *fakeCapturer) Grab(req screen.Request) (*screen.Frame, error) {
	full := c.pc.render()
	c.pc.mu.Lock()
	px, py := c.pc.x, c.pc.y
	c.pc.mu.Unlock()
	src := desk
	if req.Mode == protocol.PeekCursor || req.Mode == protocol.PeekCaret {
		w := min(max(req.Span, 240), desk.Dx())
		h := w * max(req.Aspect, 300) / 1000
		if h > desk.Dy() {
			w, h = w*desk.Dy()/h, desk.Dy()
		}
		src = image.Rect(px-w/2, py-h/2, px-w/2+w, py-h/2+h)
		src = src.Add(image.Pt(max(0, -src.Min.X)-max(0, src.Max.X-desk.Dx()), max(0, -src.Min.Y)-max(0, src.Max.Y-desk.Dy())))
	}
	dw, dh := src.Dx(), src.Dy()
	if req.MaxWidth > 0 && dw > req.MaxWidth {
		dw, dh = req.MaxWidth, dh*req.MaxWidth/dw
	}
	out := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for yy := range dh {
		for xx := range dw {
			out.Set(xx, yy, full.At(src.Min.X+xx*src.Dx()/dw, src.Min.Y+yy*src.Dy()/dh))
		}
	}
	h := crc32.ChecksumIEEE(out.Pix)
	f := &screen.Frame{W: dw, H: dh, Src: src, Hash: h,
		CursorX: (px - src.Min.X) * dw / src.Dx(), CursorY: (py - src.Min.Y) * dh / src.Dy()}
	if h != c.last {
		var b bytes.Buffer
		jpeg.Encode(&b, out, &jpeg.Options{Quality: 75})
		f.JPEG = b.Bytes()
		c.last = h
	}
	return f, nil
}

func main() {
	port := flag.Int("port", 8090, "listen port")
	flag.Parse()

	pc := &fakePC{x: 600, y: 400}
	key := config.NewKey()
	srv := server.New(server.Host{
		Input:  pc,
		Screen: func() screen.Capturer { return &fakeCapturer{pc: pc} },
		Power:  pc,
	}, key, web.Phone, web.Icon, "riftdev")
	k := base64.RawURLEncoding.EncodeToString(key)
	fmt.Printf("local:  http://localhost:%d/#k=%s\n", *port, k)
	for _, a := range netinfo.Candidates() {
		fmt.Printf("%-7s http://%s:%d/#k=%s\n", a.Interface+":", a.IP, *port, k)
	}
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), srv.Handler()))
}
