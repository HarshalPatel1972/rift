package server

import (
	"log"
	"net/http"

	"github.com/HarshalPatel1972/rift/internal/injector"
	"github.com/HarshalPatel1972/rift/web"
	"github.com/gorilla/websocket"
)

type Message struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Key  string `json:"key,omitempty"`
}


type Server struct {
	Addr     string
	Token    string
	upgrader websocket.Upgrader
}

func New(addr, token string) *Server {
	return &Server{
		Addr:  addr,
		Token: token,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Start() error {
	// Serve embedded assets
	http.Handle("/", http.FileServer(http.FS(web.Assets)))
	
	// WebSocket endpoint
	http.HandleFunc("/ws", s.handleWS)
	
	return http.ListenAndServe(s.Addr, nil)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Security Check
	token := r.URL.Query().Get("token")
	if token != s.Token {
		http.Error(w, "Forbidden: Invalid Token", http.StatusForbidden)
		log.Printf("Blocked unauthorized connection attempt from %s\n", r.RemoteAddr)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()
	log.Println("Client connected")

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		switch msg.Type {
		case "input":
			if msg.Text != "" {
				injector.TypeStr(msg.Text)
			}
		case "key":
			if msg.Key != "" {
				injector.TapKey(msg.Key)
			}
		}
	}
}
