package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/HarshalPatel1972/rift/internal/server"
	"github.com/google/uuid"
	"github.com/mdp/qrterminal/v3"
)

func main() {
	ip := GetLocalIP()
	port := "8080"
	token := uuid.New().String()
	
	url := fmt.Sprintf("http://%s:%s?token=%s", ip, port, token)

	fmt.Println("\nSCAN THIS TO CONNECT:")
	config := qrterminal.Config{
		Level:     qrterminal.L,
		Writer:    os.Stdout,
		BlackChar: qrterminal.WHITE,
		WhiteChar: qrterminal.BLACK,
		QuietZone: 1,
	}
	qrterminal.GenerateWithConfig(url, config)

	fmt.Printf("\nRIFT Secure Link:\n%s\n\n", url)

	srv := server.New(":"+port, token)
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}

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
