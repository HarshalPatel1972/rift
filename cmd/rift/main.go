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
	"time"

	"github.com/HarshalPatel1972/rift/internal/server"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

var (
	inputServer  *server.Server
	sessionToken string
	
	// Connection state
	connMu      sync.Mutex
	isConnected bool
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>RIFT</title>
<link rel="icon" type="image/png" href="/favicon">
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;700&display=swap" rel="stylesheet">
<style>
:root {
    --bg: #050505;
    --card-bg: rgba(12, 12, 12, 0.95);
    --border: rgba(255, 255, 255, 0.08);
    --accent-cyan: #00F0FF;
    --accent-green: #00FF9D;
    --accent-red: #FF3333;
    --text-main: #ECECEC;
    --text-muted: #888;
}

* { margin:0; padding:0; box-sizing:border-box; }

body {
    background: var(--bg);
    color: var(--text-main);
    font-family: 'Space Grotesk', sans-serif;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    position: relative;
}

canvas {
    position: absolute;
    top: 0; left: 0;
    width: 100%; height: 100%;
    z-index: 0;
}

/* --- Monolith Card --- */
.monolith {
    position: relative;
    z-index: 10;
    width: 100%;
    max-width: 400px;
    background: var(--card-bg);
    border: 1px solid var(--border);
    border-radius: 4px;
    box-shadow: 0 20px 50px rgba(0,0,0,0.9);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: all 0.5s cubic-bezier(0.23, 1, 0.32, 1);
}

.monolith::before {
    content: '';
    position: absolute;
    top: 0; left: 0; right: 0; height: 1px;
    background: linear-gradient(90deg, transparent, var(--accent-cyan), transparent);
    opacity: 0.5;
}

/* Header Section */
.header {
    padding: 32px 32px 24px;
    text-align: center;
    background: rgba(255,255,255,0.01);
    border-bottom: 1px solid var(--border);
}

h1 {
    font-size: 36px;
    font-weight: 700;
    letter-spacing: -1px;
    background: linear-gradient(to bottom, #fff, #888);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    margin-bottom: 4px;
}

.subtitle {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 2px;
    color: var(--text-muted);
}

/* Content Area */
.content {
    padding: 0;
    display: flex;
    flex-direction: column;
}

/* QR Scanning Bracket */
.qr-stage {
    height: 0;
    overflow: hidden;
    transition: height 0.6s cubic-bezier(0.4, 0, 0.2, 1);
    background: rgba(0,0,0,0.3);
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
}

.qr-stage.active {
    height: 380px;
    border-bottom: 1px solid var(--border);
}

.scanner-frame {
    width: 200px;
    height: 200px;
    margin: 30px auto 16px;
    position: relative;
    padding: 10px;
}

.scanner-frame img {
    width: 100%;
    height: 100%;
    display: block;
    border-radius: 2px;
    opacity: 0.9;
}

/* Corner Brackets */
.corner {
    position: absolute;
    width: 20px; height: 20px;
    border-color: var(--accent-cyan);
    border-style: solid;
    transition: all 0.3s;
}
.tl { top: 0; left: 0; border-width: 2px 0 0 2px; }
.tr { top: 0; right: 0; border-width: 2px 2px 0 0; }
.bl { bottom: 0; left: 0; border-width: 0 0 2px 2px; }
.br { bottom: 0; right: 0; border-width: 0 2px 2px 0; }

.qr-stage.active .corner {
    animation: scanPulse 2s infinite;
}

/* Copy Link Box */
.link-box {
    margin-bottom: 24px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: rgba(255,255,255,0.05);
    border: 1px solid rgba(255,255,255,0.1);
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.2s;
    max-width: 80%;
}
.link-box:hover {
    background: rgba(255,255,255,0.1);
    border-color: var(--accent-cyan);
}
.link-text {
    font-family: monospace;
    font-size: 10px;
    color: #aaa;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 180px;
}
.link-icon { font-size: 12px; opacity: 0.7; }

/* Status Row */
.status-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 24px;
}

.status-light {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #333;
    transition: all 0.3s;
}

.status-light.green {
    background: var(--accent-green);
    box-shadow: 0 0 10px var(--accent-green);
    animation: pulseGreen 2s infinite;
}
.status-light.red {
    background: var(--accent-red);
    box-shadow: 0 0 10px var(--accent-red);
    animation: pulseRed 2s infinite;
}

.status-text {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
    letter-spacing: 1px;
}

/* Glass-Gold Button */
.action-area { padding: 0; }

button {
    width: 100%;
    height: 64px;
    background: transparent;
    border: none;
    color: #fff;
    font-family: 'Space Grotesk', sans-serif;
    font-size: 14px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
    cursor: pointer;
    position: relative;
    transition: all 0.3s;
    overflow: hidden;
}

button:hover {
    background: rgba(255,255,255,0.03);
    color: var(--accent-cyan);
}

button::after {
    content: '';
    position: absolute;
    top: 0; left: -100%;
    width: 100%; height: 100%;
    background: linear-gradient(90deg, transparent, rgba(255,255,255,0.1), transparent);
    animation: shimmer 3s infinite;
}

/* Animations */
@keyframes shimmer {
    0% { left: -100%; }
    20% { left: 100%; }
    100% { left: 100%; }
}

@keyframes scanPulse {
    0% { border-color: var(--accent-cyan); box-shadow: 0 0 0 var(--accent-cyan); }
    50% { border-color: #fff; box-shadow: 0 0 10px var(--accent-cyan); }
    100% { border-color: var(--accent-cyan); box-shadow: 0 0 0 var(--accent-cyan); }
}

@keyframes pulseGreen {
    0% { opacity: 0.6; } 50% { opacity: 1; box-shadow: 0 0 15px var(--accent-green); } 100% { opacity: 0.6; }
}
@keyframes pulseRed {
    0% { opacity: 0.6; } 50% { opacity: 1; box-shadow: 0 0 15px var(--accent-red); } 100% { opacity: 0.6; }
}

</style>
</head>
<body>

<canvas id="nerve-canvas"></canvas>

<div class="monolith">
    <div class="header">
        <h1>RIFT</h1>
        <div class="subtitle">Air Typing Host</div>
    </div>

    <div class="content">
        <!-- QR Stage (Hidden by default) -->
        <div id="qr-stage" class="qr-stage">
            <div class="scanner-frame">
                <div class="corner tl"></div><div class="corner tr"></div>
                <div class="corner bl"></div><div class="corner br"></div>
                <img id="qr" alt="Data Link">
            </div>

            <!-- Manually Copy Link -->
            <div class="link-box" onclick="copyLink()">
                <span class="link-text" id="link-text"></span>
                <span class="link-icon">📋</span>
                <span id="copy-feedback" style="display:none;font-size:10px;color:var(--accent-green)">COPIED!</span>
            </div>

            <div class="status-row">
                <div id="status-light" class="status-light"></div>
                <div id="status-text" class="status-text">INITIALIZING...</div>
            </div>
        </div>

        <!-- Action Button -->
        <div class="action-area">
            <button id="main-btn" onclick="init()">Connect Device</button>
        </div>
    </div>
</div>

<script>
// --- ENGINE: THE DIGITAL NERVE ---
const canvas = document.getElementById('nerve-canvas');
const ctx = canvas.getContext('2d');
let w, h;
let strands = [];
let packets = [];
let connected = false;

function resize() {
    w = window.innerWidth;
    h = window.innerHeight;
    canvas.width = w;
    canvas.height = h;
    initStrands();
}
window.addEventListener('resize', resize);

class Strand {
    constructor(y) {
        this.y = y;
        this.phase = Math.random() * Math.PI * 2;
        this.speed = 0.0005 + Math.random() * 0.001;
        this.amplitude = 30 + Math.random() * 50;
    }
    
    getY(x) {
        return this.y + 
               Math.sin(x * 0.002 + this.phase) * this.amplitude + 
               Math.sin(x * 0.005 - this.phase) * (this.amplitude * 0.5);
    }

    update() {
        this.phase += connected ? this.speed * 4 : this.speed;
    }
}

class Packet {
    constructor(strand) {
        this.strand = strand;
        this.x = Math.random() * w;
        this.size = Math.random() * 2 + 1;
        this.speed = 2 + Math.random() * 3;
        this.color = Math.random() > 0.5 ? '#00F0FF' : '#BD00FF'; 
    }

    update() {
        this.x += connected ? this.speed * 2.5 : this.speed;
        if(this.x > w) {
            this.x = -10;
            this.strand = strands[Math.floor(Math.random() * strands.length)];
        }
    }

    draw() {
        const y = this.strand.getY(this.x);
        ctx.shadowBlur = 15;
        ctx.shadowColor = this.color;
        ctx.fillStyle = "#fff";
        ctx.beginPath();
        ctx.arc(this.x, y, this.size, 0, Math.PI*2);
        ctx.fill();
        ctx.shadowBlur = 0;
    }
}

function initStrands() {
    strands = [];
    const count = 5;
    const spacing = h / count;
    for(let i=0; i<count + 2; i++) {
        strands.push(new Strand(i * spacing));
    }
    packets = [];
    for(let i=0; i<20; i++) {
        packets.push(new Packet(strands[i % strands.length]));
    }
}

function animate() {
    ctx.fillStyle = "#050505";
    ctx.fillRect(0, 0, w, h);

    ctx.lineWidth = 1;
    strands.forEach(s => {
        s.update();
        ctx.beginPath();
        ctx.strokeStyle = "rgba(100, 100, 100, 0.2)";
        for(let x=0; x<w; x+=10) {
            const y = s.getY(x);
            if(x==0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        }
        ctx.stroke();
    });

    packets.forEach(p => {
        p.update();
        p.draw();
    });

    requestAnimationFrame(animate);
}

resize();
animate();

// --- LOGIC ---
let linkUrl = "";
let pollInterval;

async function init() {
    const btn = document.getElementById('main-btn');
    const stage = document.getElementById('qr-stage');
    const textEl = document.getElementById('status-text');
    const lightEl = document.getElementById('status-light');
    const linkText = document.getElementById('link-text');
    
    if(pollInterval) clearInterval(pollInterval);

    btn.innerHTML = "ESTABLISHING UPLINK...";
    btn.disabled = true;

    try {
        const res = await fetch('/start');
        const data = await res.json();
        
        document.getElementById('qr').src = data.qr;
        linkUrl = data.url;
        linkText.innerHTML = linkUrl;
        
        btn.innerHTML = "REGENERATE LINK";
        btn.disabled = false;
        stage.classList.add('active');
        
        if (data.isLoopback) {
            textEl.innerHTML = "OFFLINE MODE";
            lightEl.className = "status-light red";
        } else {
            textEl.innerHTML = "READY TO CONNECT";
            lightEl.className = "status-light green";
        }

        pollInterval = setInterval(async () => {
            try {
                const sRes = await fetch('/status');
                const sData = await sRes.json();
                
                if(sData.connected) {
                    if(!connected) {
                        connected = true;
                        textEl.innerHTML = "DEVICE CONNECTED";
                        lightEl.className = "status-light green";
                    }
                } else {
                    if(connected) {
                        connected = false;
                        textEl.innerHTML = "READY TO CONNECT";
                        lightEl.className = "status-light green";
                    }
                }
            } catch(e) {
                console.error("Poll failed", e);
            }
        }, 1000);

    } catch(e) {
        btn.innerHTML = "CONNECTION FAILED";
        btn.disabled = false;
        alert(e);
    }
}

async function copyLink() {
    if(!linkUrl) return;
    try {
        await navigator.clipboard.writeText(linkUrl);
        const fb = document.getElementById('copy-feedback');
        fb.style.display = 'inline';
        setTimeout(() => fb.style.display = 'none', 2000);
    } catch(e) {
        console.error(e);
    }
}
// Clean shutdown when user closes window
window.addEventListener('beforeunload', () => {
    navigator.sendBeacon('/shutdown');
});
</script>
</body>
</html>`

func main() {
	sessionToken = uuid.New().String()

	inputServer = server.New(":8080", sessionToken)
	
	inputServer.OnConnect = func() {
		connMu.Lock()
		isConnected = true
		connMu.Unlock()
		log.Println("Client Connected")
	}
	inputServer.OnDisconnect = func() {
		connMu.Lock()
		isConnected = false
		connMu.Unlock()
		log.Println("Client Disconnected")
	}

	go func() {
		if err := inputServer.Start(); err != nil {
			log.Printf("Input server error: %v", err)
		}
	}()

	http.HandleFunc("/", serveDashboard)
	http.HandleFunc("/start", handleStart)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/shutdown", handleShutdown)
	http.HandleFunc("/favicon", serveFavicon)

	// Auto-open in App Mode
	go func() {
		time.Sleep(500 * time.Millisecond)
		launchAppMode("http://localhost:8081")
	}()

	fmt.Println("RIFT | Air Typing Host @ http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

// launchAppMode opens the URL in Chrome/Edge app mode (no URL bar, looks native)
func launchAppMode(url string) {
	// Try Edge first (built-in on Windows 10/11)
	if path := findBrowser("msedge.exe"); path != "" {
		exec.Command(path, "--app="+url, "--window-size=600,600").Start()
		return
	}
	// Try Chrome
	if path := findBrowser("chrome.exe"); path != "" {
		exec.Command(path, "--app="+url, "--window-size=600,600").Start()
		return
	}
	// Fallback to default browser (no app mode)
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// findBrowser locates Chrome or Edge executable
func findBrowser(name string) string {
	// Try system PATH first
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	// Check common installation paths
	commonPaths := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

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

func handleStart(w http.ResponseWriter, r *http.Request) {
	ip := GetOutboundIP()
	
	isLoopback := false
	if ip == "localhost" || ip == "127.0.0.1" || ip == "::1" {
		isLoopback = true
	}

	connectionURL := fmt.Sprintf("http://%s:8080?token=%s", ip, sessionToken)

	qrBytes, err := qrcode.Encode(connectionURL, qrcode.Medium, 280)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrBytes)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"qr":"%s","url":"%s","ip":"%s","isLoopback":%t}`, qrDataURI, connectionURL, ip, isLoopback)
}

func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return GetLocalIP()
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
}

// serveFavicon serves the icon.png file
func serveFavicon(w http.ResponseWriter, r *http.Request) {
	iconData, err := os.ReadFile("web/icon.png")
	if err != nil {
		http.Error(w, "Icon not found", 404)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(iconData)
}

// handleShutdown receives the signal from JS when window closes
func handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	log.Println("UI closed, shutting down RIFT...")
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}
