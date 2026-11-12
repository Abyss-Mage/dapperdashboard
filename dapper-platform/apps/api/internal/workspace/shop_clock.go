package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

// shopClockOnly answers the old personal clock-in, break and clock-out addresses: time is now recorded on the shop iPad (owner decision 2026-11-11).
func shopClockOnly(w http.ResponseWriter) {
	reply(w, 409, map[string]string{"error": "Clock in, breaks and clock out are on the shop iPad: tap your name there and type your PIN.", "code": "shop_clock"})
}

// registerShopClockRoutes: the owner's PIN list and shop login for a franchise. The iPad's own routes are in shop_device.go.
func registerShopClockRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/shop-clock"
	scopeOf := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return postgres.Scope{}, false
		}
		return postgres.Scope{ActorID: requestSession(r).principalID, OrganizationID: org, LocationID: loc}, true
	}
	pinFailure := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrClockPinInput):
			reply(w, 422, map[string]string{"error": "Enter 6 digits, for someone who records time at this franchise."})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+base+"/staff", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		items, err := store.ClockStaffList(r.Context(), scope)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("PUT "+base+"/staff/{principal}/pin", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		principal := r.PathValue("principal")
		var input struct {
			Pin string `json:"pin"`
		}
		if !uuid(principal) || !decode(w, r, &input) {
			if !uuid(principal) {
				reply(w, 400, map[string]string{"error": "Valid person selector required"})
			}
			return
		}
		if err := store.SetClockPin(r.Context(), scope, principal, input.Pin); err != nil {
			pinFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/staff/{principal}/pin", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		principal := r.PathValue("principal")
		if !uuid(principal) {
			reply(w, 400, map[string]string{"error": "Valid person selector required"})
			return
		}
		if err := store.ClearClockPin(r.Context(), scope, principal); err != nil {
			pinFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})

	login := base + "/login"
	loginFailure := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrShopLoginInput):
			reply(w, 422, map[string]string{"error": "The username is 3 to 40 lowercase letters, numbers, dots, dashes or underscores; the password at least 10 characters."})
		case errors.Is(err, postgres.ErrShopUsernameTaken):
			reply(w, 409, map[string]string{"error": "That username is already used by another franchise. Choose another."})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+login, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		username, err := store.ShopLogin(r.Context(), scope)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]string{"username": username})
	})
	mux.HandleFunc("PUT "+login, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.SaveShopLogin(r.Context(), scope, input.Username, input.Password); err != nil {
			loginFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+login, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		if err := store.DeleteShopLogin(r.Context(), scope); err != nil {
			loginFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
}
