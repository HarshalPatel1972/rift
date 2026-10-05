//go:build darwin

package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	isMac       = true
	blockedHint = "macOS blocked RIFT's input. Allow RIFT under System Settings → Privacy & Security → Accessibility."
	agentLabel  = "com.harshalpatel.rift"
)

// openWindow shows the dashboard as a chromeless app window when a Chromium
// browser is installed (Safari has no app mode), else in the default browser.
func openWindow(url string) {
	for _, app := range []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Arc"} {
		if _, err := os.Stat(filepath.Join("/Applications", app+".app")); err == nil {
			cmd := exec.Command("/usr/bin/open", "-na", app, "--args", "--app="+url, "--window-size=460,820")
			if err := cmd.Start(); err == nil {
				go cmd.Wait()
				return
			}
		}
	}
	if err := exec.Command("/usr/bin/open", url).Start(); err != nil {
		log.Printf("open window: %v", err)
	}
}

func showError(msg string) {
	// AppleScript string literal: escape backslashes and quotes.
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(msg)
	exec.Command("/usr/bin/osascript", "-e",
		fmt.Sprintf(`display alert "RIFT" message "%s" as critical`, esc)).Run()
}

func agentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist")
}

// executable is the path launchd should start: the binary inside RIFT.app.
func executable() string {
	exe, _ := os.Executable()
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return exe
}

func autostartEnabled() bool {
	b, err := os.ReadFile(agentPath())
	return err == nil && strings.Contains(string(b), html.EscapeString(executable()))
}

func setAutostart(on bool) error {
	if !on {
		if err := os.Remove(agentPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--background</string>
	</array>
	<key>RunAtLoad</key><true/>
	<key>ProcessType</key><string>Interactive</string>
</dict>
</plist>
`, agentLabel, html.EscapeString(executable()))
	if err := os.MkdirAll(filepath.Dir(agentPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(agentPath(), []byte(plist), 0o644)
}
