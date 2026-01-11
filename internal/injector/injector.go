package injector

import (
	"log"

	"github.com/go-vgo/robotgo"
)

func TypeStr(text string) {
	log.Printf("Typing: %s\n", text)
	robotgo.TypeStr(text)
}

func TapKey(key string) {
	log.Printf("Key Tap: %s\n", key)
	robotgo.KeyTap(key)
}

