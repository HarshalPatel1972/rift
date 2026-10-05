//go:build !windows && !(darwin && cgo)

package injector

import "errors"

var errUnsupported = errors.New("injector: input injection is only implemented on Windows")

type unsupported struct{}

// New returns an injector that rejects every call on non-Windows hosts, so the
// rest of RIFT still builds and tests everywhere.
func New() Injector { return unsupported{} }

func (unsupported) Edit(int, string) error    { return errUnsupported }
func (unsupported) Key(uint8, uint8) error    { return errUnsupported }
func (unsupported) Move(int, int) error       { return errUnsupported }
func (unsupported) MoveTo(int, int) error     { return errUnsupported }
func (unsupported) Button(uint8, uint8) error { return errUnsupported }
func (unsupported) Scroll(int, int) error     { return errUnsupported }
