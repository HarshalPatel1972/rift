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
  --error: #FF3333;
  --warning: #FFCC00;
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
}
.card {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 520px;
  background: rgba(10, 10, 10, 0.9);
  backdrop-filter: blur(16px);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 48px;
  box-shadow: 0 4px 40px rgba(0,0,0,0.6);
  text-align: center;
  opacity: 0; /* Handled by anime.js */
}
.card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 16px;
  padding: 1px;
  background: linear-gradient(180deg, rgba(255,255,255,0.15), transparent);
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
  display: block; /* Setup for animation */
  margin: 32px 0;
  opacity: 0;
}
.qr-frame {
  background: white;
  padding: 16px;
  border-radius: 12px;
  display: inline-block;
  box-shadow: 0 0 20px rgba(255,255,255,0.1);
  transform: scale(0.9);
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
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-family: 'Fira Code', monospace;
  font-size: 12px;
  color: #555;
  background: rgba(0,0,0,0.3);
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.2s;
  opacity: 0;
}
.url-c:hover {
  background: rgba(255,255,255,0.05);
  color: #888;
}
.copy-icon { opacity: 0.6; font-size: 14px; }
.warning-box {
    display: none;
    margin-top: 16px;
    padding: 12px;
    background: rgba(255, 204, 0, 0.1);
    border: 1px solid rgba(255, 204, 0, 0.2);
    border-radius: 8px;
    color: var(--warning);
    font-size: 13px;
    text-align: center;
}
</style>
</head>
<body>
  <canvas id="bg-canvas"></canvas>
  <div class="card" id="main-card">
    <h1>RIFT</h1>
    <div class="subtitle">Air Typing Bridge</div>

    <div id="qr-area" class="qr-container">
      <div class="qr-frame" id="qr-frame">
        <img id="qr" width="240" height="240" alt="QR" />
      </div>
      <div class="meta">
        <span class="dot" id="status-dot"></span>
        <span id="ip-display"></span>
      </div>
    </div>

    <div id="network-warning" class="warning-box">
        ⚠️ <strong>Offline Mode</strong><br/>
        Connect to Wi-Fi to use Air Typing from other devices.
    </div>

    <button id="btn-init" onclick="init()">Connect Device</button>
    <button id="btn-refresh" onclick="init()">Generate New Link</button>

    <div class="status" id="status">Secure P2P Connection</div>
    
    <div class="url-c" id="url-container" onclick="copyLink()" title="Click to Copy">
        <span id="url-text"></span>
        <span class="copy-icon">📋</span>
    </div>
  </div>

<script>
// --- Anime.js Classy Network Animation ---
const canvas = document.getElementById('bg-canvas');
const ctx = canvas.getContext('2d');
let w, h;
let particles = [];

function resize() {
  w = window.innerWidth;
  h = window.innerHeight;
  canvas.width = w;
  canvas.height = h;
}
window.addEventListener('resize', resize);
resize();

// Particle System
const bgTimeline = anime.timeline({
    loop: true,
    easing: 'linear'
});

function createParticles() {
    particles = [];
    const count = Math.min((w * h) / 15000, 100); // Responsive density
    
    for(let i=0; i<count; i++) {
        particles.push({
            x: Math.random() * w,
            y: Math.random() * h,
            r: Math.random() * 2 + 1,
            alpha: Math.random() * 0.3 + 0.1,
            vx: (Math.random() - 0.5) * 0.5, // Slow float
            vy: (Math.random() - 0.5) * 0.5
        });
    }
}
createParticles();

// Render Loop
function render() {
    ctx.clearRect(0,0,w,h);
    
    // Draw connections
    ctx.lineWidth = 1;
    for(let i=0; i<particles.length; i++) {
        let p1 = particles[i];
        
        // Update position
        p1.x += p1.vx;
        p1.y += p1.vy;
        
        // Bounce off edges
        if(p1.x < 0 || p1.x > w) p1.vx *= -1;
        if(p1.y < 0 || p1.y > h) p1.vy *= -1;
        
        // Draw Dot
        ctx.beginPath();
        ctx.arc(p1.x, p1.y, p1.r, 0, Math.PI*2);
        ctx.fillStyle = 'rgba(255,255,255,' + p1.alpha + ')';
        ctx.fill();
        
        // Connect to neighbors
        for(let j=i+1; j<particles.length; j++) {
            let p2 = particles[j];
            let dist = Math.hypot(p1.x-p2.x, p1.y-p2.y);
            
            if(dist < 120) {
                ctx.beginPath();
                ctx.moveTo(p1.x, p1.y);
                ctx.lineTo(p2.x, p2.y);
                let alpha = (1 - dist/120) * 0.15; // Fade out by distance
                ctx.strokeStyle = 'rgba(255,255,255,' + alpha + ')';
                ctx.stroke();
            }
        }
    }
    requestAnimationFrame(render);
}
render();

// --- UI Animations (Anime.js) ---

// Entrance Animation
anime.timeline()
.add({
    targets: '#main-card',
    opacity: [0, 1],
    translateY: [40, 0],
    scale: [0.96, 1],
    duration: 1200,
    easing: 'cubicBezier(0.16, 1, 0.3, 1)'
});

// Pulse Status Dot
anime({
    targets: '#status-dot',
    scale: [1, 1.4],
    opacity: [0.8, 0.4],
    easing: 'easeInOutSine',
    duration: 1500,
    direction: 'alternate',
    loop: true
});

// --- Logic ---
let currentURL = "";

async function copyLink() {
    if(!currentURL) return;
    try {
        await navigator.clipboard.writeText(currentURL);
        const st = document.getElementById('status');
        const original = st.textContent;
        st.textContent = "✅ Link Copied!";
        st.style.color = "#00AA55";
        
        anime({
            targets: st,
            translateY: [-2, 0],
            duration: 300,
            easing: 'easeOutElastic(1, .8)'
        });
        
        setTimeout(() => {
            st.textContent = original;
            st.style.color = "#666";
        }, 2000);
    } catch(e) {
        alert("Could not copy link: " + currentURL);
    }
}

async function init() {
  const btn = document.getElementById('btn-init');
  const ref = document.getElementById('btn-refresh');
  const st = document.getElementById('status');
  const qrArea = document.getElementById('qr-area');
  const qrFrame = document.getElementById('qr-frame');
  const warning = document.getElementById('network-warning');
  const urlContainer = document.getElementById('url-container');
  
  btn.disabled = true;
  ref.disabled = true;
  st.textContent = "Negotiating...";
  warning.style.display = 'none';
  
  // Button Click Effect
  anime({
      targets: btn,
      scale: [1, 0.96],
      duration: 100,
      direction: 'alternate',
      easing: 'easeInOutQuad'
  });
  
  try {
    const res = await fetch('/start');
    const data = await res.json();
    
    document.getElementById('qr').src = data.qr;
    document.getElementById('ip-display').textContent = data.ip;
    document.getElementById('url-text').textContent = data.url;
    currentURL = data.url;
    
    if (data.isLoopback) {
        warning.style.display = 'block';
        st.textContent = "⚠️ Offline Mode";
    } else {
        st.textContent = "Signal Active";
    }

    // New Reveal Animation
    btn.style.display = 'none';
    ref.style.display = 'block';
    ref.disabled = false;
    urlContainer.style.display = 'flex';
    
    anime.timeline()
    .add({
        targets: qrArea,
        opacity: [0, 1],
        duration: 800,
        easing: 'easeOutQuad'
    })
    .add({
        targets: qrFrame,
        scale: [0.8, 1],
        rotate: [-2, 0],
        duration: 1000,
        easing: 'easeOutElastic(1, .8)'
    }, '-=600')
    .add({
        targets: urlContainer,
        opacity: [0, 1],
        translateY: [10, 0],
        duration: 600,
        easing: 'easeOutQuad'
    }, '-=800');
    
  } catch(e) {
    st.textContent = "Failed: " + e.message;
    btn.disabled = false;
    ref.disabled = false;
  }
}
// Init hidden
document.getElementById('url-container').style.display = 'none';
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

	fmt.Println("RIFT Dashboard (Combined Updates) starting at http://localhost:8081")
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
	
	// Check for loopback
	isLoopback := false
	if ip == "localhost" || ip == "127.0.0.1" || ip == "::1" {
		isLoopback = true
	}

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
	// Use manual JSON construction for simplicity in main.go main pkg
	fmt.Fprintf(w, `{"qr":"%s","url":"%s","ip":"%s","isLoopback":%t}`, qrDataURI, connectionURL, ip, isLoopback)
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
