// Local HTTPS entry point; never installs trust or changes certificate settings.
package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	if os.Getenv("DAPPER_HTTPS_WORKSPACE") != "1" {
		log.Fatal("Explicit local HTTPS workspace opt-in required")
	}
	// The installer writes DAPPER_LISTEN / DAPPER_PUBLIC_HOST; without them this stays the local-only 127.0.0.1:8443 gateway.
	listen := os.Getenv("DAPPER_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8443"
	}
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		log.Fatal("DAPPER_LISTEN must look like 127.0.0.1:8443")
	}
	hosts := map[string]bool{"127.0.0.1:" + port: true, "localhost:" + port: true}
	if public := os.Getenv("DAPPER_PUBLIC_HOST"); public != "" {
		hosts[public] = true
	}
	apiURL, _ := url.Parse("http://127.0.0.1:8121")
	webURL, _ := url.Parse("http://127.0.0.1:3006")
	handler := gatewayHandler(apiURL, webURL, hosts)
	go refreshAssetOverrides(apiURL)
	server := &http.Server{Addr: listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	if os.Getenv("DAPPER_BEHIND_PROXY") == "1" {
		log.Printf("Dapper listening on %s over plain HTTP behind your reverse proxy, which must serve the public HTTPS address", listen)
		log.Fatal(server.ListenAndServe())
	}
	certificate, key := os.Getenv("DAPPER_TLS_CERT"), os.Getenv("DAPPER_TLS_KEY")
	if certificate == "" || key == "" {
		log.Fatal("Explicit TLS certificate and private key paths required")
	}
	log.Printf("Dapper listening on %s over HTTPS; certificate trust is managed by the operator", listen)
	log.Fatal(server.ListenAndServeTLS(certificate, key))
}

// assetOverrides is the set of built-in website files the owner has replaced in the dashboard; the gateway serves those from the API.
var assetOverrides = struct {
	sync.RWMutex
	paths map[string]bool
}{paths: map[string]bool{}}

// refreshAssetOverrides keeps that set current: it asks the API for the list every few seconds.
func refreshAssetOverrides(apiURL *url.URL) {
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		if resp, err := client.Get(apiURL.String() + "/api/v1/public/site-asset-paths"); err == nil {
			var body struct {
				Paths []string `json:"paths"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body) == nil {
				next := make(map[string]bool, len(body.Paths))
				for _, p := range body.Paths {
					next[p] = true
				}
				assetOverrides.Lock()
				assetOverrides.paths = next
				assetOverrides.Unlock()
			}
			resp.Body.Close()
		}
		time.Sleep(5 * time.Second)
	}
}

func gatewayHandler(apiURL, webURL *url.URL, hosts map[string]bool) http.Handler {
	api, web := httputil.NewSingleHostReverseProxy(apiURL), httputil.NewSingleHostReverseProxy(webURL)
	unavailable := func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Workspace service unavailable", 503)
	}
	// The API only accepts its own loopback host; the public host is checked here, so it is rewritten for the upstream hop.
	director := api.Director
	api.Director = func(r *http.Request) { director(r); r.Host = apiURL.Host }
	api.ErrorHandler = unavailable
	web.ErrorHandler = unavailable
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", api)
	mux.HandleFunc("/site/assets/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/site/assets/")
		assetOverrides.RLock()
		replaced := (r.Method == http.MethodGet || r.Method == http.MethodHead) && assetOverrides.paths[p]
		assetOverrides.RUnlock()
		if !replaced {
			web.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path, r2.URL.RawPath, r2.URL.RawQuery = "/api/v1/public/site-asset", "", "path="+url.QueryEscape(p)
		api.ServeHTTP(w, r2)
	})
	mux.Handle("/", web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			http.Error(w, "Unrecognised host", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/employee/uploads" || r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/site-assets") {
			// Multi-photo posts from phones need a longer window than the global limits.
			controller := http.NewResponseController(w)
			_ = controller.SetReadDeadline(time.Now().Add(3 * time.Minute))
			_ = controller.SetWriteDeadline(time.Now().Add(3 * time.Minute))
		}
		mux.ServeHTTP(w, r)
	})
}
