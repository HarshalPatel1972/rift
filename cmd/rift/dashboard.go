package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"

	"github.com/HarshalPatel1972/rift/internal/config"
	"github.com/HarshalPatel1972/rift/internal/netinfo"
	"github.com/HarshalPatel1972/rift/internal/perms"
	"github.com/HarshalPatel1972/rift/internal/server"
	"github.com/HarshalPatel1972/rift/web"
)

type dashboard struct {
	srv       *server.Server
	phonePort int
	adminKey  string
	onQuit    func()

	mu       sync.Mutex
	key      []byte
	settings config.Settings
}

func newDashboard(srv *server.Server, key []byte, phonePort int, adminKey string, settings config.Settings) *dashboard {
	return &dashboard{srv: srv, key: key, phonePort: phonePort, adminKey: adminKey, settings: settings}
}

func (d *dashboard) handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(web.Desktop))
	mux.HandleFunc("GET /icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(web.Icon)
	})
	mux.HandleFunc("GET /api/pairing", d.auth(d.pairing))
	mux.HandleFunc("POST /api/pairing/reset", d.auth(d.resetPairing))
	mux.HandleFunc("POST /api/pause", d.auth(d.pause))
	mux.HandleFunc("POST /api/disconnect", d.auth(func(w http.ResponseWriter, r *http.Request) {
		d.srv.Disconnect()
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("GET /api/settings", d.auth(d.settingsAPI))
	mux.HandleFunc("POST /api/settings", d.auth(d.settingsAPI))
	mux.HandleFunc("POST /api/sleep", d.auth(d.sleep))
	mux.HandleFunc("GET /api/permissions", d.auth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, perms.Check())
	}))
	mux.HandleFunc("POST /api/permissions", d.auth(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Kind string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := perms.Request(body.Kind); err != nil {
			log.Printf("permissions: %v", err)
		}
		writeJSON(w, perms.Check())
	}))
	mux.HandleFunc("GET /api/autostart", d.auth(d.autostart))
	mux.HandleFunc("POST /api/autostart", d.auth(d.autostart))
	mux.HandleFunc("POST /api/quit", d.auth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		if d.onQuit != nil {
			go d.onQuit()
		}
	}))
	mux.HandleFunc("GET /api/events", d.auth(d.events))
	return d.guard(mux)
}

// guard blocks DNS-rebinding (a hostile site resolving its own name to
// 127.0.0.1) by insisting on a loopback Host header, and sets strict headers.
func (d *dashboard) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || (host != "127.0.0.1" && host != "localhost") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; font-src 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// auth requires the per-launch admin key (header, or ?k= for EventSource,
// which cannot set headers). Other local pages can't learn it.
func (d *dashboard) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		k := r.Header.Get("X-Rift-Admin")
		if k == "" {
			k = r.URL.Query().Get("k")
		}
		if subtle.ConstantTimeCompare([]byte(k), []byte(d.adminKey)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (d *dashboard) pairing(w http.ResponseWriter, r *http.Request) {
	addrs := netinfo.Candidates()
	ip := r.URL.Query().Get("ip")
	valid := false
	for _, a := range addrs {
		valid = valid || a.IP == ip
	}
	if !valid {
		ip = ""
		if len(addrs) > 0 {
			ip = addrs[0].IP
		}
	}

	d.mu.Lock()
	key := base64.RawURLEncoding.EncodeToString(d.key)
	d.mu.Unlock()

	resp := map[string]any{"addrs": addrs, "port": d.phonePort, "ip": ip}
	if ip != "" {
		// The key travels in the URL fragment, which browsers never send over
		// the network: the phone reads it locally and keeps it.
		link := fmt.Sprintf("http://%s:%d/#k=%s", ip, d.phonePort, key)
		resp["url"] = link
		if svg, err := qrSVG(link); err == nil {
			resp["qr"] = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
		}
	}
	writeJSON(w, resp)
}

// qrSVG renders a crisp, resolution-independent QR code.
func qrSVG(text string) (string, error) {
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return "", err
	}
	q.DisableBorder = true
	bits := q.Bitmap()
	n := len(bits)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-2 -2 %d %d" shape-rendering="crispEdges"><rect x="-2" y="-2" width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n+4, n+4, n+4, n+4)
	for y, row := range bits {
		for x := 0; x < n; {
			if !row[x] {
				x++
				continue
			}
			start := x
			for x < n && row[x] {
				x++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", start, y, x-start, x-start)
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}

func (d *dashboard) resetPairing(w http.ResponseWriter, r *http.Request) {
	k := config.NewKey()
	if err := config.SaveKey(k); err != nil {
		log.Printf("save key: %v", err)
	}
	d.mu.Lock()
	d.key = k
	d.mu.Unlock()
	d.srv.SetKey(k)
	log.Print("pairing key rotated")
	w.WriteHeader(http.StatusNoContent)
}

func (d *dashboard) pause(w http.ResponseWriter, r *http.Request) {
	var body struct{ Paused bool }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d.srv.SetPaused(body.Paused)
	w.WriteHeader(http.StatusNoContent)
}

func (d *dashboard) autostart(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct{ Enabled bool }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := setAutostart(body.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]bool{"enabled": autostartEnabled()})
}

// events streams Status as Server-Sent Events: pushed on change instead of
// polled, and capped at ~30 updates/s so typing bursts stay cheap.
func (d *dashboard) events(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	ch, cancel := d.srv.Subscribe()
	defer cancel()

	send := func() error {
		b, _ := json.Marshal(d.srv.Status())
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		return rc.Flush()
	}
	if send() != nil {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			if send() != nil {
				return
			}
			time.Sleep(33 * time.Millisecond)
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

// settingsAPI reads or patches host settings. POST bodies may carry any
// subset of fields.
func (d *dashboard) settingsAPI(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.Method == http.MethodPost {
		var patch struct {
			PeekAllowed *bool `json:"peekAllowed"`
			Onboarded   *bool `json:"onboarded"`
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if patch.PeekAllowed != nil {
			d.settings.PeekAllowed = *patch.PeekAllowed
			d.srv.SetPeekAllowed(*patch.PeekAllowed)
		}
		if patch.Onboarded != nil {
			d.settings.Onboarded = *patch.Onboarded
		}
		if err := config.SaveSettings(d.settings); err != nil {
			log.Printf("save settings: %v", err)
		}
	}
	writeJSON(w, d.settings)
}

func (d *dashboard) sleep(w http.ResponseWriter, r *http.Request) {
	var body struct{ Minutes int }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d.srv.SetSleepTimer(time.Duration(min(max(body.Minutes, 0), 720)) * time.Minute)
	w.WriteHeader(http.StatusNoContent)
}
