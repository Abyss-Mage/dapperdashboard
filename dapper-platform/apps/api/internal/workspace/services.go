package workspace

import (
	"time"
	"encoding/json"
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func serviceFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrServiceConflict) {
		reply(w, 409, map[string]string{"error": "Service changed, is unavailable, or still has active add-ons. Refresh before editing"})
		return
	}
	if errors.Is(err, postgres.ErrServiceInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"parentId": "Choose an active main service in this location"}})
		return
	}
	failure(w, err)
}
func registerServiceRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/services"
	for _, route := range []string{"GET " + base, "POST " + base, "PUT " + base + "/{service}", "POST " + base + "/{service}/archive", "POST " + base + "/{service}/restore"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			organization, location, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("service")
			if !uuid(organization) || !uuid(location) || id != "" && !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid location and service selectors required"})
				return
			}
			action := "catalogue.manage"
			if r.Method == "GET" {
				action = "catalogue.read"
			}
			scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, action)
			if err != nil {
				failure(w, err)
				return
			}
			if r.Method == "GET" {
				items, err := store.Services(r.Context(), scope)
				if err != nil {
					failure(w, err)
					return
				}
				reply(w, 200, map[string]any{"items": items, "limit": 1000})
				return
			}
			if restore := strings.HasSuffix(r.URL.Path, "/restore"); restore || strings.HasSuffix(r.URL.Path, "/archive") {
				var input struct {
					Version int `json:"version"`
				}
				if !decode(w, r, &input) {
					return
				}
				if input.Version < 1 {
					reply(w, 422, map[string]any{"errors": map[string]string{"version": "Refresh to obtain the current service version"}})
					return
				}
				change := store.ArchiveService
				if restore {
					change = store.RestoreService
				}
				service, err := change(r.Context(), scope, id, input.Version)
				if err != nil {
					serviceFailure(w, err)
					return
				}
				reply(w, 200, service)
				return
			}
			var input struct {
				Name     string  `json:"name"`
				Kind     string  `json:"kind"`
				ParentID  *string  `json:"parentId"`
				ParentIDs []string `json:"parentIds"`
				Version   int      `json:"version"`
				// Time to complete, in minutes (null or absent = not set).
				DurationMinutes *int `json:"durationMinutes"`
				// error404 #112: a franchise's own service has its own price, CAD cents before tax (null = no price).
				// Price is optional on an edit: absent = leave unchanged, null = no price, number = CAD cents before tax.
				Price json.RawMessage `json:"priceCents"`
				// The sale (error404 #133): absent = leave unchanged, null = no sale.
				Sale     json.RawMessage `json:"salePriceCents"`
				SaleEnds *string         `json:"saleEndsOn"`
				// The booking category (#166): absent = leave unchanged, "" = from the name.
				Group *string `json:"bookingGroup"`
			}
			if !decode(w, r, &input) {
				return
			}
			fields := map[string]string{}
			if input.DurationMinutes != nil && (*input.DurationMinutes < 1 || *input.DurationMinutes > 10080) {
				fields["durationMinutes"] = "Enter the time to complete in minutes (1 to 10080)"
			}
			if n := utf8.RuneCountInString(strings.TrimSpace(input.Name)); n < 2 || n > 120 {
				fields["name"] = "Enter a service name between 2 and 120 characters"
			}
			if input.Kind != "main" && input.Kind != "addon" {
				fields["kind"] = "Choose main or addon"
			}
			parentsOK := input.ParentID == nil || uuid(*input.ParentID)
			for _, p := range input.ParentIDs {
				if !uuid(p) {
					parentsOK = false
				}
			}
			if !parentsOK || input.Kind == "addon" && input.ParentID == nil && len(input.ParentIDs) == 0 || input.Kind == "main" && (input.ParentID != nil || len(input.ParentIDs) > 0) {
				fields["parentId"] = "Add-ons require at least one valid main service; main services have none"
			}
			if id != "" && input.Version < 1 {
				fields["version"] = "Refresh to obtain the current service version"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			sale, ok := readSale(w, input.Sale, input.SaleEnds)
			if !ok {
				return
			}
			var price *int64
			if input.Price != nil && string(input.Price) != "null" {
				var cents int64
				if err := json.Unmarshal(input.Price, &cents); err != nil {
					reply(w, 422, map[string]any{"errors": map[string]string{"priceCents": "Enter a price of 0 or more"}})
					return
				}
				price = &cents
			}
			service, err := store.SaveService(r.Context(), scope, postgres.CatalogService{GroupSet: input.Group != nil, BookingGroup: deref(input.Group), KeepPrice: input.Price == nil, SaleSet: sale.set, SalePriceCents: sale.cents, SaleEndsOn: sale.ends, ID: id, Name: input.Name, Kind: input.Kind, ParentID: input.ParentID, ParentIDs: input.ParentIDs, Version: input.Version, DurationMinutes: input.DurationMinutes, PriceCents: price})
			if err != nil {
				serviceFailure(w, err)
				return
			}
			status := 200
			if id == "" {
				status = 201
			}
			reply(w, status, service)
		})
	}
	mux.HandleFunc("DELETE "+base+"/{service}", func(w http.ResponseWriter, r *http.Request) {
		organization, location, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("service")
		if !uuid(organization) || !uuid(location) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid location and service selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, organization, location, "catalogue.manage")
		if err != nil {
			failure(w, err)
			return
		}
		version := 0
		if vStr := r.URL.Query().Get("version"); vStr != "" {
			if v, err := strconv.Atoi(vStr); err == nil && v > 0 {
				version = v
			}
		}
		res, err := store.DeleteService(r.Context(), scope, id, version)
		if err != nil {
			serviceFailure(w, err)
			return
		}
		reply(w, 200, res)
	})
}

type saleInput struct {
	set   bool
	cents *int64
	ends  *string
}

// readSale reads an optional sale from a service save: absent leaves the stored sale, null clears it, a number sets it with its
// optional last day (YYYY-MM-DD). It answers 422 and false for a bad value.
func readSale(w http.ResponseWriter, raw json.RawMessage, ends *string) (saleInput, bool) {
	if raw == nil {
		return saleInput{}, true
	}
	out := saleInput{set: true}
	if string(raw) == "null" {
		return out, true
	}
	var cents int64
	if err := json.Unmarshal(raw, &cents); err != nil || cents < 0 {
		reply(w, 422, map[string]any{"errors": map[string]string{"salePriceCents": "Enter a sale price of 0 or more"}})
		return out, false
	}
	out.cents = &cents
	if ends != nil && strings.TrimSpace(*ends) != "" {
		day := strings.TrimSpace(*ends)
		if _, err := time.Parse("2006-01-02", day); err != nil {
			reply(w, 422, map[string]any{"errors": map[string]string{"saleEndsOn": "Pick the sale's last day"}})
			return out, false
		}
		out.ends = &day
	}
	return out, true
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
