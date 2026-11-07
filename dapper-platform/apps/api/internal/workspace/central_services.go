package workspace

import (
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func centralServiceFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrCentralServiceConflict) || errors.Is(err, postgres.ErrCentralServiceHasChildren) {
		reply(w, 409, map[string]string{"error": "Central service changed, is unavailable, or still has active add-ons. Refresh before editing"})
		return
	}
	if errors.Is(err, postgres.ErrCentralServiceInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"parentId": "Choose an active central main service in this organization"}})
		return
	}
	failure(w, err)
}

func registerCentralServiceRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/central-services"
	for _, route := range []string{"GET " + base, "POST " + base, "PUT " + base + "/{service}", "POST " + base + "/{service}/archive"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			org, id := r.PathValue("organization"), r.PathValue("service")
			if !uuid(org) || (id != "" && !uuid(id)) {
				reply(w, 400, map[string]string{"error": "Valid organization and service selectors required"})
				return
			}
			actor := requestSession(r).principalID
			if r.Method == "GET" {
				items, err := store.ListCentralServices(r.Context(), actor, org)
				if err != nil {
					centralServiceFailure(w, err)
					return
				}
				reply(w, 200, map[string]any{"items": items, "limit": 1000})
				return
			}
			if strings.HasSuffix(r.URL.Path, "/archive") {
				var input struct {
					Version int `json:"version"`
				}
				if !decode(w, r, &input) {
					return
				}
				if input.Version <= 0 {
					reply(w, 422, map[string]any{"errors": map[string]string{"version": "Send current service version"}})
					return
				}
				item, err := store.ArchiveCentralService(r.Context(), actor, org, id, input.Version)
				if err != nil {
					centralServiceFailure(w, err)
					return
				}
				reply(w, 200, item)
				return
			}
			var input struct {
				Name     string  `json:"name"`
				Kind     string  `json:"kind"`
				ParentID  *string  `json:"parentId"`
				ParentIDs []string `json:"parentIds"`
				Version   int      `json:"version"`
				// Price is optional: absent = leave unchanged, null = clear, number = set (whole cents, CAD, before tax).
				Price json.RawMessage `json:"priceCents"`
				// Duration is optional like the price: absent = leave unchanged, null = clear, number = minutes to complete.
				Duration json.RawMessage `json:"durationMinutes"`
			}
			if !decode(w, r, &input) {
				return
			}
			input.Name = strings.TrimSpace(input.Name)
			fields := map[string]string{}
			if n := utf8.RuneCountInString(input.Name); n < 2 || n > 120 {
				fields["name"] = "Enter a service name between 2 and 120 characters"
			}
			if input.Kind != "main" && input.Kind != "addon" {
				fields["kind"] = "Kind must be main or addon"
			}
			if input.Kind == "main" && (input.ParentID != nil || len(input.ParentIDs) > 0) {
				fields["parentId"] = "Main services cannot have a parent"
			}
			if input.Kind == "addon" {
				ids := input.ParentIDs
				if len(ids) == 0 && input.ParentID != nil {
					ids = []string{*input.ParentID}
				}
				okIDs := len(ids) > 0 && len(ids) <= 200
				for _, p := range ids {
					if !uuid(p) || p == id {
						okIDs = false
					}
				}
				if !okIDs {
					fields["parentId"] = "Choose at least one active central main service"
				}
			}
			if r.Method == "PUT" && input.Version <= 0 {
				fields["version"] = "Send current service version"
			}
			if r.Method == "POST" && input.Version != 0 {
				fields["version"] = "Omit version when creating a service"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			price := postgres.CentralServiceInput{}
			if input.Price != nil {
				price.PriceSet = true
				if string(input.Price) != "null" {
					var cents int64
					if err := json.Unmarshal(input.Price, &cents); err != nil || cents < 0 || cents > 100000000000 {
						reply(w, 422, map[string]any{"errors": map[string]string{"priceCents": "Enter a price of 0 or more"}})
						return
					}
					price.PriceCents = &cents
				}
			}
			var durationSet bool
			var duration *int
			if input.Duration != nil {
				durationSet = true
				if string(input.Duration) != "null" {
					var minutes int
					if err := json.Unmarshal(input.Duration, &minutes); err != nil || minutes < 1 || minutes > 10080 {
						reply(w, 422, map[string]any{"errors": map[string]string{"durationMinutes": "Enter the time to complete in minutes (1 to 10080)"}})
						return
					}
					duration = &minutes
				}
			}
			item, err := store.SaveCentralService(r.Context(), actor, org, postgres.CentralServiceInput{
				DurationSet:     durationSet,
				DurationMinutes: duration,
				ID:              id,
				Name:            input.Name,
				Kind:            input.Kind,
				ParentID:        input.ParentID,
				ParentIDs:       input.ParentIDs,
				Version:         input.Version,
				PriceSet:        price.PriceSet,
				PriceCents:      price.PriceCents,
			})
			if err != nil {
				centralServiceFailure(w, err)
				return
			}
			status := 200
			if r.Method == "POST" {
				status = 201
			}
			reply(w, status, item)
		})
	}
	mux.HandleFunc("DELETE "+base+"/{service}", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("service")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid organization and service selectors required"})
			return
		}
		actor := requestSession(r).principalID
		version := 0
		if vStr := r.URL.Query().Get("version"); vStr != "" {
			if v, err := strconv.Atoi(vStr); err == nil && v > 0 {
				version = v
			}
		}
		res, err := store.DeleteCentralService(r.Context(), actor, org, id, version)
		if err != nil {
			centralServiceFailure(w, err)
			return
		}
		reply(w, 200, res)
	})
}
