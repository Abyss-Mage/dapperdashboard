package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTLSGatewayRoutingAndCookie(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" || r.Method != "POST" || r.Header.Get("Origin") != "https://127.0.0.1:8443" {
			http.Error(w, "unexpected request", 400)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "__Host-dapper_workspace", Value: "synthetic-fixture", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(204)
	}))
	defer api.Close()
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "workspace fixture") }))
	defer web.Close()
	apiURL, _ := url.Parse(api.URL)
	webURL, _ := url.Parse(web.URL)
	gateway := httptest.NewTLSServer(gatewayHandler(apiURL, webURL))
	defer gateway.Close()
	client := gateway.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	request := func(method, path, host string) *http.Response {
		r, _ := http.NewRequest(method, gateway.URL+path, strings.NewReader(`{}`))
		r.Host = host
		r.Header.Set("Origin", "https://127.0.0.1:8443")
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := request("POST", "/api/v1/auth/login", "127.0.0.1:8443")
	defer res.Body.Close()
	if res.StatusCode != 204 || len(res.Cookies()) != 1 || !res.Cookies()[0].Secure || !res.Cookies()[0].HttpOnly {
		t.Fatal("protected upstream cookie not preserved")
	}
	res = request("GET", "/workspace", "127.0.0.1:8443")
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "workspace fixture" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("frontend routing failed")
	}
	res = request("GET", "/", "127.0.0.1:8443")
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "workspace fixture" {
		t.Fatal("root routing failed")
	}
	res = request("GET", "/workspace", "untrusted.example")
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("host rebinding accepted")
	}
}
