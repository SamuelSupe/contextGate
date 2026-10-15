package ui

import (
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedAssetCachingAndDeepLinks(t *testing.T) {
	entries, err := fs.ReadDir(assets, "dist/assets")
	if err != nil {
		t.Fatal(err)
	}
	var script string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".js") {
			script = "/assets/" + entry.Name()
			break
		}
	}
	if script == "" {
		t.Fatal("build the embedded UI before testing")
	}
	for _, test := range []struct {
		path, cache, contentType string
		status                   int
	}{
		{"/", "no-cache", "text/html", 200},
		{"/sources/example/semantics", "no-cache", "text/html", 200},
		{script, "public, max-age=31536000, immutable", "javascript", 200},
		{"/assets/unavailable.js", "no-store", "text/plain", 404},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			Handler().ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
			if response.Code != test.status || response.Header().Get("Cache-Control") != test.cache || !strings.Contains(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatal("unexpected asset response", response.Code, response.Header())
			}
			if response.Body.Len() == 0 {
				t.Fatal("empty asset response")
			}
		})
	}
}
