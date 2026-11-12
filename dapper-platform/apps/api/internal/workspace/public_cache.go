package workspace

import (
	"bytes"
	"net/http"
	"strings"

	"dapper.local/platform/internal/cache"
)

// publicCache holds the public website's data in Redis when DAPPER_REDIS_URL is set (owner 2026-11-13). Nil means no cache.
var publicCache *cache.Cache

// SetPublicCache is called once at start-up.
func SetPublicCache(c *cache.Cache) { publicCache = c }

// cachedPublicPath: only the website's own reads, which are the same for every visitor. Bookings, payments, staff and dealer data are
// never cached.
func cachedPublicPath(p string) bool {
	return p == "/api/v1/public/portfolio" || p == "/api/v1/public/dealers" || strings.HasPrefix(p, "/api/v1/public/sites/") && strings.Count(p, "/") == 5
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }

// Unwrap keeps http.ResponseController (longer upload deadlines, flushing) working through the wrapper.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

type captureWriter struct {
	statusWriter
	body bytes.Buffer
}

func (c *captureWriter) Write(b []byte) (int, error) { c.body.Write(b); return c.ResponseWriter.Write(b) }

func servePublicCached(w http.ResponseWriter, r *http.Request, next http.Handler) {
	key := r.URL.Path + "?" + r.URL.RawQuery
	if body, ok := publicCache.Get(r.Context(), key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Dapper-Cache", "hit")
		_, _ = w.Write(body)
		return
	}
	if publicCache == nil {
		next.ServeHTTP(w, r)
		return
	}
	cw := &captureWriter{statusWriter: statusWriter{ResponseWriter: w, status: 200}}
	next.ServeHTTP(cw, r)
	if cw.status == 200 && strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		publicCache.Set(r.Context(), key, cw.body.Bytes())
	}
}
