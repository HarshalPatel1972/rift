// Package web embeds RIFT's front ends: the phone app (served on the LAN) and
// the desktop dashboard (served on loopback only).
package web

import (
	"embed"
	"io/fs"
)

//go:embed phone desktop icon.png
var files embed.FS

// Icon is the app icon PNG.
var Icon, _ = files.ReadFile("icon.png")

// Phone is the phone web app's file tree.
var Phone, _ = fs.Sub(files, "phone")

// Desktop is the dashboard's file tree.
var Desktop, _ = fs.Sub(files, "desktop")
