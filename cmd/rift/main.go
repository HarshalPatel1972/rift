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
<script src="https://cdnjs.cloudflare.com/ajax/libs/animejs/3.2.1/anime.min.js"></script>
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
  overflow: hidden;
  position: relative;
}
canvas {
  position: absolute;
  top: 0; left: 0;
  width: 100%; height: 100%;
  z-index: 0;
  opacity: 0.4;
}
.card {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 520px; /* Made larger */
  background: rgba(10, 10, 10, 0.85); /* Slight transparency */
  backdrop-filter: blur(12px);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 48px;
  box-shadow: 0 4px 40px rgba(0,0,0,0.6);
  text-align: center;
}
.card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 16px;
  padding: 1px;
  background: linear-gradient(180deg, rgba(255,255,255,0.1), transparent);
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  pointer-events: none;
}
h1 {
  font-size: 32px;
  font-weight: 700;
  margin-bottom: 8px;
  letter-spacing: -1px;
}
.subtitle {
  color: var(--text-muted);
  font-size: 15px;
  margin-bottom: 32px;
}
.qr-container {
  display: none;
  margin: 32px 0;
  animation: fadeScale 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}
.qr-container.visible { display: block; }
.qr-frame {
  background: white;
  padding: 16px;
  border-radius: 12px;
  display: inline-block;
  box-shadow: 0 0 20px rgba(255,255,255,0.1);
}
.qr-frame img { display: block; border-radius: 6px; }
.meta {
  margin-top: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 14px;
  color: var(--text-muted);
}
.dot {
  width: 8px; height: 8px;
  background: var(--success);
  border-radius: 50%;
  box-shadow: 0 0 10px rgba(0, 170, 85, 0.6);
  animation: pulse 2s infinite;
}
button {
  width: 100%;
  height: 56px;
  background: var(--accent);
  color: black;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  margin-top: 8px;
}
button:hover { opacity: 0.9; transform: scale(1.02); }
button:disabled { opacity: 0.5; cursor: wait; transform: none; }
#btn-refresh {
  background: rgba(255,255,255,0.08);
  color: white;
  margin-top: 16px;
  display: none;
}
#btn-refresh:hover { background: rgba(255,255,255,0.12); }
.status {
  margin-top: 32px;
  font-size: 13px;
  color: #666;
  font-weight: 500;
}
.url-c {
  margin-top: 8px;
  font-family: 'Fira Code', monospace;
  font-size: 12px;
  color: #555;
  opacity: 0.6;
}
@keyframes pulse {
  0% { box-shadow: 0 0 0 0 rgba(0, 170, 85, 0.4); }
  70% { box-shadow: 0 0 0 10px rgba(0, 170, 85, 0); }
  100% { box-shadow: 0 0 0 0 rgba(0, 170, 85, 0); }
}
@keyframes fadeScale {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
</style>
</head>
<body>
  <canvas id="bg-canvas"></canvas>
  <div class="card">
    <h1>RIFT</h1>
    <div class="subtitle">Air Typing Bridge</div>

    <div id="qr-area" class="qr-container">
      <div class="qr-frame">
        <img id="qr" width="240" height="240" alt="QR" />
      </div>
      <div class="meta">
        <span class="dot"></span>
        <span id="ip-display"></span>
      </div>
    </div>

    <button id="btn-init" onclick="init()">Connect Device</button>
    <button id="btn-refresh" onclick="init()">Generate New Link</button>

    <div class="status" id="status">Secure P2P Connection</div>
    <div class="url-c" id="url-text"></div>
  </div>

<script>
// Vanilla JS Background Effect - "Data Rain / Signal Flow"
const canvas = document.getElementById('bg-canvas');
const ctx = canvas.getContext('2d');
let width, height;

function resize() {
  width = window.innerWidth;
  height = window.innerHeight;
  canvas.width = width;
  canvas.height = height;
}
window.addEventListener('resize', resize);
resize();

// Create particles
const particles = [];
const particleCount = 80; // More particles

class Particle {
  constructor() {
    this.reset();
  }
  
  reset() {
    this.x = Math.random() * width;
    this.y = Math.random() * height + height; // Start below
    this.size = Math.random() * 2 + 1;
    this.speed = Math.random() * 1.5 + 0.5;
    this.opacity = Math.random() * 0.5 + 0.1;
  }
  
  update() {
    this.y -= this.speed;
    if (this.y < -10) {
      this.reset();
    }
  }
  
  draw() {
    ctx.fillStyle = 'rgba(255, 255, 255, ' + (this.opacity * 0.1) + ')'; // Very subtle
    ctx.beginPath();
    ctx.arc(this.x, this.y, this.size, 0, Math.PI * 2);
    ctx.fill();
  }
}

for(let i=0; i<particleCount; i++) {
  particles.push(new Particle());
  // Pre-scatter
  particles[i].y = Math.random() * height;
}

function animate() {
  ctx.clearRect(0, 0, width, height);
  
  // Update and draw particles
  particles.forEach(p => {
    p.update();
    p.draw();
  });

  // Connecting lines
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.03)';
  ctx.beginPath();
  for(let i=0; i<particles.length; i++) {
    for(let j=i+1; j<particles.length; j++) {
      const dx = particles[i].x - particles[j].x;
      const dy = particles[i].y - particles[j].y;
      const dist = Math.sqrt(dx*dx + dy*dy);
      
      if(dist < 100) {
        ctx.moveTo(particles[i].x, particles[i].y);
        ctx.lineTo(particles[j].x, particles[j].y);
      }
    }
  }
  ctx.stroke();
  
  requestAnimationFrame(animate);
}
animate();

// Main Logic
async function init() {
  const btn = document.getElementById('btn-init');
  const ref = document.getElementById('btn-refresh');
  const st = document.getElementById('status');
  const qrArea = document.getElementById('qr-area');
  
  btn.disabled = true;
  ref.disabled = true;
  st.textContent = "Negotiating...";
  
  try {
    const res = await fetch('/start');
    const data = await res.json();
    
    document.getElementById('qr').src = data.qr;
    document.getElementById('ip-display').textContent = data.ip;
    document.getElementById('url-text').textContent = data.url;
    
    // Vanilla Fade In
    qrArea.style.display = 'block';
    // Use timeout to allow display:block to apply before opacity transition
    setTimeout(() => {
        qrArea.classList.add('visible');
    }, 10);
    
    btn.style.display = 'none';
    ref.style.display = 'block';
    ref.disabled = false;
    st.textContent = "Signal Active • Ready for Input";
    
  } catch(e) {
    st.textContent = "Link Failed: " + e.message;
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

	fmt.Println("RIFT Dashboard (Anime.js Update) starting at http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
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
