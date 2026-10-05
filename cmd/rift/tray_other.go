//go:build !windows

package main

import "github.com/HarshalPatel1972/rift/internal/server"

var quitCh = make(chan struct{})

func runTray(_ *server.Server, _ func(), onExit func()) {
	<-quitCh
	onExit()
}

func quitTray() { close(quitCh) }
