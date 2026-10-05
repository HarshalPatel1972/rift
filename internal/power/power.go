// Package power puts the PC to bed: lock, display off and sleep.
package power

import "errors"

// ErrUnsupported is returned on hosts without a power backend.
var ErrUnsupported = errors.New("power: not supported on this platform")

type Controller interface {
	Lock() error
	DisplayOff() error
	Sleep() error
}
