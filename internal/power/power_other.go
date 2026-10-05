//go:build !windows

package power

type unsupported struct{}

func New() Controller { return unsupported{} }

func (unsupported) Lock() error       { return ErrUnsupported }
func (unsupported) DisplayOff() error { return ErrUnsupported }
func (unsupported) Sleep() error      { return ErrUnsupported }
