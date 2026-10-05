// Package web embeds RIFT's front ends: the phone app (served on the LAN) and
// the desktop dashboard (served on loopback only).
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"strings"
	"time"
)

//go:embed phone desktop icon.png
var files embed.FS

// Icon is the app icon PNG.
var Icon, _ = files.ReadFile("icon.png")

// Phone is the phone web app's file tree.
var Phone fs.FS

// Desktop is the dashboard's file tree.
var Desktop fs.FS

func init() {
	// Browsers cache icons by URL (Edge app windows especially), so a new
	// logo would keep showing the old one. Stamp every icon reference with a
	// hash of the icon itself: a changed icon gets a URL no cache has seen.
	sum := sha256.Sum256(Icon)
	v := "?v=" + hex.EncodeToString(sum[:4])

	phone, _ := fs.Sub(files, "phone")
	Phone = rewrite(phone, map[string][]string{
		"index.html":           {"icon-192.png"},
		"manifest.webmanifest": {"icon-192.png", "icon-512.png"},
	}, v)
	desktop, _ := fs.Sub(files, "desktop")
	Desktop = rewrite(desktop, map[string][]string{
		"index.html": {"/icon.png"},
	}, v)
}

// rewrite returns fsys with each ref in the named files suffixed by v.
func rewrite(fsys fs.FS, refs map[string][]string, v string) fs.FS {
	o := overlay{FS: fsys, files: map[string][]byte{}}
	for name, rs := range refs {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			continue
		}
		s := string(b)
		for _, r := range rs {
			s = strings.ReplaceAll(s, `"`+r+`"`, `"`+r+v+`"`)
		}
		o.files[name] = []byte(s)
	}
	return o
}

// overlay serves replacement contents for a few files of an fs.FS.
type overlay struct {
	fs.FS
	files map[string][]byte
}

func (o overlay) Open(name string) (fs.File, error) {
	if b, ok := o.files[name]; ok {
		return &memFile{Reader: bytes.NewReader(b), name: name, size: int64(len(b))}, nil
	}
	return o.FS.Open(name)
}

type memFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *memFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *memFile) Close() error               { return nil }

// fs.FileInfo
func (f *memFile) Name() string       { return f.name[strings.LastIndex(f.name, "/")+1:] }
func (f *memFile) Size() int64        { return f.size }
func (f *memFile) Mode() fs.FileMode  { return 0o444 }
func (f *memFile) ModTime() time.Time { return time.Time{} }
func (f *memFile) IsDir() bool        { return false }
func (f *memFile) Sys() any           { return nil }
