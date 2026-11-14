package workspace

import (
	"errors"
	"log"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

// Owner-only permanent deletes of a dealership invoice (error404 #124) and a dealership (#116). The invoice number or the dealership's
// code must be typed to confirm. Organization access.manage is checked in the store.
func registerPurgeRoutes(mux *http.ServeMux, store *postgres.Store) {
	loc := "/api/v1/organizations/{organization}/locations/{location}"
	handle := func(path, param, what string, run func(*http.Request, postgres.Scope, string, string) error) {
		mux.HandleFunc("POST "+loc+path, func(w http.ResponseWriter, r *http.Request) {
			org, l, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue(param)
			if !uuid(org) || !uuid(l) || !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid selectors required"})
				return
			}
			var in struct {
				Confirm string `json:"confirm"`
			}
			if !decode(w, r, &in) {
				return
			}
			err := run(r, postgres.Scope{OrganizationID: org, LocationID: l, ActorID: requestSession(r).principalID}, id, in.Confirm)
			switch {
			case err == nil:
				reply(w, 200, map[string]any{"purged": true})
			case errors.Is(err, postgres.ErrPurgeConfirm):
				reply(w, 422, map[string]string{"error": "That does not match. Type it exactly as shown."})
			case errors.Is(err, postgres.ErrPurgeSquare):
				reply(w, 409, map[string]string{"error": "A Square card payment was attempted or taken for this invoice. Card history has to match Square, so it can't be deleted permanently; void it instead."})
			case errors.Is(err, postgres.ErrDealershipInUse):
				reply(w, 409, map[string]string{"error": "This dealership still has invoices, staff assigned to it or staff photos. Delete its invoices and move its staff first, or switch it off instead."})
			case errors.Is(err, postgres.ErrForbidden):
				reply(w, 403, map[string]string{"error": "Only the owner and administrators can delete permanently."})
			case errors.Is(err, postgres.ErrInvoiceNotFound), errors.Is(err, postgres.ErrDealershipNotFound):
				reply(w, 404, map[string]string{"error": what + " not found"})
			default:
				log.Printf("Permanent delete of %s %s failed: %v", what, id, err)
				reply(w, 500, map[string]string{"error": "It could not be deleted permanently; nothing was removed. " + err.Error()})
			}
		})
	}
	handle("/dealer-invoices/{invoice}/purge", "invoice", "Invoice", func(r *http.Request, s postgres.Scope, id, c string) error {
		return store.PurgeDealerInvoice(r.Context(), s, id, c)
	})
	handle("/dealerships/{dealershipId}/purge", "dealershipId", "Dealership", func(r *http.Request, s postgres.Scope, id, c string) error {
		return store.PurgeDealership(r.Context(), s, id, c)
	})
}
