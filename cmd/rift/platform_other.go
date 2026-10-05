//go:build !windows && !darwin

package main

import (
	"errors"
	"log"
)

// RIFT targets Windows; these stubs only keep the package building elsewhere.

const (
	isMac       = false
	blockedHint = ""
)

func openWindow(url string) { log.Printf("open %s", url) }

func showError(msg string) {}

func autostartEnabled() bool { return false }

func setAutostart(bool) error { return errors.New("autostart is only supported on Windows") }
