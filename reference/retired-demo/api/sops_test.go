package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSOPReferenceBoundary(t *testing.T) {
	s, err := newStore(filepath.Join(t.TempDir(), "demo.json"))
	if err != nil {
		t.Fatal(err)
	}
	h := s.handler()
	get := func(path string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Host = "127.0.0.1:8120"
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "dapper_demo", Value: s.token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if get("/api/v1/demo/sops/onboarding_policy", false).Code != 401 {
		t.Fatal("anonymous document access allowed")
	}
	w := get("/api/v1/demo/sops", true)
	var body struct {
		Documents []sopRecord `json:"documents"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body.Documents) != 5 {
		t.Fatal("manifest missing")
	}
	for _, document := range body.Documents {
		w = get("/api/v1/demo/sops/"+document.ID, true)
		if w.Code != 200 || w.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(w.Body.String(), "%PDF-") {
			t.Fatal("document unavailable")
		}
		if fmt.Sprintf("%x", sha256.Sum256(w.Body.Bytes())) != document.SHA256 {
			t.Fatal("document identity mismatch")
		}
	}
	if get("/api/v1/demo/sops/unknown", true).Code != 404 {
		t.Fatal("unknown document accepted")
	}
}
