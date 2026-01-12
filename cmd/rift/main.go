package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/HarshalPatel1972/rift/internal/server"
	"golang.org/x/sys/windows"

var (
	inputServer  *server.Server
	sessionToken string

	// Connection state
	connMu      sync.Mutex
	isConnected bool

	// GUI
	ni       *walk.NotifyIcon
	mw       *walk.MainWindow
	icon     *walk.Icon
	exitCh   = make(chan struct{})

	// Runtime
	assignedPort = 0 // 0 means dynamic, or set a start port
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<!-- ... (HTML Content remains same) ... -->
</html>`

func main() {
	// 0. Single Instance Check
	if err := acquireGlobalLock("Global\\RIFT_App_Mutex_v1"); err != nil {
		showErrorBox("RIFT is Already Running", "Check your System Tray (bottom right).\nRIFT runs in the background.")
		os.Exit(0)
	}

	// 0. Error Handling Wrapper
	defer func() {
		if r := recover(); r != nil {
			showErrorBox("Critical Error", fmt.Sprint(r))
			os.Exit(1)
		}
	}()

	// Generate session token
	sessionToken = uuid.New().String()

	// 1. Start Input Server (UDP/WebSocket Logic)
	// Try ports 8080-8090 for Input Server
	inputPort := findAvailablePort(8080)
	inputServer = server.New(fmt.Sprintf(":%d", inputPort), sessionToken)
	
	inputServer.OnConnect = func() {
		connMu.Lock()
		isConnected = true
		connMu.Unlock()
		updateTrayTooltip("RIFT: Connected")
		log.Println("State update: Client Connected")
	}
	inputServer.OnDisconnect = func() {
		connMu.Lock()
		isConnected = false
		connMu.Unlock()
		updateTrayTooltip("RIFT: Waiting...")
		log.Println("State update: Client Disconnected")
	}

	go func() {
		if err := inputServer.Start(); err != nil {
			log.Printf("Input server warning: %v", err)
		}
	}()

	// 2. Start HTTP Dashboard Server
	// Try ports 8081-8091 for Dashboard
	dashPort := findAvailablePort(8081)
	if dashPort == inputPort { dashPort++ } // Avoid conflict

	assignedPort = dashPort

	http.HandleFunc("/", serveDashboard)
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		handleStart(w, r, inputPort)
	})
	http.HandleFunc("/status", handleStatus)

	go func() {
		addr := fmt.Sprintf(":%d", dashPort)
		if err := http.ListenAndServe(addr, nil); err != nil {
			showErrorBox("Startup Error", fmt.Sprintf("Failed to start Web Server on %s: %v", addr, err))
			os.Exit(1)
		}
	}()

	// 3. Start System Tray (Main Thread)
	startSystemTray()
}

func acquireGlobalLock(name string) error {
	namePtr, _ := windows.UTF16PtrFromString(name)
	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		return err // Should not happen usually
	}
	if handle == 0 {
		return fmt.Errorf("failed to create mutex")
	}
	// Check if already exists
	if err := windows.GetLastError(); err == windows.ERROR_ALREADY_EXISTS {
		return fmt.Errorf("instance already running")
	}
	return nil
}

func findAvailablePort(start int) int {
	for port := start; port < start+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return start // Fallback, will likely fail later if heavily used
}


func showErrorBox(title, msg string) {
	walk.MsgBox(nil, title, msg, walk.MsgBoxIconError)
}

func startSystemTray() {
	// We need a specific manifest for walk to work properly on Windows
	mw = new(walk.MainWindow)

	if err := (MainWindow{
		AssignTo: &mw,
		Title:    "RIFT Background Host",
		Visible:  false, // start hidden
	}.Create()); err != nil {
		showErrorBox("Initialization Failed", "Could not create Main Window. Ensure Common Controls 6.0 is available.\nError: "+err.Error())
		os.Exit(1)
	}

	// Create Tray Icon
	var err error
	ni, err = walk.NewNotifyIcon(mw)
	if err != nil {
		showErrorBox("Tray Error", err.Error())
		os.Exit(1)
	}
	defer ni.Dispose()

	// Load Icon (Standard Application Icon or specific .ico if file exists)
	// For now, use standard info icon if customized one not found
	if icon, err = walk.NewIconFromFile("app.ico"); err != nil {
		// Fallback to generic system icon if file missing
		icon = walk.IconApplication()
	}
	ni.SetIcon(icon)
	ni.SetToolTip("RIFT: Active")
	ni.SetVisible(true)

	// Context Menu
	detailsAction := walk.NewAction()
	detailsAction.SetText("Open Dashboard")
	detailsAction.Triggered().Attach(func() {
		openBrowser()
	})
	ni.ContextMenu().Actions().Add(detailsAction)
	
	ni.ContextMenu().Actions().Add(walk.NewSeparatorAction())

	updateAction := walk.NewAction()
	updateAction.SetText("Check for Updates")
	updateAction.Triggered().Attach(func() {
		checkForUpdates()
	})
	ni.ContextMenu().Actions().Add(updateAction)

	exitAction := walk.NewAction()
	exitAction.SetText("Exit RIFT")
	exitAction.Triggered().Attach(func() {
		walk.App().Exit(0)
	})
	ni.ContextMenu().Actions().Add(exitAction)

	// Click on icon -> Open Dashboard
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			openBrowser()
		}
	})

	// Show initial notification
	ni.ShowCustom("RIFT is Running", "Air Typing Host is active in the background.", icon)

	// Launch browser on start
	openBrowser()

	// Run message loop
	mw.Run()
}

func updateTrayTooltip(msg string) {
	if ni != nil {
		ni.SetToolTip(msg)
	}
}

func openBrowser() {
	if assignedPort == 0 {
		return // Should not happen if started correct
	}
	url := fmt.Sprintf("http://localhost:%d", assignedPort)
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func checkForUpdates() {
	repoURL := "https://github.com/HarshalPatel1972/rift/releases/latest"
	exec.Command("rundll32", "url.dll,FileProtocolHandler", repoURL).Start()
}


// ... (Existing Handlers: serveDashboard, handleStatus, IP helpers) ...

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write([]byte(dashboardHTML))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	connMu.Lock()
	status := isConnected
	connMu.Unlock()
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, `{"connected":%t}`, status)
}

func handleStart(w http.ResponseWriter, r *http.Request, inputPort int) {
	// Get current outbound IP (dynamic)
	ip := GetOutboundIP()
	
	// Check for Loopback
	isLoopback := false
	if ip == "localhost" || ip == "127.0.0.1" || ip == "::1" {
		isLoopback = true
	}

	connectionURL := fmt.Sprintf("http://%s:%d?token=%s", ip, inputPort, sessionToken)

	// Generate QR code with current IP
	qrBytes, err := qrcode.Encode(connectionURL, qrcode.Medium, 280)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Convert to data URI
	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrBytes)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"qr":"%s","url":"%s","ip":"%s","isLoopback":%t}`, qrDataURI, connectionURL, ip, isLoopback)
}

// GetOutboundIP uses UDP dial trick to find the preferred outbound IP
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return GetLocalIP()
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// GetLocalIP is the fallback method
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
}

