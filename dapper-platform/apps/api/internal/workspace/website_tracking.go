package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

func trackingFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrTrackingInput):
		reply(w, 422, map[string]string{"error": "Check the identifiers: Google Tag looks like GTM-XXXXXXX, the Meta Pixel is digits only, and the Mailchimp script must be an https address on chimpstatic.com or list-manage.com"})
	case errors.Is(err, postgres.ErrTrackingConflict):
		reply(w, 409, map[string]string{"error": "These settings were changed by someone else. Reload and try again."})
	default:
		failure(w, err)
	}
}

// registerWebsiteTrackingRoutes: administrators set the identifiers; the public site reads them to load its tags.
func registerWebsiteTrackingRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/website-tracking"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		t, err := store.GetWebsiteTracking(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			trackingFailure(w, err)
			return
		}
		reply(w, 200, t)
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return
		}
		var input struct {
			GTMID              string `json:"gtmId"`
			MetaPixelID        string `json:"metaPixelId"`
			MailchimpScriptURL string `json:"mailchimpScriptUrl"`
			Version            int    `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		t, err := store.SaveWebsiteTracking(r.Context(), requestSession(r).principalID, org, postgres.WebsiteTracking{GTMID: input.GTMID, MetaPixelID: input.MetaPixelID, MailchimpScriptURL: input.MailchimpScriptURL}, input.Version)
		if err != nil {
			trackingFailure(w, err)
			return
		}
		reply(w, 200, t)
	})
	mux.HandleFunc("GET /api/v1/public/tracking", func(w http.ResponseWriter, r *http.Request) {
		t, err := store.PublicWebsiteTracking(r.Context())
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]string{"gtmId": t.GTMID, "metaPixelId": t.MetaPixelID, "mailchimpScriptUrl": t.MailchimpScriptURL})
	})
}
