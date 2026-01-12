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
<link href="https://fonts.googleapis.com/css2?family=Playfair+Display:wght@700&family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{
font-family:'Manrope',sans-serif;
min-height:100vh;
display:flex;
align-items:center;
justify-content:center;
background:#020202;
background:radial-gradient(circle at 50% 50%,#111 0%,#020202 100%);
color:#E0E0E0;
}
.card{
max-width:460px;
width:90%;
background:#141414;
border:1px solid rgba(212,175,55,0.2);
padding:3.5rem 3rem;
box-shadow:0 20px 50px rgba(0,0,0,0.7);
opacity:0;
transform:translateY(30px);
animation:slideUp 1s ease-out forwards;
}
@keyframes slideUp{
to{opacity:1;transform:translateY(0)}
}
h1{
font-family:'Playfair Display',serif;
font-size:2.8em;
font-weight:700;
text-align:center;
letter-spacing:4px;
color:#E0E0E0;
margin-bottom:0.5rem;
}
.tagline{
text-align:center;
font-size:0.95em;
color:#888;
margin-bottom:3rem;
letter-spacing:0.5px;
}
.qr-section{
opacity:0;
max-height:0;
overflow:hidden;
transition:all 0.7s ease;
margin:2.5rem 0;
text-align:center;
}
.qr-section.show{
opacity:1;
max-height:500px;
}
.qr-box{
display:inline-block;
padding:1.8rem;
background:#0a0a0a;
border:1px solid rgba(212,175,55,0.25);
}
.qr-box img{display:block}
.conn-info{
margin-top:1.5rem;
display:flex;
align-items:center;
justify-content:center;
gap:0.8rem;
font-size:0.9em;
color:#999;
}
.amber-dot{
width:7px;
height:7px;
border-radius:50%;
background:#D4AF37;
box-shadow:0 0 12px rgba(212,175,55,0.6);
animation:pulse 2.5s ease-in-out infinite;
}
@keyframes pulse{
0%,100%{opacity:1}
50%{opacity:0.3}
}
.btn{
width:100%;
padding:1.1rem;
margin:0.6rem 0;
font:600 0.75em/1 'Manrope',sans-serif;
letter-spacing:3px;
text-transform:uppercase;
color:#D4AF37;
background:transparent;
border:1px solid rgba(212,175,55,0.3);
cursor:pointer;
position:relative;
overflow:hidden;
transition:all 0.4s ease;
}
.btn:before{
content:'';
position:absolute;
top:0;
left:-100%;
width:100%;
height:100%;
background:linear-gradient(90deg,transparent,rgba(212,175,55,0.15),transparent);
transition:left 0.5s;
}
.btn:hover:before{left:100%}
.btn:hover{
border-color:rgba(212,175,55,0.6);
box-shadow:0 0 25px rgba(212,175,55,0.15);
}
.btn:disabled{
opacity:0.3;
cursor:not-allowed;
}
.btn-alt{
color:#999;
border-color:rgba(150,150,150,0.2);
}
.status-bar{
margin-top:2.5rem;
padding:1rem 0;
text-align:center;
font-size:0.9em;
color:#666;
border-top:1px solid rgba(255,255,255,0.04);
}
.url-display{
margin-top:1rem;
padding:0.9rem;
background:rgba(0,0,0,0.35);
color:#777;
font:0.8em/1.4 monospace;
word-break:break-all;
text-align:center;
}
</style>
</head>
<body>
<div class="card">
<h1>RIFT</h1>
<p class="tagline">Seamless Connection</p>
<div class="qr-section" id="qr-area">
<div class="qr-box">
<img id="qr" width="250" height="250" alt="QR">
</div>
<div class="conn-info">
<span class="amber-dot"></span>
<span id="ip-text"></span>
</div>
</div>
<button class="btn" id="init-btn" onclick="init()">INITIALIZE</button>
<button class="btn btn-alt" id="refresh-btn" onclick="init()" style="display:none">REFRESH</button>
<div class="status-bar" id="status">Ready</div>
<div class="url-display" id="url"></div>
</div>
<script>
async function init(){
const initBtn=document.getElementById('init-btn');
const refreshBtn=document.getElementById('refresh-btn');
const qrArea=document.getElementById('qr-area');
const status=document.getElementById('status');
initBtn.disabled=true;
refreshBtn.disabled=true;
status.textContent='Connecting...';
try{
const res=await fetch('/start');
const data=await res.json();
document.getElementById('qr').src=data.qr;
document.getElementById('ip-text').textContent=data.ip;
document.getElementById('url').textContent=data.url;
qrArea.classList.add('show');
status.textContent='Ready';
initBtn.style.display='none';
refreshBtn.style.display='block';
refreshBtn.disabled=false;
}catch(err){
status.textContent='Failed';
initBtn.disabled=false;
refreshBtn.disabled=false;
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
