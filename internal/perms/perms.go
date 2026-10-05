// Package perms reports and requests the OS permissions RIFT needs. Only
// macOS gates input and screen capture behind user consent; elsewhere
// everything is granted.
package perms

// Status is what the dashboard shows in its permission checklist.
type Status struct {
	// Required is true when this OS has permissions to grant at all.
	Required bool `json:"required"`
	// Input: macOS Accessibility, needed to type and click.
	Input bool `json:"input"`
	// Screen: macOS Screen Recording, needed for Screen Peek.
	Screen bool `json:"screen"`
}

// Kinds accepted by Request.
const (
	Input  = "input"
	Screen = "screen"
)
