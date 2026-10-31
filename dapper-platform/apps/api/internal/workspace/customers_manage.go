package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Customer directory with edit, delete and a customer's records. Read: customers.read. Edit: customers.create. Delete: customer_accounts.manage.
func registerCustomerManageRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}"
	resolve := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid franchise selectors required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, action)
		if err != nil {
			failure(w, err)
			return postgres.Scope{}, false
		}
		return scope, true
	}
	fail := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, postgres.ErrCustomerNotFound):
			reply(w, 404, map[string]string{"error": "That customer was not found at this franchise."})
		case errors.Is(err, postgres.ErrCustomerInUse):
			reply(w, 409, map[string]string{"error": "This customer has bookings, work orders, invoices or other records, so it cannot be deleted. Edit the details instead."})
		case errors.Is(err, postgres.ErrCustomerDuplicate):
			reply(w, 409, map[string]any{"error": "Another customer already uses this email or phone number. Open that customer instead.", "errors": map[string]string{"email": "Already used by another customer", "phone": "Already used by another customer"}})
		case errors.Is(err, postgres.ErrCustomerInput):
			reply(w, 422, map[string]any{"errors": map[string]string{"customer": "Check the name, email and phone"}})
		default:
			failure(w, err)
		}
	}
	mux.HandleFunc("GET "+base+"/customer-directory", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "customers.read")
		if !ok {
			return
		}
		items, err := store.CustomerDirectory(r.Context(), scope)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 1000})
	})
	mux.HandleFunc("PUT "+base+"/customers/{customer}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "customers.create")
		if !ok {
			return
		}
		id := r.PathValue("customer")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		var input customerInput
		if !decode(w, r, &input) {
			return
		}
		input.Name, input.Email, input.Phone = strings.TrimSpace(input.Name), strings.TrimSpace(input.Email), strings.TrimSpace(input.Phone)
		fields := map[string]string{}
		if n := utf8.RuneCountInString(input.Name); n < 2 || n > 100 {
			fields["name"] = "Enter a name between 2 and 100 characters"
		}
		if input.Email != "" {
			if a, err := mail.ParseAddress(input.Email); err != nil || a.Address != input.Email || len(input.Email) > 254 {
				fields["email"] = "Enter a valid email address"
			}
		}
		if utf8.RuneCountInString(input.Phone) > 30 {
			fields["phone"] = "Phone number must be 30 characters or fewer"
		}
		if len(fields) > 0 {
			reply(w, 422, map[string]any{"errors": fields})
			return
		}
		c, err := store.UpdateCustomer(r.Context(), scope, id, input.Name, input.Email, input.Phone)
		if err != nil {
			fail(w, err)
			return
		}
		if input.Address != nil {
			if err = store.SetCustomerAddress(r.Context(), scope, id, *input.Address); err != nil {
				fail(w, err)
				return
			}
		}
		reply(w, 200, c)
	})
	mux.HandleFunc("DELETE "+base+"/customers/{customer}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "customer_accounts.manage")
		if !ok {
			return
		}
		id := r.PathValue("customer")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		result, err := store.DeleteCustomer(r.Context(), scope, id)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, result)
	})
	mux.HandleFunc("GET "+base+"/customers/{customer}/records", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "customers.read")
		if !ok {
			return
		}
		id := r.PathValue("customer")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid customer selector required"})
			return
		}
		records, err := store.CustomerRecordsFor(r.Context(), scope, id)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, records)
	})
}
