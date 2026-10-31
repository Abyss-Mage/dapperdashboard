package workspace

import (
	"net/http"
	"strconv"
	"time"

	"dapper.local/platform/internal/postgres"
)

// statusRecorder remembers the status a handler answered with, for the activity log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func parseDay(v string, endOfDay bool) time.Time {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		if endOfDay {
			return t.Add(24 * time.Hour)
		}
		return t
	}
	return time.Time{}
}

// registerActivityLogRoutes: who signed in, from where, and what each person changed (administrators with access.manage).
func registerActivityLogRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/organizations/{organization}/activity-log", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		items, err := store.ActivityLog(r.Context(), requestSession(r).principalID, org, postgres.ActivityFilter{
			Kind: q.Get("kind"), Query: q.Get("q"), From: parseDay(q.Get("from"), false), To: parseDay(q.Get("to"), true), Limit: limit,
		})
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
}
