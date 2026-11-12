package workspace

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"dapper.local/platform/internal/postgres"
	"github.com/jackc/pgx/v5/pgconn"
)

var franchiseCode = regexp.MustCompile(`^[A-Z0-9-]{2,12}$`)

func structureFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrFranchiseAddress):
		reply(w, 422, map[string]any{"errors": map[string]string{"address": "Enter the full address: street, city, province or state, postal code and country"}})
		return
	case errors.Is(err, postgres.ErrStructureInput):
		reply(w, 422, map[string]any{"errors": map[string]string{"franchise": "Check the franchise code, name, address and contact people"}})
		return
	case errors.Is(err, postgres.ErrFranchiseCode):
		reply(w, 409, map[string]any{"errors": map[string]string{"code": "This franchise code is already used. Choose a different one"}})
		return
	case errors.Is(err, postgres.ErrFranchiseInUse):
		reply(w, 409, map[string]string{"error": "Cannot delete a franchise that has bookings, customers, invoices, attendance, uploads or role assignments"})
		return
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		reply(w, 409, map[string]any{"errors": map[string]string{"name": "A franchise with this name already exists"}})
		return
	}
	failure(w, err)
}

// franchiseInput validates the request body. A franchise is one site: a code (the identifier), a name, and an
// optional address. There is no separate location to create.
func franchiseInput(w http.ResponseWriter, r *http.Request) (postgres.FranchiseInput, bool) {
	var input struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Address struct {
			Street     string `json:"street"`
			City       string `json:"city"`
			Region     string `json:"region"`
			PostalCode string `json:"postalCode"`
			Country    string `json:"country"`
		} `json:"address"`
		Contacts        []postgres.FranchiseContact `json:"contacts"`
		TaxRegistration *string                     `json:"taxRegistration"`
		Ownership       *string                     `json:"ownership"`
	}
	if !decode(w, r, &input) {
		return postgres.FranchiseInput{}, false
	}
	fields := map[string]string{}
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if !franchiseCode.MatchString(code) {
		fields["code"] = "Enter a code of 2 to 12 letters, numbers or hyphens"
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(input.Name)); n < 2 || n > 120 {
		fields["name"] = "Enter a name between 2 and 120 characters"
	}
	a := input.Address
	if utf8.RuneCountInString(strings.TrimSpace(a.Street)) > 200 || utf8.RuneCountInString(strings.TrimSpace(a.City)) > 100 || utf8.RuneCountInString(strings.TrimSpace(a.Region)) > 100 ||
		utf8.RuneCountInString(strings.TrimSpace(a.PostalCode)) > 20 || utf8.RuneCountInString(strings.TrimSpace(a.Country)) > 100 {
		fields["address"] = "One of the address fields is too long"
	}
	if len(fields) > 0 {
		reply(w, 422, map[string]any{"errors": fields})
		return postgres.FranchiseInput{}, false
	}
	return postgres.FranchiseInput{Code: code, Name: input.Name, Address: postgres.Address{Street: a.Street, City: a.City, Region: a.Region, PostalCode: a.PostalCode, Country: a.Country}, Contacts: input.Contacts, TaxRegistration: input.TaxRegistration, Ownership: input.Ownership}, true
}

func registerStructureRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/"
	mux.HandleFunc("GET "+base+"structure", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization required"})
			return
		}
		franchises, err := store.Structure(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			structureFailure(w, err)
			return
		}
		// "locations" is a read-only compatibility view for screens that still list sites; it is the same franchises.
		sites := make([]postgres.Location, 0, len(franchises))
		for _, f := range franchises {
			sites = append(sites, postgres.Location{ID: f.ID, Name: f.Name, Code: f.Code, FranchiseID: f.FranchiseID})
		}
		reply(w, 200, map[string]any{"franchises": franchises, "locations": sites, "limit": 100})
	})
	mux.HandleFunc("POST "+base+"franchises", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization required"})
			return
		}
		input, ok := franchiseInput(w, r)
		if !ok {
			return
		}
		franchise, err := store.CreateFranchise(r.Context(), requestSession(r).principalID, org, input)
		if err != nil {
			structureFailure(w, err)
			return
		}
		reply(w, 201, franchise)
	})
	mux.HandleFunc("PUT "+base+"franchises/{franchise}", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("franchise")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid organization and franchise required"})
			return
		}
		input, ok := franchiseInput(w, r)
		if !ok {
			return
		}
		franchise, err := store.UpdateFranchise(r.Context(), requestSession(r).principalID, org, id, input)
		if err != nil {
			structureFailure(w, err)
			return
		}
		reply(w, 200, franchise)
	})
	mux.HandleFunc("DELETE "+base+"franchises/{franchise}", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("franchise")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid organization and franchise required"})
			return
		}
		res, err := store.DeleteFranchise(r.Context(), requestSession(r).principalID, org, id)
		if err != nil {
			structureFailure(w, err)
			return
		}
		reply(w, 200, res)
	})
}
