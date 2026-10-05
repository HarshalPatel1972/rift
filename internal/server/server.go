// Package server is the LAN-facing half of RIFT: it serves the phone web app
// and runs the encrypted input channel over WebSocket.
package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"

	"github.com/HarshalPatel1972/rift/internal/injector"
	"github.com/HarshalPatel1972/rift/internal/power"
	"github.com/HarshalPatel1972/rift/internal/protocol"
	"github.com/HarshalPatel1972/rift/internal/screen"
)

const (
	helloTimeout = 10 * time.Second
	readTimeout  = 10 * time.Second // > 4 missed pings
	writeTimeout = 5 * time.Second
	pingEvery    = 2 * time.Second
	maxDelete    = 4096

	// Close codes the phone understands (4000-4999 are app-defined).
	CloseUnpaired = 4401 // wrong or revoked key: phone stops and asks to rescan
	CloseReplaced = 4409 // another phone took over
	CloseKicked   = 4410 // host pressed Disconnect
)

// Host is what the server drives on this machine. Screen and Power are
// optional; the phone hides features the host doesn't offer.
type Host struct {
	Input  injector.Injector
	Screen func() screen.Capturer // one capturer per session
	Power  power.Controller
}

// Status is a snapshot of the connection for the desktop dashboard.
type Status struct {
	Connected   bool      `json:"connected"`
	Device      string    `json:"device,omitempty"`
	RemoteAddr  string    `json:"remoteAddr,omitempty"`
	Since       time.Time `json:"since,omitzero"`
	RTTms       int       `json:"rttMs"`
	Paused      bool      `json:"paused"`
	Activity    uint64    `json:"activity"`
	Warning     string    `json:"warning,omitempty"`
	Peeking     bool      `json:"peeking"`
	PeekAllowed bool      `json:"peekAllowed"`
	SleepAt     time.Time `json:"sleepAt,omitzero"`
}

type Server struct {
	host     Host
	assets   fs.FS
	icon     []byte
	hostName string
	epoch    time.Time
	upgrader websocket.Upgrader

	mu         sync.Mutex
	keys       protocol.Keys
	active     *session
	status     Status
	subs       map[chan struct{}]struct{}
	sleepTimer *time.Timer
}

// New builds a server. assets is the phone web app; icon is served at
// /icon.png for the home-screen shortcut.
func New(host Host, key []byte, assets fs.FS, icon []byte, hostName string) *Server {
	s := &Server{
		host:     host,
		assets:   assets,
		icon:     icon,
		hostName: hostName,
		epoch:    time.Now(),
		keys:     protocol.DeriveKeys(key),
		subs:     map[chan struct{}]struct{}{},
		status:   Status{PeekAllowed: true},
	}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 64 * 1024,
		CheckOrigin:     sameOrigin,
	}
	return s
}

// sameOrigin rejects cross-site pages that try to open the socket. Frames are
// authenticated anyway; this just refuses them before any work is done.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

func init() {
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
	mime.AddExtensionType(".woff2", "font/woff2")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	files := http.FileServerFS(s.assets)
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(s.icon)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws://"+r.Host+
			"; img-src 'self' data: blob:; font-src 'self'; style-src 'self'; script-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	return mux
}

// --- dashboard-facing API ---

func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Subscribe returns a channel that receives a (coalesced) tick whenever
// Status changes. Call cancel when done.
func (s *Server) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

// notifyLocked must be called with s.mu held.
func (s *Server) notifyLocked() {
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // a tick is already pending; consumers read fresh Status
		}
	}
}

func (s *Server) update(f func(*Status)) {
	s.mu.Lock()
	f(&s.status)
	s.notifyLocked()
	s.mu.Unlock()
}

// SetKey swaps the pairing key and drops the current phone, which must
// rescan the new QR code.
func (s *Server) SetKey(key []byte) {
	s.mu.Lock()
	s.keys = protocol.DeriveKeys(key)
	old := s.active
	s.mu.Unlock()
	if old != nil {
		old.kick(CloseUnpaired, "pairing revoked")
	}
}

func (s *Server) SetPaused(p bool) {
	s.update(func(st *Status) { st.Paused = p })
	s.pingActive()
}

// SetPeekAllowed is the host's privacy switch for Screen Peek. Turning it
// off stops any stream immediately.
func (s *Server) SetPeekAllowed(on bool) {
	s.mu.Lock()
	s.status.PeekAllowed = on
	sess := s.active
	s.notifyLocked()
	s.mu.Unlock()
	if !on && sess != nil {
		sess.peek.set(screen.Request{Mode: protocol.PeekOff})
		s.update(func(st *Status) { st.Peeking = false })
	}
	s.pingActive()
}

func (s *Server) Disconnect() {
	s.mu.Lock()
	sess := s.active
	s.mu.Unlock()
	if sess != nil {
		sess.kick(CloseKicked, "disconnected by host")
	}
}

func (s *Server) pingActive() {
	s.mu.Lock()
	sess := s.active
	s.mu.Unlock()
	if sess != nil {
		s.ping(sess) // push state to the phone immediately
	}
}

// --- sessions ---

type session struct {
	conn      *websocket.Conn
	wmu       sync.Mutex
	w         protocol.Writer
	done      chan struct{}
	closeOnce sync.Once
	peek      *peeker
}

func (c *session) send(op byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.BinaryMessage, c.w.Seal(op, payload))
}

func (c *session) kick(code int, reason string) {
	c.wmu.Lock()
	c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
	c.wmu.Unlock()
	c.close()
}

func (c *session) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.conn.Close()
	})
}

// flags reports host state and capabilities to the phone.
func (s *Server) flagsLocked() byte {
	var f byte
	if s.status.Paused {
		f |= protocol.FlagPaused
	}
	if s.host.Screen != nil && s.status.PeekAllowed {
		f |= protocol.FlagPeek
	}
	if s.host.Power != nil {
		f |= protocol.FlagPower
	}
	return f
}

func (s *Server) ping(c *session) {
	s.mu.Lock()
	rtt, flags := s.status.RTTms, s.flagsLocked()
	var timer uint32
	if !s.status.SleepAt.IsZero() {
		timer = uint32(max(time.Until(s.status.SleepAt).Seconds(), 0))
	}
	s.mu.Unlock()
	p := make([]byte, 15)
	binary.LittleEndian.PutUint64(p, uint64(time.Since(s.epoch)))
	binary.LittleEndian.PutUint16(p[8:], uint16(min(rtt, 65535)))
	p[10] = flags
	binary.LittleEndian.PutUint32(p[11:], timer)
	c.send(protocol.OpPing, p)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the HTTP error
	}
	conn.SetReadLimit(protocol.MaxFrame)
	defer conn.Close()

	// 1. Fresh challenge, so a recorded HELLO can't be replayed later.
	var challenge [protocol.ChallengeSize]byte
	rand.Read(challenge[:])
	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := conn.WriteMessage(websocket.BinaryMessage, append(protocol.Magic[:], challenge[:]...)); err != nil {
		return
	}

	// 2. HELLO must decrypt under the current pairing key and echo the challenge.
	s.mu.Lock()
	keys := s.keys
	s.mu.Unlock()
	reader := protocol.Reader{Key: &keys.C2S}

	conn.SetReadDeadline(time.Now().Add(helloTimeout))
	mt, msg, err := conn.ReadMessage()
	if err != nil || mt != websocket.BinaryMessage {
		return
	}
	op, hello, err := reader.Open(msg)
	if err != nil || op != protocol.OpHello || len(hello) < protocol.ChallengeSize+protocol.ClientNonceSz ||
		subtle.ConstantTimeCompare(hello[:protocol.ChallengeSize], challenge[:]) != 1 {
		log.Printf("rejected unpaired client %s", r.RemoteAddr)
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(CloseUnpaired, "pairing rejected"), time.Now().Add(time.Second))
		return
	}
	clientNonce := hello[protocol.ChallengeSize : protocol.ChallengeSize+protocol.ClientNonceSz]
	device := cleanName(hello[protocol.ChallengeSize+protocol.ClientNonceSz:])

	sess := &session{conn: conn, w: protocol.Writer{Key: &keys.S2C}, done: make(chan struct{}), peek: newPeeker()}
	defer sess.close()

	// 3. One phone at a time: the newest one wins.
	s.mu.Lock()
	prev := s.active
	s.active = sess
	st := s.status
	s.status = Status{
		Connected: true, Device: device, RemoteAddr: r.RemoteAddr, Since: time.Now(),
		Paused: st.Paused, Activity: st.Activity, PeekAllowed: st.PeekAllowed, SleepAt: st.SleepAt,
	}
	flags := s.flagsLocked()
	s.notifyLocked()
	s.mu.Unlock()
	if prev != nil {
		prev.kick(CloseReplaced, "replaced by another device")
	}
	log.Printf("paired: %s (%s)", device, r.RemoteAddr)

	defer func() {
		s.mu.Lock()
		if s.active == sess {
			s.active = nil
			s.status.Connected, s.status.Device, s.status.RemoteAddr = false, "", ""
			s.status.Since, s.status.RTTms, s.status.Warning, s.status.Peeking = time.Time{}, 0, "", false
			s.notifyLocked()
		}
		s.mu.Unlock()
		log.Printf("disconnected: %s", device)
	}()

	// 4. WELCOME proves we hold the key too and binds to the phone's nonce.
	if err := sess.send(protocol.OpWelcome, append(append(append([]byte{}, clientNonce...), flags), s.hostName...)); err != nil {
		return
	}

	go func() {
		s.ping(sess)
		t := time.NewTicker(pingEvery)
		defer t.Stop()
		for {
			select {
			case <-sess.done:
				return
			case <-t.C:
				s.ping(sess)
			}
		}
	}()
	if s.host.Screen != nil {
		go s.streamPeek(sess)
	}

	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.BinaryMessage {
			continue
		}
		op, payload, err := reader.Open(msg)
		if err != nil {
			log.Printf("dropping session %s: %v", device, err)
			return
		}
		s.dispatch(sess, op, payload)
	}
}

func cleanName(b []byte) string {
	name := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.ToValidUTF8(string(b), ""))
	if utf8.RuneCountInString(name) > 48 {
		name = string([]rune(name)[:48])
	}
	if name == "" {
		name = "Phone"
	}
	return name
}

func (s *Server) dispatch(sess *session, op byte, p []byte) {
	switch op {
	case protocol.OpPong:
		if len(p) >= 8 {
			sent := time.Duration(binary.LittleEndian.Uint64(p))
			rtt := int((time.Since(s.epoch) - sent).Milliseconds())
			s.update(func(st *Status) { st.RTTms = max(rtt, 0) })
		}
		return
	case protocol.OpPeek:
		s.handlePeek(sess, p)
		return
	case protocol.OpAck:
		if len(p) >= 4 {
			sess.peek.ack(binary.LittleEndian.Uint32(p))
		}
		return
	}

	s.mu.Lock()
	paused := s.status.Paused
	s.mu.Unlock()
	if paused {
		return
	}

	inj := s.host.Input
	var err error
	switch op {
	case protocol.OpEdit:
		if len(p) < 2 {
			return
		}
		del := min(int(binary.LittleEndian.Uint16(p)), maxDelete)
		err = inj.Edit(del, strings.ToValidUTF8(string(p[2:]), ""))
	case protocol.OpKey:
		if len(p) < 2 {
			return
		}
		err = inj.Key(p[0], p[1])
	case protocol.OpMove:
		if len(p) < 4 {
			return
		}
		err = inj.Move(protocol.I16(p, 0), protocol.I16(p, 2))
	case protocol.OpButton:
		if len(p) < 2 {
			return
		}
		err = inj.Button(p[0], p[1])
	case protocol.OpScroll:
		if len(p) < 4 {
			return
		}
		err = inj.Scroll(protocol.I16(p, 0), protocol.I16(p, 2))
	case protocol.OpPoint:
		if len(p) < 5 {
			return
		}
		err = s.point(sess, binary.LittleEndian.Uint16(p), binary.LittleEndian.Uint16(p[2:]), p[4])
	case protocol.OpAction:
		if len(p) < 5 {
			return
		}
		err = s.action(p[0], binary.LittleEndian.Uint32(p[1:]))
	default:
		return
	}

	warning := ""
	if err != nil {
		warning = "Input blocked — the focused window is probably running as administrator. Run RIFT as administrator to control it."
	}
	s.update(func(st *Status) {
		st.Activity++
		if st.Warning != warning {
			if warning != "" {
				log.Printf("inject: %v", err)
			}
			st.Warning = warning
		}
	})
}
