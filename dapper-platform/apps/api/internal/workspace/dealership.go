package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func dealershipFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrDealershipNotFound) || errors.Is(err, postgres.ErrDealershipIntakeNotFound) {
		reply(w, 404, map[string]string{"error": "Dealership record not found"})
		return
	}
	if errors.Is(err, postgres.ErrDealershipInput) {
		reply(w, 422, map[string]string{"error": "Check the input fields, formatting, and constraints"})
		return
	}
	if errors.Is(err, postgres.ErrDealershipCodeConflict) {
		reply(w, 409, map[string]string{"error": "Account code already in use for this location"})
		return
	}
	if errors.Is(err, postgres.ErrDealershipStockConflict) {
		reply(w, 409, map[string]string{"error": "Stock number already exists for this dealership"})
		return
	}
	if errors.Is(err, postgres.ErrDealershipCustomer) {
		reply(w, 404, map[string]string{"error": "Linked customer record not found"})
		return
	}
	failure(w, err)
}

func registerDealershipRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/dealerships"

	// GET list dealership accounts
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.read")
		if err != nil {
			failure(w, err)
			return
		}
		accounts, err := store.ListDealershipAccounts(r.Context(), scope)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": accounts, "limit": 200})
	})

	// POST create dealership account
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.CreateDealershipInput
		if !decodeLimited(w, r, &input, 32*1024) {
			return
		}
		account, err := store.CreateDealershipAccount(r.Context(), scope, input)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 201, account)
	})

	// GET single dealership account with tags
	mux.HandleFunc("GET "+base+"/{dealershipId}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.read")
		if err != nil {
			failure(w, err)
			return
		}
		account, err := store.GetDealershipAccount(r.Context(), scope, dealerID)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, account)
	})

	// PUT update dealership account
	mux.HandleFunc("PUT "+base+"/{dealershipId}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.UpdateDealershipInput
		if !decodeLimited(w, r, &input, 32*1024) {
			return
		}
		account, err := store.UpdateDealershipAccount(r.Context(), scope, dealerID, input)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, account)
	})

	// POST add tag
	mux.HandleFunc("POST "+base+"/{dealershipId}/tags", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			TagName  string `json:"tagName"`
			TagColor string `json:"tagColor"`
		}
		if !decodeLimited(w, r, &input, 8192) {
			return
		}
		tag, err := store.AddDealershipTag(r.Context(), scope, dealerID, input.TagName, input.TagColor)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 201, tag)
	})

	// DELETE tag
	mux.HandleFunc("DELETE "+base+"/{dealershipId}/tags/{tagId}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID, tagID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId"), r.PathValue("tagId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) || !uuid(tagID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		if err := store.RemoveDealershipTag(r.Context(), scope, dealerID, tagID); err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "removed"})
	})

	// GET list fleet intake records
	mux.HandleFunc("GET "+base+"/{dealershipId}/intake", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.read")
		if err != nil {
			failure(w, err)
			return
		}
		statusFilter := r.URL.Query().Get("status")
		items, err := store.ListFleetIntake(r.Context(), scope, dealerID, statusFilter)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 200})
	})

	// POST create fleet intake record
	mux.HandleFunc("POST "+base+"/{dealershipId}/intake", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.CreateFleetIntakeInput
		if !decodeLimited(w, r, &input, 16*1024) {
			return
		}
		intake, err := store.CreateFleetIntake(r.Context(), scope, dealerID, input)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 201, intake)
	})

	// PUT update fleet intake status
	mux.HandleFunc("PUT "+base+"/{dealershipId}/intake/{intakeId}/status", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID, intakeID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId"), r.PathValue("intakeId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) || !uuid(intakeID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			Status string `json:"status"`
			Notes  string `json:"notes,omitempty"`
		}
		if !decodeLimited(w, r, &input, 8192) {
			return
		}
		updated, err := store.UpdateFleetIntakeStatus(r.Context(), scope, intakeID, input.Status, input.Notes)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, updated)
	})

	// PUT assign fleet intake to employee
	mux.HandleFunc("PUT "+base+"/{dealershipId}/intake/{intakeId}/assign", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID, intakeID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId"), r.PathValue("intakeId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) || !uuid(intakeID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			AssignedPrincipalID *string `json:"assignedPrincipalId"`
		}
		if !decodeLimited(w, r, &input, 4096) {
			return
		}
		updated, err := store.AssignFleetIntake(r.Context(), scope, intakeID, input.AssignedPrincipalID)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, updated)
	})

	// GET dealership report
	mux.HandleFunc("GET "+base+"/{dealershipId}/report", func(w http.ResponseWriter, r *http.Request) {
		org, loc, dealerID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("dealershipId")
		if !uuid(org) || !uuid(loc) || !uuid(dealerID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "dealership.read")
		if err != nil {
			failure(w, err)
			return
		}
		report, err := store.GetDealershipReport(r.Context(), scope, dealerID)
		if err != nil {
			dealershipFailure(w, err)
			return
		}
		reply(w, 200, report)
	})
}
