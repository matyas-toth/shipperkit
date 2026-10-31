package ui

import (
	"net/http/httptest"
	"testing"
)

func TestAssetsDoNotFallbackForMissingScripts(t *testing.T) {
	for _, path := range []string{"/assets/missing.js", "/missing.woff2"} {
		w := httptest.NewRecorder()
		Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("missing asset returned %d", w.Code)
		}
	}
}
