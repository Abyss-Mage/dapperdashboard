package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"time"
)

func attendanceFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrAttendanceActiveShift) {
		reply(w, 409, map[string]string{"error": "You already have an active shift in this location"})
		return
	}
	if errors.Is(err, postgres.ErrAttendanceNoActiveShift) {
		reply(w, 409, map[string]string{"error": "No active shift in this location"})
		return
	}
	if errors.Is(err, postgres.ErrAttendanceInvalidTransition) {
		reply(w, 409, map[string]string{"error": "Invalid action for current shift state"})
		return
	}
	if errors.Is(err, postgres.ErrGPSRequired) {
		reply(w, 422, map[string]string{"error": "Location is required to clock in. Allow location access for this site in your browser, then try again.", "code": "location_required"})
		return
	}
	if errors.Is(err, postgres.ErrGPSInput) {
		reply(w, 422, map[string]string{"error": "Your location reading was not current. Try again.", "code": "location_invalid"})
		return
	}
	if errors.Is(err, postgres.ErrAttendanceInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"notes": "Notes must not exceed 1000 characters"}})
		return
	}
	failure(w, err)
}

func registerAttendanceRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/attendance"

	mux.HandleFunc("POST "+base+"/clock-in", func(w http.ResponseWriter, r *http.Request) {
		shopClockOnly(w)
	})

	mux.HandleFunc("POST "+base+"/start-break", func(w http.ResponseWriter, r *http.Request) {
		shopClockOnly(w)
	})

	mux.HandleFunc("POST "+base+"/end-break", func(w http.ResponseWriter, r *http.Request) {
		shopClockOnly(w)
	})

	mux.HandleFunc("POST "+base+"/clock-out", func(w http.ResponseWriter, r *http.Request) {
		shopClockOnly(w)
	})

	mux.HandleFunc("GET "+base+"/active", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		actor := requestSession(r).principalID
		scope, err := store.ResolveScope(r.Context(), actor, org, loc, "attendance.record")
		if err != nil {
			scope, err = store.ResolveScope(r.Context(), actor, org, loc, "attendance.read")
		}
		if err != nil {
			failure(w, err)
			return
		}
		item, err := store.ActiveAttendance(r.Context(), scope)
		if err != nil {
			attendanceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"item": item})
	})

	// Worked time per staff member and day, for the day / week / month totals on the Staff hours page.
	mux.HandleFunc("GET "+base+"/totals", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "attendance.read")
		if err != nil {
			failure(w, err)
			return
		}
		q := r.URL.Query()
		items, err := store.AttendanceTotals(r.Context(), scope, q.Get("from"), q.Get("to"), q.Get("tz"))
		if errors.Is(err, postgres.ErrAttendanceTotalsInput) {
			reply(w, 422, map[string]string{"error": "Choose a date range of at most 400 days"})
			return
		}
		if err != nil {
			attendanceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"staff": items})
	})

	mux.HandleFunc("GET "+base+"/history", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		actor := requestSession(r).principalID
		scope, err := store.ResolveScope(r.Context(), actor, org, loc, "attendance.read")
		if err != nil {
			scope, err = store.ResolveScope(r.Context(), actor, org, loc, "attendance.record")
		}
		if err != nil {
			failure(w, err)
			return
		}

		from := time.Now().Add(-30 * 24 * time.Hour)
		to := time.Now().Add(24 * time.Hour)
		if raw := r.URL.Query().Get("from"); raw != "" {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, raw)
			}
			if err != nil {
				reply(w, 422, map[string]any{"errors": map[string]string{"from": "Provide timestamp with explicit UTC offset"}})
				return
			}
			from = parsed
		}
		if raw := r.URL.Query().Get("to"); raw != "" {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, raw)
			}
			if err != nil {
				reply(w, 422, map[string]any{"errors": map[string]string{"to": "Provide timestamp with explicit UTC offset"}})
				return
			}
			to = parsed
		}

		items, err := store.ListAttendance(r.Context(), scope, from, to, 50)
		if err != nil {
			attendanceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 50})
	})

	mux.HandleFunc("GET "+base+"/{attendance}/events", func(w http.ResponseWriter, r *http.Request) {
		org, loc, attID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("attendance")
		if !uuid(org) || !uuid(loc) || !uuid(attID) {
			reply(w, 400, map[string]string{"error": "Valid organization, location and attendance selectors required"})
			return
		}
		actor := requestSession(r).principalID
		scope, err := store.ResolveScope(r.Context(), actor, org, loc, "attendance.read")
		if err != nil {
			scope, err = store.ResolveScope(r.Context(), actor, org, loc, "attendance.record")
		}
		if err != nil {
			failure(w, err)
			return
		}
		items, err := store.AttendanceEvents(r.Context(), scope, attID)
		if err != nil {
			attendanceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
}

// locationInput is the optional location reading sent with a clock-in.
type locationInput struct {
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	AccuracyMeters *float64 `json:"accuracyMeters"`
	CapturedAt     string   `json:"capturedAt"`
}

// fix converts the request value (nil when absent) into a store fix; a malformed timestamp is rejected here.
func (l *locationInput) fix(w http.ResponseWriter) (*postgres.GPSFix, bool) {
	if l == nil {
		return nil, true
	}
	captured, err := time.Parse(time.RFC3339Nano, l.CapturedAt)
	if err != nil {
		reply(w, 422, map[string]string{"error": "Your location reading was not current. Try again.", "code": "location_invalid"})
		return nil, false
	}
	return &postgres.GPSFix{Latitude: l.Latitude, Longitude: l.Longitude, AccuracyMeters: l.AccuracyMeters, CapturedAt: captured}, true
}
