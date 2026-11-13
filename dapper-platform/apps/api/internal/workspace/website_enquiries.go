package workspace

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"dapper.local/platform/internal/postgres"
)

func enquiryFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrEnquiryInput):
		reply(w, 422, map[string]string{"error": "Enter your name, an email address or phone number, and keep the message under 3000 characters"})
	case errors.Is(err, postgres.ErrEnquiryNotFound):
		reply(w, 404, map[string]string{"error": "That enquiry no longer exists"})
	default:
		failure(w, err)
	}
}

// registerWebsiteEnquiryRoutes: the public contact form saves an enquiry; administrators with marketing access read and sort them.
func registerWebsiteEnquiryRoutes(mux *http.ServeMux, store *postgres.Store) {
	limiter := &resetLimiter{hits: map[[32]byte][]time.Time{}, max: 8, horizon: time.Hour}
	mux.HandleFunc("POST /api/v1/public/enquiries", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name    string `json:"name"`
			Email   string `json:"email"`
			Phone   string `json:"phone"`
			Vehicle string `json:"vehicle"`
			Service string `json:"service"`
			Message string `json:"message"`
			Website string `json:"website"` // hidden field: people leave it empty, form-filling bots do not
		}
		if !decode(w, r, &input) {
			return
		}
		if strings.TrimSpace(input.Website) != "" {
			reply(w, 201, map[string]bool{"received": true}) // pretend success, store nothing
			return
		}
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !limiter.allow("enquiry-ip:" + ip) {
			reply(w, 429, map[string]string{"error": "Too many messages from this connection. Please call us instead."})
			return
		}
		if _, err = store.CreateWebsiteEnquiry(r.Context(), postgres.WebsiteEnquiryInput{Name: input.Name, Email: input.Email, Phone: input.Phone, Vehicle: input.Vehicle, Service: input.Service, Message: input.Message}); err != nil {
			enquiryFailure(w, err)
			return
		}
		reply(w, 201, map[string]bool{"received": true})
	})
	base := "/api/v1/organizations/{organization}/website-enquiries"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		items, fresh, err := store.WebsiteEnquiries(r.Context(), requestSession(r).principalID, org, r.URL.Query().Get("status"))
		if err != nil {
			enquiryFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "new": fresh})
	})
	mux.HandleFunc("POST "+base+"/{enquiry}/status", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("enquiry")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		var input struct {
			Status string `json:"status"`
		}
		if !decode(w, r, &input) {
			return
		}
		if err := store.SetWebsiteEnquiryStatus(r.Context(), requestSession(r).principalID, org, id, input.Status); err != nil {
			enquiryFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	// error404 #108: delete an archived enquiry.
	mux.HandleFunc("DELETE "+base+"/{enquiry}", func(w http.ResponseWriter, r *http.Request) {
		org, id := r.PathValue("organization"), r.PathValue("enquiry")
		if !uuid(org) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		if err := store.DeleteWebsiteEnquiry(r.Context(), requestSession(r).principalID, org, id); err != nil {
			enquiryFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"deleted": true})
	})
}
