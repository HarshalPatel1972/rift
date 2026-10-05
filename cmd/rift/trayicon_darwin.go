//go:build darwin && cgo

package main

import (
	_ "embed"

	"fyne.io/systray"
)

// A monochrome template image adapts to light and dark menu bars.
//
//go:embed tray_template.png
var trayTemplate []byte

func setTrayIcon() { systray.SetTemplateIcon(trayTemplate, trayTemplate) }
