package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/HarshalPatel1972/rift/internal/server"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

var (
	inputServer  *server.Server
	sessionToken string
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>RIFT</title>
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⚡</text></svg>">
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<style>
:root {
  --bg: #030303;
  --card-bg: #0a0a0a;
  --border: #222;
  --text-main: #ededed;
  --text-muted: #888;
  --accent: #fff;
  --success: #00AA55;
}
* { margin:0; padding:0; box-sizing:border-box; }
body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
  background: var(--bg);
  color: var(--text-main);
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  -webkit-font-smoothing: antialiased;
}
.card {
  width: 100%;
  max-width: 400px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 40px;
  box-shadow: 0 4px 24px rgba(0,0,0,0.4);
  text-align: center;
  position: relative;
  overflow: hidden;
}
/* Subtle top glow */
.card::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, #333, transparent);
}
h1 {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 8px;
  letter-spacing: -0.5px;
}
.subtitle {
  color: var(--text-muted);
  font-size: 14px;
  margin-bottom: 32px;
}
.qr-container {
  display: none;
  margin: 24px 0;
  animation: fadeScale 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}
.qr-container.visible { display: block; }
.qr-frame {
  background: white;
  padding: 12px;
  border-radius: 8px;
  display: inline-block;
}
.qr-frame img { display: block; border-radius: 4px; }
.meta {
  margin-top: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-muted);
}
.dot {
  width: 6px; height: 6px;
  background: var(--success);
  border-radius: 50%;
  box-shadow: 0 0 8px rgba(0, 170, 85, 0.4);
}
button {
  width: 100%;
  height: 48px;
  background: var(--accent);
  color: black;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}
button:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}
button:disabled {
  opacity: 0.5;
  cursor: wait;
  transform: none;
}
#btn-refresh {
  background: #222;
  color: white;
  margin-top: 12px;
  display: none;
}
#btn-refresh:hover { background: #333; }
.status {
  margin-top: 24px;
  font-size: 12px;
  color: #444;
}
.url-c {
  margin-top: 12px;
  font-family: monospace;
  font-size: 11px;
  color: #555;
  word-break: break-all;
}
@keyframes fadeScale {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
</style>
</head>
<body>
  <div class="card">
    <h1>RIFT</h1>
    <div class="subtitle">Air Typing Bridge</div>

    <div id="qr-area" class="qr-container">
      <div class="qr-frame">
        <img id="qr" width="220" height="220" alt="QR" />
      </div>
      <div class="meta">
        <span class="dot"></span>
        <span id="ip-display"></span>
      </div>
    </div>

    <button id="btn-init" onclick="init()">Connect</button>
    <button id="btn-refresh" onclick="init()">Regenerate Link</button>

    <div class="status" id="status">Ready</div>
    <div class="url-c" id="url-text"></div>
  </div>

<script>
async function init() {
  const btn = document.getElementById('btn-init');
  const ref = document.getElementById('btn-refresh');
  const st = document.getElementById('status');
  const qrArea = document.getElementById('qr-area');
  
  btn.disabled = true;
  ref.disabled = true;
  st.textContent = "Connecting...";
  
  try {
    const res = await fetch('/start');
    const data = await res.json();
    
    document.getElementById('qr').src = data.qr;
    document.getElementById('ip-display').textContent = data.ip;
    document.getElementById('url-text').textContent = data.url;
    
    qrArea.classList.add('visible');
    
    btn.style.display = 'none';
    ref.style.display = 'block';
    ref.disabled = false;
    st.textContent = "Connection Active";
  } catch(e) {
    st.textContent = "Failed to connect";
    btn.disabled = false;
    ref.disabled = false;
  }
}
</script>
</body>
</html>`

func main() {
	// Generate session token
	sessionToken = uuid.New().String()

	// Start input server on port 8080 in background
	inputServer = server.New(":8080", sessionToken)
	go func() {
		log.Println("Starting input server on port 8080...")
		if err := inputServer.Start(); err != nil {
			log.Fatalf("Input server failed: %v", err)
		}
	}()

	// Setup HTTP dashboard server on port 8081
	http.HandleFunc("/", serveDashboard)
	http.HandleFunc("/start", handleStart)

	// Auto-open browser
	go func() {
		time.Sleep(500 * time.Millisecond)
		exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost:8081").Start()
	}()

	fmt.Println("RIFT Dashboard starting at http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	// Get current outbound IP (dynamic)
	ip := GetOutboundIP()
	port := "8080"

	connectionURL := fmt.Sprintf("http://%s:%s?token=%s", ip, port, sessionToken)

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
	fmt.Fprintf(w, `{"qr":"%s","url":"%s","ip":"%s"}`, qrDataURI, connectionURL, ip)
}

// GetOutboundIP uses UDP dial trick to find the preferred outbound IP
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback to interface enumeration
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
