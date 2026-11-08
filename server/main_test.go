package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func TestHealth(t *testing.T) {
	w := get(newHandler(fstest.MapFS{}, nil), "/api/health")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
}

func TestSPAFallback(t *testing.T) {
	h := newHandler(fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("js")},
	}, nil)
	if w := get(h, "/some/route"); w.Code != 200 || w.Body.String() != "<html>app</html>" {
		t.Fatalf("fallback: %d %q", w.Code, w.Body.String())
	}
	if w := get(h, "/assets/app.js"); w.Body.String() != "js" {
		t.Fatalf("asset: %q", w.Body.String())
	}
	if w := get(h, "/api/nope"); w.Code != 404 {
		t.Fatalf("api 404: %d", w.Code)
	}
}

func TestMissingIndex(t *testing.T) {
	if w := get(newHandler(fstest.MapFS{}, nil), "/"); w.Code != 503 {
		t.Fatalf("got %d", w.Code)
	}
}
