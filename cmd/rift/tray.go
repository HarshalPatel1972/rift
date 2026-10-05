//go:build windows

package main

import (
	_ "embed"
	"fmt"

	"fyne.io/systray"

	"github.com/HarshalPatel1972/rift/internal/server"
)

//go:embed rift.ico
var trayIcon []byte

// runTray keeps RIFT alive in the notification area after the window closes.
// Left-click opens the dashboard; right-click shows the menu. Blocks until
// Quit, then runs onExit.
func runTray(srv *server.Server, open func(), onExit func()) {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("RIFT")
		systray.SetOnTapped(open)

		mOpen := systray.AddMenuItem("Open RIFT", "Show the pairing window")
		mPause := systray.AddMenuItemCheckbox("Pause input", "Ignore input from the phone", false)
		mDisconnect := systray.AddMenuItem("Disconnect phone", "Drop the current phone")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit RIFT", "Stop RIFT")

		ch, cancel := srv.Subscribe()
		refresh := func() {
			st := srv.Status()
			tip := "RIFT · waiting for phone"
			if st.Connected {
				tip = fmt.Sprintf("RIFT · %s · %d ms", st.Device, st.RTTms)
			}
			if st.Peeking {
				tip += " · viewing your screen"
			}
			if st.Paused {
				tip += " · paused"
			}
			systray.SetTooltip(tip)
			if st.Paused != mPause.Checked() {
				if st.Paused {
					mPause.Check()
				} else {
					mPause.Uncheck()
				}
			}
			if st.Connected {
				mDisconnect.Enable()
			} else {
				mDisconnect.Disable()
			}
		}
		refresh()

		go func() {
			defer cancel()
			for {
				select {
				case <-ch:
					refresh()
				case <-mOpen.ClickedCh:
					open()
				case <-mPause.ClickedCh:
					srv.SetPaused(!srv.Status().Paused)
				case <-mDisconnect.ClickedCh:
					srv.Disconnect()
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, onExit)
}

func quitTray() { systray.Quit() }
