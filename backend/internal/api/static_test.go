package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticServesIcons(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "icon.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644)
	os.WriteFile(filepath.Join(dir, "favicon.ico"), []byte{0, 0, 1, 0, 1, 0, 16, 16}, 0o644)
	os.WriteFile(filepath.Join(dir, "apple-icon.png"), []byte("\x89PNG\r\n\x1a\n...."), 0o644)
	srv := httptest.NewServer(staticHandler(dir))
	defer srv.Close()
	for path, want := range map[string]string{
		"/icon.svg":       "image/svg+xml",
		"/favicon.ico":    "image/",
		"/apple-icon.png": "image/png",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), want) {
			t.Errorf("%s: %d %q", path, res.StatusCode, res.Header.Get("Content-Type"))
		}
	}
}
