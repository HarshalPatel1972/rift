package server

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"image"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"

	"github.com/HarshalPatel1972/rift/internal/protocol"
	"github.com/HarshalPatel1972/rift/internal/screen"
)

// fakeScreen returns a new tiny "JPEG" for every grab, covering a 1000×500
// area at (100, 50).
type fakeScreen struct{ n int }

func (f *fakeScreen) Grab(req screen.Request) (*screen.Frame, error) {
	f.n++
	return &screen.Frame{
		JPEG: []byte(fmt.Sprintf("jpeg-%d-mode-%d", f.n, req.Mode)),
		W:    500, H: 250, CursorX: 10, CursorY: 20,
		Src:  image.Rect(100, 50, 1100, 550),
		Hash: uint32(f.n),
	}, nil
}

type recorder struct {
	mu    sync.Mutex
	calls []string
	fail  bool
}

func (r *recorder) add(s string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, s)
	if r.fail {
		return fmt.Errorf("blocked")
	}
	return nil
}
func (r *recorder) Edit(d int, s string) error { return r.add(fmt.Sprintf("edit %d %q", d, s)) }
func (r *recorder) Key(vk, m uint8) error      { return r.add(fmt.Sprintf("key %d %d", vk, m)) }
func (r *recorder) Move(x, y int) error        { return r.add(fmt.Sprintf("move %d %d", x, y)) }
func (r *recorder) MoveTo(x, y int) error      { return r.add(fmt.Sprintf("moveto %d %d", x, y)) }
func (r *recorder) Lock() error                { return r.add("lock") }
func (r *recorder) DisplayOff() error          { return r.add("display off") }
func (r *recorder) Sleep() error               { return r.add("sleep") }
func (r *recorder) Button(b, a uint8) error    { return r.add(fmt.Sprintf("button %d %d", b, a)) }
func (r *recorder) Scroll(x, y int) error      { return r.add(fmt.Sprintf("scroll %d %d", x, y)) }
func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func newTestServer(t *testing.T, key []byte) (*Server, *recorder, string) {
	t.Helper()
	rec := &recorder{}
	host := Host{
		Input:  rec,
		Screen: func() screen.Capturer { return &fakeScreen{} },
		Power:  rec,
	}
	s := New(host, key, fstest.MapFS{"index.html": {Data: []byte("phone")}}, []byte("png"), "TEST-PC")
	hs := httptest.NewServer(s.Handler())
	t.Cleanup(hs.Close)
	return s, rec, "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws"
}

// client is a Go port of the handshake in web/phone/app.js.
type client struct {
	conn  *websocket.Conn
	w     protocol.Writer
	r     protocol.Reader
	host  string
	flags byte
}

func dial(t *testing.T, url string, key []byte) (*client, error) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	keys := protocol.DeriveKeys(key)
	c := &client{conn: conn, w: protocol.Writer{Key: &keys.C2S}, r: protocol.Reader{Key: &keys.S2C}}

	_, ch, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(ch[:3], protocol.Magic[:]) || len(ch) != 3+protocol.ChallengeSize {
		t.Fatalf("bad challenge %x", ch)
	}
	nonce := make([]byte, protocol.ClientNonceSz)
	rand.Read(nonce)
	hello := append(append(append([]byte{}, ch[3:]...), nonce...), "Pixel 9"...)
	if err := conn.WriteMessage(websocket.BinaryMessage, c.w.Seal(protocol.OpHello, hello)); err != nil {
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	op, p, err := c.r.Open(msg)
	if err != nil || op != protocol.OpWelcome || !bytes.Equal(p[:16], nonce) {
		t.Fatalf("bad welcome op=%#x err=%v", op, err)
	}
	c.flags, c.host = p[16], string(p[17:])
	return c, nil
}

func (c *client) send(t *testing.T, op byte, p []byte) {
	t.Helper()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, c.w.Seal(op, p)); err != nil {
		t.Fatal(err)
	}
}

func closeCode(t *testing.T, conn *websocket.Conn) int {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if ce, ok := err.(*websocket.CloseError); ok {
				return ce.Code
			}
			t.Fatalf("expected close frame, got %v", err)
		}
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func i16s(a, b int16) []byte {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint16(p, uint16(a))
	binary.LittleEndian.PutUint16(p[2:], uint16(b))
	return p
}

func TestPairedClientDrivesInput(t *testing.T) {
	key := make([]byte, 32)
	s, rec, url := newTestServer(t, key)
	c, err := dial(t, url, key)
	if err != nil {
		t.Fatal(err)
	}
	if c.host != "TEST-PC" {
		t.Fatalf("host = %q", c.host)
	}
	st := s.Status()
	if !st.Connected || st.Device != "Pixel 9" {
		t.Fatalf("status = %+v", st)
	}

	c.send(t, protocol.OpEdit, append([]byte{3, 0}, "héllo 😀\n"...))
	c.send(t, protocol.OpKey, []byte{0x43, protocol.ModCtrl})
	c.send(t, protocol.OpMove, i16s(-12, 7))
	c.send(t, protocol.OpButton, []byte{protocol.ButtonRight, protocol.ActionClick})
	c.send(t, protocol.OpScroll, i16s(0, -240))

	want := []string{`edit 3 "héllo 😀\n"`, "key 67 1", "move -12 7", "button 1 2", "scroll 0 -240"}
	waitFor(t, "all inputs", func() bool { return len(rec.snapshot()) == len(want) })
	for i, got := range rec.snapshot() {
		if got != want[i] {
			t.Errorf("call %d = %s, want %s", i, got, want[i])
		}
	}
	waitFor(t, "activity count", func() bool { return s.Status().Activity == uint64(len(want)) })
}

func TestWrongKeyIsRejected(t *testing.T) {
	_, rec, url := newTestServer(t, make([]byte, 32))
	wrong := bytes.Repeat([]byte{9}, 32)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, ch, _ := conn.ReadMessage()
	keys := protocol.DeriveKeys(wrong)
	w := protocol.Writer{Key: &keys.C2S}
	conn.WriteMessage(websocket.BinaryMessage, w.Seal(protocol.OpHello, append(ch[3:], make([]byte, 16)...)))
	if code := closeCode(t, conn); code != CloseUnpaired {
		t.Fatalf("close code = %d", code)
	}
	if len(rec.snapshot()) != 0 {
		t.Fatal("input injected for unpaired client")
	}
}

func TestStaleChallengeIsRejected(t *testing.T) {
	key := make([]byte, 32)
	_, _, url := newTestServer(t, key)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.ReadMessage() // ignore the real challenge, answer a made-up one
	keys := protocol.DeriveKeys(key)
	w := protocol.Writer{Key: &keys.C2S}
	conn.WriteMessage(websocket.BinaryMessage, w.Seal(protocol.OpHello, make([]byte, 32)))
	if code := closeCode(t, conn); code != CloseUnpaired {
		t.Fatalf("close code = %d", code)
	}
}

func TestReplayDropsSession(t *testing.T) {
	key := make([]byte, 32)
	s, rec, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	frame := c.w.Seal(protocol.OpKey, []byte{0x41, 0})
	c.conn.WriteMessage(websocket.BinaryMessage, frame)
	c.conn.WriteMessage(websocket.BinaryMessage, frame)
	waitFor(t, "disconnect", func() bool { return !s.Status().Connected })
	if n := len(rec.snapshot()); n != 1 {
		t.Fatalf("replayed frame injected: %d calls", n)
	}
}

func TestNewestPhoneWins(t *testing.T) {
	key := make([]byte, 32)
	_, _, url := newTestServer(t, key)
	first, _ := dial(t, url, key)
	if _, err := dial(t, url, key); err != nil {
		t.Fatal(err)
	}
	if code := closeCode(t, first.conn); code != CloseReplaced {
		t.Fatalf("close code = %d", code)
	}
}

func TestPauseBlocksInputAndReachesPhone(t *testing.T) {
	key := make([]byte, 32)
	s, rec, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	s.SetPaused(true)

	// The phone hears about it via an immediate PING.
	var ping []byte
	c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for ping == nil {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		op, p, err := c.r.Open(msg)
		if err != nil {
			t.Fatal(err)
		}
		if op == protocol.OpPing && p[10]&protocol.FlagPaused != 0 {
			ping = p
		}
	}

	c.send(t, protocol.OpKey, []byte{0x41, 0})
	// Frames are handled in order, so once this PONG registers, the key above
	// has been processed (and dropped) while still paused.
	time.Sleep(5 * time.Millisecond)
	c.send(t, protocol.OpPong, ping[:8])
	waitFor(t, "pong", func() bool { return s.Status().RTTms > 0 })
	s.SetPaused(false)
	c.send(t, protocol.OpKey, []byte{0x42, 0})
	waitFor(t, "unpaused input", func() bool { return len(rec.snapshot()) == 1 })
	if got := rec.snapshot()[0]; got != "key 66 0" {
		t.Fatalf("got %s", got)
	}
}

func TestRotatingKeyUnpairsPhone(t *testing.T) {
	key := make([]byte, 32)
	s, _, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	s.SetKey(bytes.Repeat([]byte{1}, 32))
	if code := closeCode(t, c.conn); code != CloseUnpaired {
		t.Fatalf("close code = %d", code)
	}
}

func TestPongMeasuresLatency(t *testing.T) {
	key := make([]byte, 32)
	s, _, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	op, p, err := c.r.Open(msg)
	if err != nil || op != protocol.OpPing {
		t.Fatalf("op=%#x err=%v", op, err)
	}
	time.Sleep(20 * time.Millisecond)
	c.send(t, protocol.OpPong, p[:8])
	waitFor(t, "rtt", func() bool { return s.Status().RTTms >= 20 })
}

func TestInjectionFailureSurfacesWarning(t *testing.T) {
	key := make([]byte, 32)
	s, rec, url := newTestServer(t, key)
	rec.mu.Lock()
	rec.fail = true
	rec.mu.Unlock()
	c, _ := dial(t, url, key)
	c.send(t, protocol.OpKey, []byte{0x41, 0})
	waitFor(t, "warning", func() bool { return s.Status().Warning != "" })
}

func TestCrossOriginSocketRefused(t *testing.T) {
	_, _, url := newTestServer(t, make([]byte, 32))
	_, resp, err := websocket.DefaultDialer.Dial(url, map[string][]string{"Origin": {"http://evil.example"}})
	if err == nil || resp == nil || resp.StatusCode != 403 {
		t.Fatalf("cross-origin upgrade allowed: %v", err)
	}
}

// readOp returns the next server message with the given op, skipping pings.
func (c *client) readOp(t *testing.T, want byte) []byte {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for op %#x: %v", want, err)
		}
		op, p, err := c.r.Open(msg)
		if err != nil {
			t.Fatal(err)
		}
		if op == want {
			return p
		}
	}
}

func peekReq(mode byte) []byte {
	p := make([]byte, 7)
	p[0] = mode
	binary.LittleEndian.PutUint16(p[1:], 800)
	binary.LittleEndian.PutUint16(p[3:], 600)
	binary.LittleEndian.PutUint16(p[5:], 750)
	return p
}

func ackFor(frame []byte) []byte { return frame[:4] }

func TestScreenPeekStreamsWithFlowControl(t *testing.T) {
	key := make([]byte, 32)
	s, _, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	if c.flags&protocol.FlagPeek == 0 || c.flags&protocol.FlagPower == 0 {
		t.Fatalf("capabilities not advertised: flags=%08b", c.flags)
	}

	c.send(t, protocol.OpPeek, peekReq(protocol.PeekCursor))
	f1 := c.readOp(t, protocol.OpFrame)
	if w, h := binary.LittleEndian.Uint16(f1[4:]), binary.LittleEndian.Uint16(f1[6:]); w != 500 || h != 250 {
		t.Fatalf("frame size %dx%d", w, h)
	}
	if !strings.HasSuffix(string(f1[12:]), "mode-2") {
		t.Fatalf("payload %q", f1[12:])
	}
	waitFor(t, "peeking status", func() bool { return s.Status().Peeking })

	// No ack yet: the server must not push another frame.
	c.conn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break // timeout: good
		}
		if op, _, _ := c.r.Open(msg); op == protocol.OpFrame {
			t.Fatal("frame sent before ack")
		}
	}
	// The read deadline error poisons gorilla's reader, so finish on a new client.
	c2, _ := dial(t, url, key)
	c2.send(t, protocol.OpPeek, peekReq(protocol.PeekScreen))
	f := c2.readOp(t, protocol.OpFrame)
	c2.send(t, protocol.OpAck, ackFor(f))
	f2 := c2.readOp(t, protocol.OpFrame)
	if binary.LittleEndian.Uint32(f2) != binary.LittleEndian.Uint32(f)+1 {
		t.Fatal("frame ids not sequential")
	}

	c2.send(t, protocol.OpPeek, peekReq(protocol.PeekOff))
	waitFor(t, "peek stopped", func() bool { return !s.Status().Peeking })
}

func TestPointMapsFrameToScreenAndClicks(t *testing.T) {
	key := make([]byte, 32)
	_, rec, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	c.send(t, protocol.OpPeek, peekReq(protocol.PeekScreen))
	c.readOp(t, protocol.OpFrame)

	p := make([]byte, 5)
	binary.LittleEndian.PutUint16(p, 32768) // middle
	binary.LittleEndian.PutUint16(p[2:], 0) // top
	p[4] = 3                                // right-click
	c.send(t, protocol.OpPoint, p)
	waitFor(t, "point", func() bool { return len(rec.snapshot()) == 2 })
	got := rec.snapshot()
	if got[0] != "moveto 599 50" || got[1] != "button 1 2" {
		t.Fatalf("got %v", got)
	}
}

func TestPeekDisallowedByHost(t *testing.T) {
	key := make([]byte, 32)
	s, _, url := newTestServer(t, key)
	s.SetPeekAllowed(false)
	c, _ := dial(t, url, key)
	if c.flags&protocol.FlagPeek != 0 {
		t.Fatal("peek advertised while disallowed")
	}
	c.send(t, protocol.OpPeek, peekReq(protocol.PeekScreen))
	time.Sleep(200 * time.Millisecond)
	if s.Status().Peeking {
		t.Fatal("peek started while disallowed")
	}
}

func TestPowerActionsAndSleepTimer(t *testing.T) {
	key := make([]byte, 32)
	s, rec, url := newTestServer(t, key)
	c, _ := dial(t, url, key)
	act := func(a byte, arg uint32) {
		p := make([]byte, 5)
		p[0] = a
		binary.LittleEndian.PutUint32(p[1:], arg)
		c.send(t, protocol.OpAction, p)
	}
	act(protocol.ActionLock, 0)
	act(protocol.ActionDisplayOff, 0)
	waitFor(t, "lock + display off", func() bool { return len(rec.snapshot()) == 2 })

	act(protocol.ActionSleepTimer, 30)
	waitFor(t, "timer set", func() bool { return !s.Status().SleepAt.IsZero() })
	if left := time.Until(s.Status().SleepAt); left < 29*time.Minute || left > 30*time.Minute {
		t.Fatalf("timer %v", left)
	}
	// The phone sees the countdown in pings.
	ping := c.readOp(t, protocol.OpPing)
	for binary.LittleEndian.Uint32(ping[11:]) == 0 {
		ping = c.readOp(t, protocol.OpPing)
	}
	act(protocol.ActionSleepTimer, 0)
	waitFor(t, "timer cancelled", func() bool { return s.Status().SleepAt.IsZero() })

	s.SetSleepTimer(30 * time.Millisecond)
	waitFor(t, "timer fired", func() bool {
		calls := rec.snapshot()
		return len(calls) == 3 && calls[2] == "sleep"
	})
}
