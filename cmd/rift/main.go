// Command rift turns a phone into a wireless keyboard and trackpad for this PC.
//
// Two HTTP servers run side by side:
//   - the phone server on the LAN (ports 8080-8089): serves the phone app and
//     the end-to-end encrypted input socket;
//   - the dashboard on 127.0.0.1:8081: pairing QR, live status and controls,
//     reachable only from this machine and only with a per-launch admin key.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HarshalPatel1972/rift/internal/config"
	"github.com/HarshalPatel1972/rift/internal/injector"
	"github.com/HarshalPatel1972/rift/internal/power"
	"github.com/HarshalPatel1972/rift/internal/screen"
	"github.com/HarshalPatel1972/rift/internal/server"
	"github.com/HarshalPatel1972/rift/web"
)

const (
	dashboardAddr  = "127.0.0.1:8081"
	firstPhonePort = 8080
	phonePortTries = 10
)

func main() {
	background := flag.Bool("background", false, "start in the tray without opening the window")
	flag.Parse()

	setupLogging()

	dashLn, err := net.Listen("tcp4", dashboardAddr)
	if err != nil {
		// Already running: bring the existing window forward instead.
		if inst, ierr := config.ReadInstance(); ierr == nil && inst.DashboardURL != "" {
			openWindow(inst.DashboardURL)
			return
		}
		fatal("RIFT could not start: port 8081 is in use by another program.\n\n" + err.Error())
	}

	key, err := config.LoadOrCreateKey()
	if err != nil {
		log.Printf("pairing key not persisted: %v", err)
		key = config.NewKey()
	}

	phoneLn, err := listenPhone()
	if err != nil {
		fatal("RIFT could not open a network port for your phone (8080-8089 are all busy).\n\n" + err.Error())
	}
	phonePort := phoneLn.Addr().(*net.TCPAddr).Port

	hostName, _ := os.Hostname()
	hostName = strings.TrimSuffix(hostName, ".local") // macOS Bonjour names
	srv := server.New(server.Host{
		Input:       injector.New(),
		Screen:      screen.New,
		Power:       power.New(),
		Mac:         isMac,
		BlockedHint: blockedHint,
	}, key, web.Phone, web.Icon, hostName)
	settings := config.LoadSettings()
	srv.SetPeekAllowed(settings.PeekAllowed)

	admin := make([]byte, 24)
	rand.Read(admin)
	adminKey := hex.EncodeToString(admin)
	dashURL := fmt.Sprintf("http://%s/#%s", dashboardAddr, adminKey)
	if err := config.WriteInstance(config.Instance{DashboardURL: dashURL}); err != nil {
		log.Printf("instance file: %v", err)
	}

	dash := newDashboard(srv, key, phonePort, adminKey, settings)

	phoneHTTP := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	dashHTTP := &http.Server{Handler: dash.handler(), ReadHeaderTimeout: 10 * time.Second}
	go serve(phoneHTTP, phoneLn)
	go serve(dashHTTP, dashLn)
	log.Printf("RIFT up: phone port %d, dashboard %s", phonePort, dashboardAddr)

	if !*background {
		go func() {
			time.Sleep(150 * time.Millisecond)
			openWindow(dashURL)
		}()
	}

	quit := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		phoneHTTP.Shutdown(ctx)
		dashHTTP.Shutdown(ctx)
	}
	dash.onQuit = quitTray
	runTray(srv, func() { openWindow(dashURL) }, quit) // blocks until Quit
}

func listenPhone() (net.Listener, error) {
	var err error
	for p := firstPhonePort; p < firstPhonePort+phonePortTries; p++ {
		if p == 8081 {
			continue // the dashboard's port
		}
		var ln net.Listener
		if ln, err = net.Listen("tcp4", fmt.Sprintf(":%d", p)); err == nil {
			return ln, nil
		}
	}
	return nil, err
}

func serve(s *http.Server, ln net.Listener) {
	if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("http: %v", err)
	}
}

// setupLogging sends logs to %APPDATA%\RIFT\rift.log (the GUI build has no
// console), rotating once it passes 1 MB.
func setupLogging() {
	p, err := config.LogPath()
	if err != nil {
		return
	}
	if fi, err := os.Stat(p); err == nil && fi.Size() > 1<<20 {
		os.Rename(p, p+".old")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
}

func fatal(msg string) {
	log.Print(msg)
	showError(msg)
	os.Exit(1)
}
