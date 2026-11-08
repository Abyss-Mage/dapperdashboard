package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"io"
	"net/http"
)

func contentFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrContentInput):
		reply(w, 422, map[string]string{"error": "Check the category, text, image and selected service"})
	case errors.Is(err, postgres.ErrContentConflict):
		reply(w, 409, map[string]string{"error": "Content or service changed. Refresh before saving; your edits have not been applied"})
	case errors.Is(err, postgres.ErrContentNotFound):
		reply(w, 404, map[string]string{"error": "Published content unavailable"})
	default:
		failure(w, err)
	}
}
func registerPublicContentRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/public/sites/{site}", func(w http.ResponseWriter, r *http.Request) {
		data, err := store.PublicContent(r.Context(), r.PathValue("site"))
		if err != nil {
			contentFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, data)
	})
	mux.HandleFunc("GET /api/v1/public/portfolio", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.PublicPortfolio(r.Context())
		if err != nil {
			contentFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/v1/public/dealers", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.PublicDealers(r.Context())
		if err != nil {
			contentFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/v1/public/media/{media}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("media")
		if !uuid(id) {
			reply(w, 404, map[string]string{"error": "Image unavailable"})
			return
		}
		data, mime, err := store.PublicImage(r.Context(), id)
		if err != nil {
			contentFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(data)
	})
	base := "/api/v1/organizations/{organization}/locations/{location}/content"
	scopeFor := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location required"})
			return postgres.Scope{}, false
		}
		scope := postgres.Scope{OrganizationID: org, LocationID: loc, ActorID: requestSession(r).principalID}
		if action != "" {
			var err error
			scope, err = store.ResolveScope(r.Context(), scope.ActorID, org, loc, action)
			if err != nil {
				failure(w, err)
				return scope, false
			}
		}
		return scope, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "")
		if !ok {
			return
		}
		data, err := store.ContentForManagement(r.Context(), scope)
		if err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("GET "+base+"/source-images", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := scopeFor(w, r, "content.manage"); !ok {
			return
		}
		manifest, err := sourcePortfolioManifest()
		if err != nil {
			reply(w, 503, map[string]string{"error": "Source portfolio archive unavailable"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, manifest)
	})
	mux.HandleFunc("GET "+base+"/source-images/{image}", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := scopeFor(w, r, "content.manage"); !ok {
			return
		}
		data, err := sourcePortfolioImage(r.PathValue("image"))
		if err != nil {
			reply(w, 404, map[string]string{"error": "Source image unavailable"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(data)
	})
	mux.HandleFunc("GET "+base+"/media/{media}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		id := r.PathValue("media")
		if !uuid(id) {
			reply(w, 404, map[string]string{"error": "Image unavailable"})
			return
		}
		data, mime, err := store.ManagedContentImage(r.Context(), scope, id)
		if err != nil {
			contentFailure(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(data)
	})
	mux.HandleFunc("PUT "+base+"/site", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		var input postgres.PublicSite
		if !decode(w, r, &input) {
			return
		}
		data, err := store.SavePublicSite(r.Context(), scope, input)
		if err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("PUT "+base+"/services/{service}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "catalogue.manage")
		if !ok {
			return
		}
		id := r.PathValue("service")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid service required"})
			return
		}
		var input struct {
			Category       string `json:"category"`
			Summary        string `json:"summary"`
			Published      *bool  `json:"published"`
			Version        int    `json:"version"`
			ServiceVersion int    `json:"serviceVersion"`
		}
		if !decode(w, r, &input) {
			return
		}
		if input.Published == nil {
			reply(w, 422, map[string]string{"error": "Choose draft or published explicitly"})
			return
		}
		data, err := store.SaveServicePublication(r.Context(), scope, postgres.ServicePublication{ServiceID: id, Category: input.Category, Summary: input.Summary, Published: *input.Published, Version: input.Version, ServiceVersion: input.ServiceVersion})
		if err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("POST "+base+"/media", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			reply(w, 413, map[string]string{"error": "Upload a JPEG or PNG no larger than 6 MB"})
			return
		}
		id, err := store.UploadContentImage(r.Context(), scope, data)
		if errors.Is(err, postgres.ErrContentInput) {
			reply(w, 422, map[string]string{"error": "That file could not be used as a picture. Save it as a JPEG or PNG, under 6 MB, and try again."})
			return
		}
		if err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 201, map[string]string{"id": id})
	})
	mux.HandleFunc("PUT "+base+"/portfolio", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		var input postgres.PortfolioImage
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.MediaID) || (input.ID != "" && !uuid(input.ID)) {
			reply(w, 422, map[string]string{"error": "Select a valid uploaded image"})
			return
		}
		data, err := store.SavePortfolioImage(r.Context(), scope, input)
		if err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("DELETE "+base+"/portfolio/{image}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		id := r.PathValue("image")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Select a valid photo"})
			return
		}
		if err := store.DeletePortfolioImage(r.Context(), scope, id); err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	mux.HandleFunc("PUT "+base+"/dealers", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		var input postgres.WebsiteDealer
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.MediaID) || (input.ID != "" && !uuid(input.ID)) {
			reply(w, 422, map[string]string{"error": "Select a valid uploaded logo"})
			return
		}
		data, err := store.SaveWebsiteDealer(r.Context(), scope, input)
		if err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("DELETE "+base+"/dealers/{dealer}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		id := r.PathValue("dealer")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Select a valid dealer"})
			return
		}
		if err := store.DeleteWebsiteDealer(r.Context(), scope, id); err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	mux.HandleFunc("PUT "+base+"/dealers-order", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		var input struct {
			IDs []string `json:"ids"`
		}
		if !decode(w, r, &input) {
			return
		}
		for _, id := range input.IDs {
			if !uuid(id) {
				reply(w, 400, map[string]string{"error": "Select valid dealers"})
				return
			}
		}
		if err := store.OrderWebsiteDealers(r.Context(), scope, input.IDs); err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"ordered": true})
	})
	mux.HandleFunc("PUT "+base+"/portfolio-order", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r, "content.manage")
		if !ok {
			return
		}
		var input struct {
			IDs []string `json:"ids"`
		}
		if !decode(w, r, &input) {
			return
		}
		for _, id := range input.IDs {
			if !uuid(id) {
				reply(w, 400, map[string]string{"error": "Select valid photos"})
				return
			}
		}
		if err := store.OrderPortfolio(r.Context(), scope, input.IDs); err != nil {
			contentFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"ordered": true})
	})
}
