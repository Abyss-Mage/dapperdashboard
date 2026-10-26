package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestVehicleCustomerLinkAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.json")
	s, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	send := func(origin, body string, auth bool) int {
		r := httptest.NewRequest("POST", "/api/v1/vehicles", strings.NewReader(body))
		r.Host = "127.0.0.1:8120"
		r.Header.Set("Origin", origin)
		if auth {
			r.AddCookie(&http.Cookie{Name: "dapper_demo", Value: s.token})
		}
		w := httptest.NewRecorder()
		s.handler().ServeHTTP(w, r)
		return w.Code
	}
	origin := "http://127.0.0.1:3006"
	body := `{"customerId":"c1","name":"Demo sailboat","category":"Marine","identifier":"DEMO-BOAT-1"}`
	if send(origin, body, false) != 401 || send("https://untrusted.example", body, true) != 403 {
		t.Fatal("vehicle boundary bypass")
	}
	if send(origin, strings.Replace(body, `"c1"`, `"missing"`, 1), true) != 422 {
		t.Fatal("orphan vehicle accepted")
	}
	if send(origin, strings.Replace(body, `"Demo sailboat"`, `"x"`, 1), true) != 422 {
		t.Fatal("invalid description accepted")
	}
	if send(origin, strings.Replace(body, `"category"`, `"location"`, 1), true) != 400 {
		t.Fatal("client location override accepted")
	}
	if send(origin, body, true) != 201 {
		t.Fatal("valid equipment rejected")
	}
	if send(origin, body, true) != 409 {
		t.Fatal("duplicate identifier accepted")
	}
	reloaded, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Vehicles) != 4 {
		t.Fatal("vehicle did not persist")
	}
	v := reloaded.Vehicles[3]
	if v.CustomerID != "c1" || v.Location != "peterborough" || v.Category != "Marine" {
		t.Fatal("vehicle ownership corrupted")
	}
}
