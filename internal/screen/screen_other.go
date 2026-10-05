//go:build !windows

package screen

type unsupported struct{}

// New returns a capturer that reports ErrUnsupported until this platform has
// a backend.
func New() Capturer { return unsupported{} }

func (unsupported) Grab(Request) (*Frame, error) { return nil, ErrUnsupported }
