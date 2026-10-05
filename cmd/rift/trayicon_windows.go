package main

import (
	_ "embed"

	"fyne.io/systray"
)

//go:embed rift.ico
var trayIcon []byte

func setTrayIcon() { systray.SetIcon(trayIcon) }
