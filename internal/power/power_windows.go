//go:build windows

package power

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	powrprof            = windows.NewLazySystemDLL("powrprof.dll")
	procLockWorkStation = user32.NewProc("LockWorkStation")
	procPostMessage     = user32.NewProc("PostMessageW")
	procSetSuspendState = powrprof.NewProc("SetSuspendState")
)

const (
	hwndBroadcast  = 0xFFFF
	wmSysCommand   = 0x0112
	scMonitorPower = 0xF170
	monitorOff     = 2
)

type winPower struct{}

func New() Controller { return winPower{} }

func call(p *windows.LazyProc, args ...uintptr) error {
	if r, _, err := p.Call(args...); r == 0 {
		return fmt.Errorf("%s: %w", p.Name, err)
	}
	return nil
}

func (winPower) Lock() error { return call(procLockWorkStation) }

// DisplayOff blanks every monitor; any mouse or key input wakes them.
func (winPower) DisplayOff() error {
	return call(procPostMessage, hwndBroadcast, wmSysCommand, scMonitorPower, monitorOff)
}

func (winPower) Sleep() error {
	// hibernate=false, force=false, disableWakeEvents=false
	return call(procSetSuspendState, 0, 0, 0)
}
