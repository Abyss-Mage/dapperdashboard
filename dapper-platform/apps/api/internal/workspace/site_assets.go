package workspace

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"dapper.local/platform/internal/postgres"
)

// registerSiteAssetRoutes: the owner replaces any picture or film of the website, or adds films, and can put the original back.
// The public side is read-only: the front door serves an overridden file from here, and the website asks for the list of added films.
func registerSiteAssetRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/site-assets"
	scopeFor := func(w http.ResponseWriter, r *http.Request) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "content.manage")
		if err != nil {
			failure(w, err)
			return scope, false
		}
		return scope, true
	}
	fail := func(w http.ResponseWriter, err error) {
		if errors.Is(err, postgres.ErrSiteAsset) {
			reply(w, 422, map[string]string{"error": "Upload a JPEG, PNG or WebP picture, or an MP4 or WebM film, of 25 MB or less."})
			return
		}
		failure(w, err)
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		items, err := store.SiteAssets(r.Context(), scope)
		if err != nil {
			fail(w, err)
			return
		}
		hidden, err := store.HiddenSiteAssets(r.Context(), scope)
		if err != nil {
			fail(w, err)
			return
		}
		sizes, err := store.LogoSizes(r.Context(), scope)
		if err != nil {
			fail(w, err)
			return
		}
		bin, err := store.BinnedSiteAssets(r.Context(), scope)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "hidden": hidden, "logoSizes": sizes, "bin": bin})
	})
	mux.HandleFunc("DELETE "+base+"/bin", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		if err := store.PurgeSiteAsset(r.Context(), scope, r.URL.Query().Get("path")); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	mux.HandleFunc("PUT "+base+"/logo-size", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		err := store.SetLogoSize(r.Context(), scope, r.URL.Query().Get("logo"), r.URL.Query().Get("size"))
		if errors.Is(err, postgres.ErrSiteAsset) {
			reply(w, 422, map[string]string{"error": "Choose small, medium, large or full."})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		// A film can be a few megabytes over a phone connection: give this upload more time than the usual request.
		rc := http.NewResponseController(w)
		_ = rc.SetReadDeadline(time.Now().Add(3 * time.Minute))
		_ = rc.SetWriteDeadline(time.Now().Add(3 * time.Minute))
		r.Body = http.MaxBytesReader(w, r.Body, 25<<20+1024)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			reply(w, 413, map[string]string{"error": "Upload a JPEG, PNG or WebP picture, or an MP4 or WebM film, of 25 MB or less."})
			return
		}
		if err = store.SaveSiteAsset(r.Context(), scope, r.URL.Query().Get("path"), data); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		if err := store.DeleteSiteAsset(r.Context(), scope, r.URL.Query().Get("path")); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})

	mux.HandleFunc("PUT "+base+"/hidden", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		if err := store.SetSiteAssetHidden(r.Context(), scope, r.URL.Query().Get("path"), true); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"hidden": true})
	})
	mux.HandleFunc("DELETE "+base+"/hidden", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := scopeFor(w, r)
		if !ok {
			return
		}
		if err := store.SetSiteAssetHidden(r.Context(), scope, r.URL.Query().Get("path"), false); err != nil {
			fail(w, err)
			return
		}
		reply(w, 200, map[string]bool{"hidden": false})
	})
	mux.HandleFunc("GET /api/v1/public/site-assets-hidden", func(w http.ResponseWriter, r *http.Request) {
		paths, err := store.PublicHiddenSiteAssets(r.Context())
		if err != nil {
			reply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		sizes, err := store.PublicLogoSizes(r.Context())
		if err != nil {
			reply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"paths": paths, "logoSizes": sizes})
	})
	mux.HandleFunc("GET /api/v1/public/site-asset", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		kind, at, err := store.PublicSiteAssetVersion(r.Context(), path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// The browser keeps the file for two minutes, then asks with the version it has; only a changed file is read from the database.
		etag := `"` + strconv.FormatInt(at.UnixNano(), 36) + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=120")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		data, err := store.PublicSiteAssetData(r.Context(), path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", kind)
		http.ServeContent(w, r, "", at, bytes.NewReader(data))
	})
	mux.HandleFunc("GET /api/v1/public/site-asset-paths", func(w http.ResponseWriter, r *http.Request) {
		paths, err := store.PublicSiteAssetPaths(r.Context())
		if err != nil {
			reply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		reply(w, 200, map[string]any{"paths": paths})
	})
	mux.HandleFunc("GET /api/v1/public/films", func(w http.ResponseWriter, r *http.Request) {
		films, err := store.PublicFilms(r.Context())
		if err != nil {
			reply(w, 503, map[string]string{"error": "unavailable"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, map[string]any{"items": films})
	})
}
