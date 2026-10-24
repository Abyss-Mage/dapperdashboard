package workspace

import (
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

func marketingFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrPopupNotFound) || errors.Is(err, postgres.ErrConsentNotFound) {
		reply(w, 404, map[string]string{"error": "Marketing record not found"})
		return
	}
	if errors.Is(err, postgres.ErrMarketingInput) {
		reply(w, 422, map[string]string{"error": "Check the input fields, formatting, and constraints"})
		return
	}
	if errors.Is(err, postgres.ErrDuplicatePopup) {
		reply(w, 409, map[string]string{"error": "Popup form name already exists"})
		return
	}
	failure(w, err)
}

func registerMarketingRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/marketing"

	// 1. Mailchimp settings
	mux.HandleFunc("GET "+base+"/settings", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.read")
		if err != nil {
			failure(w, err)
			return
		}
		settings, err := store.GetMailchimpSettings(r.Context(), scope)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, settings)
	})

	mux.HandleFunc("PUT "+base+"/settings", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.UpdateMailchimpSettingsInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			reply(w, 400, map[string]string{"error": "Invalid JSON body"})
			return
		}
		settings, err := store.UpdateMailchimpSettings(r.Context(), scope, input)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, settings)
	})

	// 2. Marketing consents
	mux.HandleFunc("GET "+base+"/consents", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.read")
		if err != nil {
			failure(w, err)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		consents, total, err := store.ListMarketingConsents(r.Context(), scope, limit, offset)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"consents": consents, "total": total})
	})

	mux.HandleFunc("POST "+base+"/consents", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.RecordConsentInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			reply(w, 400, map[string]string{"error": "Invalid JSON body"})
			return
		}
		if input.IPAddress == "" {
			input.IPAddress = r.RemoteAddr
		}
		if input.UserAgent == "" {
			input.UserAgent = r.UserAgent()
		}
		consent, err := store.RecordMarketingConsent(r.Context(), scope, input)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 201, consent)
	})

	// 3. Popup forms management
	mux.HandleFunc("GET "+base+"/popups", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.read")
		if err != nil {
			failure(w, err)
			return
		}
		popups, err := store.ListPopupForms(r.Context(), scope)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, popups)
	})

	mux.HandleFunc("POST "+base+"/popups", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.CreatePopupFormInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			reply(w, 400, map[string]string{"error": "Invalid JSON body"})
			return
		}
		form, err := store.CreatePopupForm(r.Context(), scope, input)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 201, form)
	})

	mux.HandleFunc("GET "+base+"/popups/{form}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, formID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("form")
		if !uuid(org) || !uuid(loc) || !uuid(formID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.read")
		if err != nil {
			failure(w, err)
			return
		}
		form, err := store.GetPopupForm(r.Context(), scope, formID)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, form)
	})

	mux.HandleFunc("PUT "+base+"/popups/{form}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, formID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("form")
		if !uuid(org) || !uuid(loc) || !uuid(formID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.UpdatePopupFormInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			reply(w, 400, map[string]string{"error": "Invalid JSON body"})
			return
		}
		form, err := store.UpdatePopupForm(r.Context(), scope, formID, input)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, form)
	})

	mux.HandleFunc("DELETE "+base+"/popups/{form}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, formID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("form")
		if !uuid(org) || !uuid(loc) || !uuid(formID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.manage")
		if err != nil {
			failure(w, err)
			return
		}
		if err := store.DeletePopupForm(r.Context(), scope, formID); err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("GET "+base+"/popups/{form}/submissions", func(w http.ResponseWriter, r *http.Request) {
		org, loc, formID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("form")
		if !uuid(org) || !uuid(loc) || !uuid(formID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "marketing.read")
		if err != nil {
			failure(w, err)
			return
		}
		submissions, err := store.ListPopupSubmissions(r.Context(), scope, formID)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, submissions)
	})

	// 4. Public popup retrieval and submission
	mux.HandleFunc("GET /api/v1/public/marketing/popups", func(w http.ResponseWriter, r *http.Request) {
		org := r.URL.Query().Get("organizationId")
		loc := r.URL.Query().Get("locationId")
		path := r.URL.Query().Get("path")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "organizationId and locationId are required"})
			return
		}
		popups, err := store.GetPublicActivePopupForms(r.Context(), org, loc, path)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 200, popups)
	})

	mux.HandleFunc("POST /api/v1/public/marketing/popups/{form}/submit", func(w http.ResponseWriter, r *http.Request) {
		formID := r.PathValue("form")
		if !uuid(formID) {
			reply(w, 400, map[string]string{"error": "Valid form ID required"})
			return
		}
		var input postgres.PublicPopupSubmissionInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			reply(w, 400, map[string]string{"error": "Invalid JSON body"})
			return
		}
		if input.IPAddress == "" {
			input.IPAddress = r.RemoteAddr
		}
		if input.UserAgent == "" {
			input.UserAgent = r.UserAgent()
		}
		submission, err := store.SubmitPublicPopup(r.Context(), formID, input)
		if err != nil {
			marketingFailure(w, err)
			return
		}
		reply(w, 201, submission)
	})
}
