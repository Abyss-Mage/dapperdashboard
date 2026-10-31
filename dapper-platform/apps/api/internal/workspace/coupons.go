package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

func couponFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.As(err, new(postgres.CouponInvalid)):
		reply(w, 422, map[string]string{"error": err.Error()})
	case errors.Is(err, postgres.ErrCouponInput):
		reply(w, 422, map[string]string{"error": err.Error()})
	case errors.Is(err, postgres.ErrCouponCode):
		reply(w, 409, map[string]string{"error": "Another coupon already uses this code. Choose a different code."})
	case errors.Is(err, postgres.ErrCouponInUse):
		reply(w, 409, map[string]string{"error": "This coupon has been used, so it cannot be deleted. Switch it off instead."})
	case errors.Is(err, postgres.ErrCouponNotFound):
		reply(w, 404, map[string]string{"error": "That coupon no longer exists, or it was changed by someone else. Reload and try again."})
	case errors.Is(err, postgres.ErrInvoiceStatusConflict):
		reply(w, 409, map[string]string{"error": "Only an invoice that has not been issued can take a coupon."})
	default:
		failure(w, err)
	}
}

// registerCouponRoutes: a franchise's discount coupons, and putting one on (or taking one off) a draft invoice.
func registerCouponRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/coupons"
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
		items, err := store.Coupons(r.Context(), scope)
		if err != nil {
			couponFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	save := func(create bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			scope, ok := scopeOf(w, r)
			if !ok {
				return
			}
			var input struct {
				postgres.CouponInput
				Version int `json:"version"`
			}
			if !decode(w, r, &input) {
				return
			}
			id := ""
			if !create {
				if id = r.PathValue("coupon"); !uuid(id) {
					reply(w, 400, map[string]string{"error": "Valid coupon selector required"})
					return
				}
			}
			saved, err := store.SaveCoupon(r.Context(), scope, id, input.CouponInput, input.Version)
			if err != nil {
				couponFailure(w, err)
				return
			}
			reply(w, 200, map[string]string{"id": saved})
		}
	}
	mux.HandleFunc("POST "+base, save(true))
	mux.HandleFunc("PUT "+base+"/{coupon}", save(false))
	mux.HandleFunc("DELETE "+base+"/{coupon}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		id := r.PathValue("coupon")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid coupon selector required"})
			return
		}
		if err := store.DeleteCoupon(r.Context(), scope, id); err != nil {
			couponFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})

	invoiceBase := "/api/v1/organizations/{organization}/locations/{location}/invoices/{invoice}/coupon"
	mux.HandleFunc("POST "+invoiceBase, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		inv := r.PathValue("invoice")
		var input struct {
			Code string `json:"code"`
		}
		if !uuid(inv) || !decode(w, r, &input) {
			if !uuid(inv) {
				reply(w, 400, map[string]string{"error": "Valid invoice selector required"})
			}
			return
		}
		out, err := store.ApplyCouponToInvoice(r.Context(), scope, inv, input.Code)
		if err != nil {
			couponFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("DELETE "+invoiceBase, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeOf(w, r)
		if !ok {
			return
		}
		inv := r.PathValue("invoice")
		if !uuid(inv) {
			reply(w, 400, map[string]string{"error": "Valid invoice selector required"})
			return
		}
		out, err := store.RemoveCouponFromInvoice(r.Context(), scope, inv)
		if err != nil {
			couponFailure(w, err)
			return
		}
		reply(w, 200, out)
	})
}
