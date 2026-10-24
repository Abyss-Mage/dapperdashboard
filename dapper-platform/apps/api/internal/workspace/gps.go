package workspace

import (
	"errors"
	"net/http"
	"time"

	"dapper.local/platform/internal/postgres"
)

func gpsFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrGPSDisabled):
		reply(w, 409, map[string]string{"error": "On-shift location is not enabled for this location", "code": "disabled"})
	case errors.Is(err, postgres.ErrGPSNotTracking):
		reply(w, 409, map[string]string{"error": "Location is only recorded while you are clocked in and not on break", "code": "not_tracking"})
	case errors.Is(err, postgres.ErrGPSRate):
		reply(w, 429, map[string]string{"error": "Sample received too soon", "code": "rate"})
	case errors.Is(err, postgres.ErrGPSInput):
		reply(w, 422, map[string]string{"error": "Send a current, valid location sample"})
	case errors.Is(err, postgres.ErrGPSSettingRange):
		reply(w, 422, map[string]string{"error": err.Error()})
	case errors.Is(err, postgres.ErrGPSVersion):
		reply(w, 409, map[string]string{"error": "Location settings changed. Refresh and review before saving again"})
	default:
		failure(w, err)
	}
}

func registerGPSRoutes(mux *http.ServeMux, store *postgres.Store) {
	// Employee device: the open shift supplies the scope; selectors and actor fields are rejected.
	mux.HandleFunc("GET /api/v1/employee/gps", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query()) != 0 {
			reply(w, 400, map[string]string{"error": "No selectors are accepted"})
			return
		}
		data, err := store.EmployeeGPSStatus(r.Context(), requestSession(r).principalID)
		if err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("POST /api/v1/employee/gps/samples", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Latitude       float64  `json:"latitude"`
			Longitude      float64  `json:"longitude"`
			AccuracyMeters *float64 `json:"accuracyMeters"`
			CapturedAt     string   `json:"capturedAt"`
		}
		if !decode(w, r, &input) {
			return
		}
		captured, err := time.Parse(time.RFC3339Nano, input.CapturedAt)
		if err != nil {
			reply(w, 422, map[string]string{"error": "Send a current, valid location sample"})
			return
		}
		data, err := store.RecordGPSSample(r.Context(), requestSession(r).principalID, postgres.GPSSampleInput{Latitude: input.Latitude, Longitude: input.Longitude, AccuracyMeters: input.AccuracyMeters, CapturedAt: captured})
		if err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 201, data)
	})
	mux.HandleFunc("POST /api/v1/employee/gps/state", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			State string `json:"state"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.ReportGPSState(r.Context(), requestSession(r).principalID, input.State); err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"state": input.State})
	})

	// Supervisors/admins: attendance.read views, attendance.manage settings.
	base := "/api/v1/organizations/{organization}/locations/{location}/gps"
	resolve := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, action)
		if err != nil {
			failure(w, err)
			return postgres.Scope{}, false
		}
		return scope, true
	}
	mux.HandleFunc("GET "+base+"/settings", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.read")
		if !ok {
			return
		}
		data, err := store.GetGPSSettings(r.Context(), scope)
		if err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("PUT "+base+"/settings", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.manage")
		if !ok {
			return
		}
		var input struct {
			Enabled               bool `json:"enabled"`
			SampleIntervalSeconds int  `json:"sampleIntervalSeconds"`
			RetentionDays         int  `json:"retentionDays"`
			Version               int  `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		data, err := store.SaveGPSSettings(r.Context(), scope, postgres.GPSSettings{Enabled: input.Enabled, SampleIntervalSeconds: input.SampleIntervalSeconds, RetentionDays: input.RetentionDays, Version: input.Version})
		if err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("GET "+base+"/live", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.read")
		if !ok {
			return
		}
		data, err := store.GPSLive(r.Context(), scope)
		if err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("GET "+base+"/shifts/{attendance}/samples", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.read")
		if !ok {
			return
		}
		if !uuid(r.PathValue("attendance")) {
			reply(w, 400, map[string]string{"error": "Valid shift required"})
			return
		}
		data, err := store.GPSShiftSamples(r.Context(), scope, r.PathValue("attendance"))
		if err != nil {
			gpsFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"samples": data})
	})
}
