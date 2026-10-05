package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIconURLsAreVersioned(t *testing.T) {
	for _, tc := range []struct {
		fsys fs.FS
		file string
		ref  string
	}{
		{Desktop, "index.html", `"/icon.png?v=`},
		{Phone, "index.html", `"icon-192.png?v=`},
		{Phone, "manifest.webmanifest", `"icon-512.png?v=`},
	} {
		b, err := fs.ReadFile(tc.fsys, tc.file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), tc.ref) {
			t.Errorf("%s: no versioned %s", tc.file, tc.ref)
		}
	}
}

// The overlay must still work behind http.FileServerFS (needs Stat + Seek).
func TestOverlayServesOverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.FileServerFS(Phone))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), "icon-192.png?v=") {
		t.Fatalf("status %d, versioned=%v", res.StatusCode, strings.Contains(string(b), "icon-192.png?v="))
	}
	if r, _ := http.Get(srv.URL + "/icon-192.png?v=abc"); r.StatusCode != 200 {
		t.Fatalf("icon with version query: %d", r.StatusCode)
	}
}
