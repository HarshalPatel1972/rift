// Package config persists RIFT's pairing key and running-instance info under
// the user's config directory (%APPDATA%\RIFT on Windows).
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

// Dir returns (and creates) RIFT's config directory.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "RIFT")
	return dir, os.MkdirAll(dir, 0o700)
}

func path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// NewKey returns a fresh random pairing key.
func NewKey() []byte {
	k := make([]byte, protocol.KeySize)
	if _, err := rand.Read(k); err != nil {
		panic(err)
	}
	return k
}

// LoadOrCreateKey returns the persisted pairing key, creating one on first
// run. Keeping it across restarts lets a paired phone reconnect on its own.
func LoadOrCreateKey() ([]byte, error) {
	p, err := path("pairing.key")
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(p); err == nil {
		if k, err := hex.DecodeString(strings.TrimSpace(string(b))); err == nil && len(k) == protocol.KeySize {
			return k, nil
		}
	}
	k := NewKey()
	return k, SaveKey(k)
}

// SaveKey replaces the persisted pairing key.
func SaveKey(k []byte) error {
	p, err := path("pairing.key")
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(hex.EncodeToString(k)), 0o600)
}

// Instance describes the running RIFT so a second launch can hand off to it.
type Instance struct {
	DashboardURL string `json:"dashboardUrl"`
}

func WriteInstance(i Instance) error {
	p, err := path("instance.json")
	if err != nil {
		return err
	}
	b, _ := json.Marshal(i)
	return os.WriteFile(p, b, 0o600)
}

func ReadInstance() (Instance, error) {
	var i Instance
	p, err := path("instance.json")
	if err != nil {
		return i, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return i, err
	}
	return i, json.Unmarshal(b, &i)
}

// LogPath is where the GUI build writes its log, since it has no console.
func LogPath() (string, error) { return path("rift.log") }

// Settings are host preferences changed from the dashboard.
type Settings struct {
	// PeekAllowed lets the paired phone view this screen (Screen Peek).
	PeekAllowed bool `json:"peekAllowed"`
	// Onboarded is set once the first-run story has been shown.
	Onboarded bool `json:"onboarded"`
}

func defaultSettings() Settings { return Settings{PeekAllowed: true} }

// LoadSettings returns saved settings, or defaults.
func LoadSettings() Settings {
	s := defaultSettings()
	if p, err := path("settings.json"); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			json.Unmarshal(b, &s)
		}
	}
	return s
}

func SaveSettings(s Settings) error {
	p, err := path("settings.json")
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, b, 0o600)
}
