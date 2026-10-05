package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	isMac       = false
	blockedHint = "Input blocked — the focused window is probably running as administrator. Run RIFT as administrator to control it."
)

// openWindow shows the dashboard as a chromeless Edge/Chrome app window,
// falling back to the default browser.
func openWindow(url string) {
	for _, b := range []string{"msedge.exe", "chrome.exe"} {
		if path := findBrowser(b); path != "" {
			// Going through `start` lets the new window take foreground focus
			// (Windows refuses focus to windows spawned directly by a tray app).
			cmd := exec.Command("cmd", "/c", "start", "", path, "--app="+url, "--window-size=900,600")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if err := cmd.Start(); err == nil {
				go cmd.Wait()
				return
			}
		}
	}
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		log.Printf("open window: %v", err)
	}
}

func findBrowser(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	var dirs []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
		if v := os.Getenv(env); v != "" {
			dirs = append(dirs, v)
		}
	}
	sub := map[string]string{
		"msedge.exe": `Microsoft\Edge\Application\msedge.exe`,
		"chrome.exe": `Google\Chrome\Application\chrome.exe`,
	}[name]
	for _, d := range dirs {
		p := filepath.Join(d, sub)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func showError(msg string) {
	title, _ := windows.UTF16PtrFromString("RIFT")
	text, _ := windows.UTF16PtrFromString(msg)
	windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}

func autostartCommand() string {
	exe, _ := os.Executable()
	return `"` + exe + `" --background`
}

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("RIFT")
	return err == nil && strings.EqualFold(v, autostartCommand())
}

func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if on {
		return k.SetStringValue("RIFT", autostartCommand())
	}
	if err := k.DeleteValue("RIFT"); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
