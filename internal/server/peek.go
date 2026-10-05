package server

import (
	"encoding/binary"
	"image"
	"log"
	"sync"
	"time"

	"github.com/HarshalPatel1972/rift/internal/protocol"
	"github.com/HarshalPatel1972/rift/internal/screen"
)

const (
	peekMinGap     = 66 * time.Millisecond  // ≤ 15 fps
	peekIdleGap    = 90 * time.Millisecond  // re-check rate when nothing changed
	peekAckTimeout = 3 * time.Second        // resend if the phone never acks
	peekErrBackoff = 700 * time.Millisecond // e.g. the lock screen is up
)

// peeker is one session's Screen Peek state. Frames are flow-controlled: the
// next one is captured only after the phone acks the last, so a slow link
// gets fewer, fresher frames instead of a growing backlog.
type peeker struct {
	mu     sync.Mutex
	req    screen.Request
	src    image.Rectangle // area of the last frame sent, for OpPoint
	change chan struct{}
	acks   chan uint32
}

func newPeeker() *peeker {
	return &peeker{change: make(chan struct{}, 1), acks: make(chan uint32, 1)}
}

func (p *peeker) set(r screen.Request) {
	p.mu.Lock()
	p.req = r
	p.mu.Unlock()
	select {
	case p.change <- struct{}{}:
	default:
	}
}

func (p *peeker) get() screen.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.req
}

func (p *peeker) ack(id uint32) {
	select {
	case p.acks <- id:
	default:
	}
}

func (s *Server) handlePeek(sess *session, p []byte) {
	if len(p) < 7 {
		return
	}
	req := screen.Request{
		Mode:     p[0],
		MaxWidth: int(binary.LittleEndian.Uint16(p[1:])),
		Span:     int(binary.LittleEndian.Uint16(p[3:])),
		Aspect:   int(binary.LittleEndian.Uint16(p[5:])),
	}
	if req.Mode > protocol.PeekCaret {
		return
	}
	s.mu.Lock()
	allowed := s.status.PeekAllowed && s.host.Screen != nil
	s.mu.Unlock()
	if !allowed {
		req.Mode = protocol.PeekOff
	}
	req.MaxWidth = min(max(req.MaxWidth, 160), 1920)
	sess.peek.set(req)
	s.update(func(st *Status) { st.Peeking = req.Mode != protocol.PeekOff })
}

// streamPeek runs for the life of a session, sending frames while the phone
// has Screen Peek open.
func (s *Server) streamPeek(sess *session) {
	capt := s.host.Screen()
	pk := sess.peek
	var id uint32
	var lastHash uint32
	var lastErr string

	for {
		req := pk.get()
		if req.Mode == protocol.PeekOff {
			select {
			case <-sess.done:
				return
			case <-pk.change:
				lastHash = 0 // always send a first frame
				continue
			}
		}

		start := time.Now()
		f, err := capt.Grab(req)
		if err != nil {
			if err.Error() != lastErr {
				log.Printf("peek: %v", err)
				lastErr = err.Error()
			}
			if !sleepOrDone(sess, pk, peekErrBackoff) {
				return
			}
			continue
		}
		lastErr = ""

		if f.JPEG == nil || f.Hash == lastHash {
			// Nothing moved: idle cheaply until something might have.
			if !sleepOrDone(sess, pk, peekIdleGap) {
				return
			}
			continue
		}

		id++
		hdr := make([]byte, 12, 12+len(f.JPEG))
		binary.LittleEndian.PutUint32(hdr, id)
		binary.LittleEndian.PutUint16(hdr[4:], uint16(f.W))
		binary.LittleEndian.PutUint16(hdr[6:], uint16(f.H))
		binary.LittleEndian.PutUint16(hdr[8:], uint16(int16(clamp16(f.CursorX))))
		binary.LittleEndian.PutUint16(hdr[10:], uint16(int16(clamp16(f.CursorY))))
		pk.mu.Lock()
		pk.src = f.Src
		pk.mu.Unlock()
		if err := sess.send(protocol.OpFrame, append(hdr, f.JPEG...)); err != nil {
			return
		}
		lastHash = f.Hash

		// Wait for the phone to show it (or give up and send a fresh one).
		deadline := time.NewTimer(peekAckTimeout)
	wait:
		for {
			select {
			case <-sess.done:
				deadline.Stop()
				return
			case got := <-pk.acks:
				if got == id {
					break wait
				}
			case <-pk.change:
				lastHash = 0
				break wait
			case <-deadline.C:
				break wait
			}
		}
		deadline.Stop()

		if gap := peekMinGap - time.Since(start); gap > 0 {
			if !sleepOrDone(sess, pk, gap) {
				return
			}
		}
	}
}

func clamp16(v int) int { return min(max(v, -32768), 32767) }

// sleepOrDone waits d, waking early on a mode change. It reports false once
// the session has ended.
func sleepOrDone(sess *session, pk *peeker, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-sess.done:
		return false
	case <-pk.change:
		// Put the signal back so the main loop sees the new mode.
		select {
		case pk.change <- struct{}{}:
		default:
		}
		return true
	case <-t.C:
		return true
	}
}

// point moves the pointer to (x, y) on the last frame (0..65535 on each axis)
// and optionally clicks there — tap-to-click on the Screen Peek view.
func (s *Server) point(sess *session, x, y uint16, click byte) error {
	sess.peek.mu.Lock()
	src := sess.peek.src
	sess.peek.mu.Unlock()
	if src.Empty() {
		return nil
	}
	px := src.Min.X + int(x)*(src.Dx()-1)/65535
	py := src.Min.Y + int(y)*(src.Dy()-1)/65535
	inj := s.host.Input
	if err := inj.MoveTo(px, py); err != nil {
		return err
	}
	switch click {
	case 1:
		return inj.Button(protocol.ButtonLeft, protocol.ActionClick)
	case 2:
		if err := inj.Button(protocol.ButtonLeft, protocol.ActionClick); err != nil {
			return err
		}
		return inj.Button(protocol.ButtonLeft, protocol.ActionClick)
	case 3:
		return inj.Button(protocol.ButtonRight, protocol.ActionClick)
	}
	return nil
}
