package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDemoCustomerBoundaryAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.json")
	s, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	h := s.handler()
	r := httptest.NewRequest("GET", "/api/v1/workspace", nil)
	r.Host = "127.0.0.1:8120"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("anonymous access allowed")
	}
	send := func(origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/customers", strings.NewReader(body))
		r.Host = "127.0.0.1:8120"
		r.AddCookie(&http.Cookie{Name: "dapper_demo", Value: s.token})
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"name":"Demo Person","email":"person@example.test","phone":""}`
	if w := send("https://untrusted.example", body); w.Code != 403 {
		t.Fatal("cross-origin write allowed")
	}
	if w := send("http://127.0.0.1:3006", `{"name":"x","email":"invalid"}`); w.Code != 422 {
		t.Fatal("invalid fields accepted")
	}
	if w := send("http://127.0.0.1:3006", body+body); w.Code != 400 {
		t.Fatal("multiple JSON objects accepted")
	}
	if w := send("http://127.0.0.1:3006", `{"name":"Demo Person","email":"Demo <person@example.test>","phone":""}`); w.Code != 422 {
		t.Fatal("formatted email address accepted")
	}
	if w := send("http://127.0.0.1:3006", body); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := send("http://127.0.0.1:3006", body); w.Code != 409 {
		t.Fatal("duplicate accepted")
	}
	if w := send("http://127.0.0.1:3006", `{"name":"Another Person","email":"second@example.test","phone":""}`); w.Code != 201 {
		t.Fatal("second atomic write failed", w.Body.String())
	}
	restored, err := newStore(path)
	if err != nil || len(restored.Customers) != 5 {
		t.Fatal("customer did not persist", err)
	}
}

func TestDemoRejectsRebindingHost(t *testing.T) {
	s, err := newStore(filepath.Join(t.TempDir(), "demo.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://attacker.example/api/v1/demo/session", nil)
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("untrusted host accepted")
	}
}
