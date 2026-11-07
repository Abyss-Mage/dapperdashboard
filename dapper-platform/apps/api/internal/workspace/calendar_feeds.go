package workspace

import (
	"errors"
	"net/http"
	"strings"

	"dapper.local/platform/internal/postgres"
)

func calendarFeedFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrCalendarFeedInput):
		reply(w, 422, map[string]string{"error": "Keep the name to 80 characters or fewer"})
	case errors.Is(err, postgres.ErrCalendarFeedNotFound):
		reply(w, 404, map[string]string{"error": "That calendar link no longer exists"})
	default:
		failure(w, err)
	}
}

// registerCalendarFeedRoutes: private subscribe links (franchise bookings and an employee's own calendar) and the public
// iCalendar endpoint they point to. The secret is the token in the address, so it is shown once, when the link is made.
func registerCalendarFeedRoutes(mux *http.ServeMux, store *postgres.Store, origin string) {
	linkFor := func(token string) map[string]string {
		https := strings.TrimRight(origin, "/") + "/api/v1/public/calendar/" + token + ".ics"
		return map[string]string{"url": https, "webcalUrl": "webcal://" + strings.TrimPrefix(https, "https://")}
	}
	base := "/api/v1/organizations/{organization}/locations/{location}/calendar-feeds"
	scopeOf := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return postgres.Scope{}, false
		}
		return postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		items, err := store.FranchiseCalendarFeeds(r.Context(), scope)
		if err != nil {
			calendarFeedFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		var input struct {
			Label string `json:"label"`
		}
		if !decode(w, r, &input) {
			return
		}
		feed, token, err := store.CreateFranchiseCalendarFeed(r.Context(), scope, input.Label)
		if err != nil {
			calendarFeedFailure(w, err)
			return
		}
		out := linkFor(token)
		reply(w, 201, map[string]any{"feed": feed, "url": out["url"], "webcalUrl": out["webcalUrl"]})
	})
	mux.HandleFunc("DELETE "+base+"/{feed}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		if id := r.PathValue("feed"); !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid link selector required"})
			return
		} else if err := store.RevokeFranchiseCalendarFeed(r.Context(), scope, id); err != nil {
			calendarFeedFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"revoked": true})
	})

	mux.HandleFunc("GET /api/v1/employee/calendar-feeds", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.EmployeeCalendarFeeds(r.Context(), requestSession(r).principalID)
		if err != nil {
			calendarFeedFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/employee/calendar-feeds", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Label string `json:"label"`
		}
		if !decode(w, r, &input) {
			return
		}
		feed, token, err := store.CreateEmployeeCalendarFeed(r.Context(), requestSession(r).principalID, input.Label)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		out := linkFor(token)
		reply(w, 201, map[string]any{"feed": feed, "url": out["url"], "webcalUrl": out["webcalUrl"]})
	})
	mux.HandleFunc("DELETE /api/v1/employee/calendar-feeds/{feed}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("feed")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid link selector required"})
			return
		}
		if err := store.RevokeEmployeeCalendarFeed(r.Context(), requestSession(r).principalID, id); err != nil {
			calendarFeedFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"revoked": true})
	})

	// The subscribe address itself. It has no sign-in: the secret token in the address is the credential.
	mux.HandleFunc("GET /api/v1/public/calendar/{file}", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSuffix(r.PathValue("file"), ".ics")
		ics, err := store.CalendarFeedICS(r.Context(), token)
		if errors.Is(err, postgres.ErrCalendarFeedNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=\"dapper-bookings.ics\"")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		_, _ = w.Write([]byte(ics))
	})
}
