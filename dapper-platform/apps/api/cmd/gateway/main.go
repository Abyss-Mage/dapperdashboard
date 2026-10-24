// Local HTTPS entry point; never installs trust or changes certificate settings.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func main() {
	if os.Getenv("DAPPER_HTTPS_WORKSPACE") != "1" {
		log.Fatal("Explicit local HTTPS workspace opt-in required")
	}
	certificate, key := os.Getenv("DAPPER_TLS_CERT"), os.Getenv("DAPPER_TLS_KEY")
	if certificate == "" || key == "" {
		log.Fatal("Explicit TLS certificate and private key paths required")
	}
	apiURL, _ := url.Parse("http://127.0.0.1:8121")
	webURL, _ := url.Parse("http://127.0.0.1:3006")
	handler := gatewayHandler(apiURL, webURL)
	server := &http.Server{Addr: "127.0.0.1:8443", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	log.Print("Local HTTPS workspace listening on 127.0.0.1:8443; certificate trust is managed by the user")
	log.Fatal(server.ListenAndServeTLS(certificate, key))
}

func gatewayHandler(apiURL, webURL *url.URL) http.Handler {
	api, web := httputil.NewSingleHostReverseProxy(apiURL), httputil.NewSingleHostReverseProxy(webURL)
	unavailable := func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Workspace service unavailable", 503)
	}
	api.ErrorHandler = unavailable
	web.ErrorHandler = unavailable
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", api)
	mux.Handle("/", web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "127.0.0.1:8443" && r.Host != "localhost:8443" {
			http.Error(w, "Local workspace host required", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/employee/uploads" {
			// Multi-photo posts from phones need a longer window than the global limits.
			controller := http.NewResponseController(w)
			_ = controller.SetReadDeadline(time.Now().Add(3 * time.Minute))
			_ = controller.SetWriteDeadline(time.Now().Add(3 * time.Minute))
		}
		mux.ServeHTTP(w, r)
	})
}
