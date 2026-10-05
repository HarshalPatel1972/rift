// Package injector turns decoded RIFT commands into OS input events.
package injector

// Injector synthesizes keyboard and mouse input on the host.
type Injector interface {
	// Edit presses Backspace del times, then types insert. Doing both in one
	// call lets the platform submit them as a single atomic batch.
	Edit(del int, insert string) error
	// Key taps a virtual-key code while holding the given protocol.Mod* bits.
	Key(vk uint8, mods uint8) error
	Move(dx, dy int) error
	// MoveTo places the pointer at absolute screen pixel coordinates.
	MoveTo(x, y int) error
	Button(button, action uint8) error
	// Scroll sends wheel deltas (120 = one notch). Positive dy scrolls up,
	// positive dx scrolls right, matching Windows conventions.
	Scroll(dx, dy int) error
}
