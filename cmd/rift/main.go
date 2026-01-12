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
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>RIFT</title>
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⚡</text></svg>">
<link href="https://fonts.googleapis.com/css2?family=Playfair+Display:wght@700&family=Manrope:wght@400;500;600&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{
font-family:'Manrope',sans-serif;
min-height:100vh;
display:flex;
align-items:center;
justify-content:center;
background:#020202;
background:radial-gradient(circle at center,#111111 0%,#020202 100%);
color:#E0E0E0;
}
.container{
max-width:480px;
width:90%;
background:#141414;
border:1px solid rgba(212,175,55,0.2);
border-radius:8px;
padding:3.5rem 3rem;
box-shadow:0 20px 50px rgba(0,0,0,0.7);
opacity:0;
transform:translateY(20px);
animation:fadeInUp 0.8s ease-out forwards;
}
@keyframes fadeInUp{
to{opacity:1;transform:translateY(0)}
}
h1{
font-family:'Playfair Display',serif;
font-size:3em;
font-weight:700;
text-align:center;
color:#E0E0E0;
letter-spacing:4px;
margin-bottom:0.3rem;
}
.subtitle{
text-align:center;
color:#888;
font-size:0.9em;
margin-bottom:3rem;
font-weight:400;
letter-spacing:1px;
}
#qr-container{
margin:2.5rem 0;
text-align:center;
opacity:0;
max-height:0;
overflow:hidden;
transition:all 0.6s cubic-bezier(0.4,0,0.2,1);
}
#qr-container.visible{
opacity:1;
max-height:600px;
}
.qr-frame{
display:inline-block;
padding:1.5rem;
background:#0a0a0a;
border:1px solid rgba(212,175,55,0.3);
border-radius:4px;
box-shadow:inset 0 0 20px rgba(0,0,0,0.5);
}
#qr-container img{
display:block;
}
.ip-info{
display:flex;
align-items:center;
justify-content:center;
gap:0.7rem;
margin-top:1.5rem;
font-size:0.85em;
color:#999;
font-weight:500;
}
.status-dot{
width:6px;
height:6px;
border-radius:50%;
background:#D4AF37;
box-shadow:0 0 10px rgba(212,175,55,0.5);
animation:breathe 2s ease-in-out infinite;
}
@keyframes breathe{
0%,100%{opacity:1}
50%{opacity:0.4}
}
button{
width:100%;
background:transparent;
border:1px solid rgba(212,175,55,0.4);
color:#D4AF37;
padding:1rem 2rem;
font-size:0.8em;
font-family:inherit;
font-weight:600;
cursor:pointer;
border-radius:4px;
transition:all 0.3s ease;
letter-spacing:2px;
text-transform:uppercase;
margin:0.5rem 0;
position:relative;
overflow:hidden;
}
button::before{
content:'';
position:absolute;
inset:0;
background:linear-gradient(135deg,rgba(212,175,55,0.1),rgba(212,175,55,0.05));
opacity:0;
transition:opacity 0.3s;
}
button:hover::before{
opacity:1;
}
button:hover{
border-color:rgba(212,175,55,0.7);
box-shadow:0 0 20px rgba(212,175,55,0.2);
}
button:disabled{
opacity:0.3;
cursor:not-allowed;
}
#refresh{
border-color:rgba(150,150,150,0.3);
color:#888;
}
#refresh:hover{
border-color:rgba(150,150,150,0.5);
}
#status{
text-align:center;
margin-top:2rem;
font-size:0.85em;
color:#666;
padding:0.8rem;
border-top:1px solid rgba(255,255,255,0.05);
font-weight:400;
}
#url{
margin-top:1rem;
font-size:0.75em;
word-break:break-all;
text-align:center;
padding:0.8rem;
background:rgba(0,0,0,0.3);
border-radius:4px;
color:#777;
font-family:monospace;
}
</style>
</head>
<body>
<div class="container">
<h1>RIFT</h1>
<div class="subtitle">Seamless Connection</div>
<div id="qr-container">
<div class="qr-frame">
<img id="qr" src="" alt="Connection Code" width="260" height="260"/>
</div>
<div class="ip-info">
<span class="status-dot"></span>
<span><span id="current-ip"></span></span>
</div>
</div>
<button id="generate" onclick="generate()">Initialize</button>
<button id="refresh" onclick="generate()" style="display:none">↻ Reconnect</button>
<div id="status">Ready</div>
<div id="url"></div>
</div>
<script>
async function generate(){
const btn=document.getElementById('generate');
const refreshBtn=document.getElementById('refresh');
const qrContainer=document.getElementById('qr-container');
btn.disabled=true;
refreshBtn.disabled=true;
document.getElementById('status').textContent='Establishing connection...';
try{
const res=await fetch('/start');
const data=await res.json();
document.getElementById('qr').src=data.qr;
qrContainer.classList.add('visible');
document.getElementById('url').textContent=data.url;
document.getElementById('current-ip').textContent=data.ip;
document.getElementById('status').textContent='Connected';
btn.style.display='none';
refreshBtn.style.display='block';
refreshBtn.disabled=false;
}catch(e){
document.getElementById('status').textContent='Connection failed';
btn.disabled=false;
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
