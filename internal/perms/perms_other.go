//go:build !(darwin && cgo)

package perms

// Check reports everything granted: only macOS asks for consent.
func Check() Status { return Status{Input: true, Screen: true} }

func Request(string) error { return nil }
