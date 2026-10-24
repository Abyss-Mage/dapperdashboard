package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func offerFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrBookingOffer) {
		reply(w, 422, map[string]string{"error": "Provide explicit equipment category, current rule, exact amounts, terms and scheduling settings; review is required to publish"})
		return
	}
	if errors.Is(err, postgres.ErrBookingOfferConflict) {
		reply(w, 409, map[string]string{"error": "Offer or scheduling rule changed, or this category/rule offer already exists. Reload and review before saving."})
		return
	}
	failure(w, err)
}
func portalOfferFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrBookingOffer) {
		reply(w, 422, map[string]string{"error": "Choose a valid appointment date or an explicit timestamp range of at most 26 hours"})
		return
	}
	if errors.Is(err, postgres.ErrBookingOfferConflict) {
		reply(w, 409, map[string]string{"error": "This service has changed or is unavailable for your equipment. Reload services before checking times."})
		return
	}
	if errors.Is(err, postgres.ErrForbidden) || errors.Is(err, postgres.ErrScope) {
		reply(w, 403, map[string]string{"error": "Your account cannot access this equipment"})
		return
	}
	failure(w, err)
}
func registerBookingOfferRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/vehicles/{vehicle}/booking-offers/{offer}/availability", func(w http.ResponseWriter, r *http.Request) {
		customer, vehicle, offer := r.PathValue("customer"), r.PathValue("vehicle"), r.PathValue("offer")
		if !uuid(customer) || !uuid(vehicle) || !uuid(offer) {
			reply(w, 400, map[string]string{"error": "Valid owned customer, equipment and offer selectors required"})
			return
		}
		version, e1 := strconv.Atoi(r.URL.Query().Get("offerVersion"))
		var from, to time.Time
		var e2, e3 error
		var days []string
		if day := r.URL.Query().Get("day"); day != "" {
			days = []string{day}
			if r.URL.Query().Get("from") != "" || r.URL.Query().Get("to") != "" {
				reply(w, 422, map[string]string{"error": "Choose a location date or an explicit timestamp range, not both"})
				return
			}
		} else {
			from, e2 = time.Parse(time.RFC3339Nano, r.URL.Query().Get("from"))
			to, e3 = time.Parse(time.RFC3339Nano, r.URL.Query().Get("to"))
		}
		if e1 != nil || e2 != nil || e3 != nil {
			reply(w, 422, map[string]string{"error": "Provide current offerVersion and explicit-offset from/to timestamps"})
			return
		}
		page, err := store.CustomerOfferAvailability(r.Context(), requestSession(r).principalID, customer, vehicle, offer, version, from, to, days...)
		if errors.Is(err, postgres.ErrAvailabilityLimit) {
			reply(w, 503, map[string]string{"error": "Availability cannot be calculated for this window. Please contact Dapper."})
			return
		}
		if err != nil {
			portalOfferFailure(w, err)
			return
		}
		reply(w, 200, page)
	})
	base := "/api/v1/organizations/{organization}/locations/{location}/booking-offers"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := offerScope(w, r, store)
		if !ok {
			return
		}
		items, err := store.BookingOffers(r.Context(), scope)
		if err != nil {
			offerFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
	mux.HandleFunc("GET "+base+"/options", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := offerScope(w, r, store)
		if !ok {
			return
		}
		options, err := store.BookingOfferChoices(r.Context(), scope)
		if err != nil {
			offerFailure(w, err)
			return
		}
		reply(w, 200, options)
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := offerScope(w, r, store)
		if !ok {
			return
		}
		var input struct {
			postgres.BookingOfferConfig
			ID              string `json:"id"`
			ExpectedVersion int    `json:"expectedVersion"`
			Reviewed        bool   `json:"reviewed"`
			Published       *bool  `json:"published"`
			LeadMinutes     *int   `json:"leadMinutes"`
			SubtotalCents   *int64 `json:"subtotalCents"`
			TaxCents        *int64 `json:"taxCents"`
			DepositCents    *int64 `json:"depositCents"`
		}
		if !decodeLimited(w, r, &input, 128*1024) {
			return
		}
		if input.Published == nil || input.LeadMinutes == nil || input.SubtotalCents == nil || input.TaxCents == nil || input.DepositCents == nil || !uuid(input.ResourceID) || !uuid(input.RuleID) || input.ID != "" && !uuid(input.ID) {
			offerFailure(w, postgres.ErrBookingOffer)
			return
		}
		c := input.BookingOfferConfig
		c.Published = *input.Published
		c.LeadMinutes = *input.LeadMinutes
		c.SubtotalCents = *input.SubtotalCents
		c.TaxCents = *input.TaxCents
		c.DepositCents = *input.DepositCents
		saved, err := store.SaveBookingOffer(r.Context(), scope, input.ID, input.ExpectedVersion, input.Reviewed, c)
		if err != nil {
			offerFailure(w, err)
			return
		}
		status := 200
		if input.ID == "" {
			status = 201
		}
		reply(w, status, saved)
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/vehicles/{vehicle}/booking-offers", func(w http.ResponseWriter, r *http.Request) {
		customer, vehicle := r.PathValue("customer"), r.PathValue("vehicle")
		if !uuid(customer) || !uuid(vehicle) {
			reply(w, 400, map[string]string{"error": "Valid owned customer and equipment selectors required"})
			return
		}
		items, err := store.CustomerBookingOffers(r.Context(), requestSession(r).principalID, customer, vehicle)
		if err != nil {
			portalOfferFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
}
func offerScope(w http.ResponseWriter, r *http.Request, store *postgres.Store) (postgres.Scope, bool) {
	org, loc := r.PathValue("organization"), r.PathValue("location")
	if !uuid(org) || !uuid(loc) {
		reply(w, 400, map[string]string{"error": "Valid scoped selectors required"})
		return postgres.Scope{}, false
	}
	scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "catalogue.manage")
	if err != nil {
		failure(w, err)
		return postgres.Scope{}, false
	}
	return scope, true
}
