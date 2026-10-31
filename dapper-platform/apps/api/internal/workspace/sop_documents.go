package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"io"
	"net/http"
	"strings"
)

func sopFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrSOPDocumentNotFound) {
		reply(w, 404, map[string]string{"error": "SOP document not found"})
		return
	}
	if errors.Is(err, postgres.ErrSOPFileInput) {
		reply(w, 422, map[string]string{"error": "Add a PDF under 10 MB, give it a title of 3 to 150 characters and pick a category"})
		return
	}
	if errors.Is(err, postgres.ErrSOPStorageOff) {
		reply(w, 503, map[string]string{"error": "Document storage is not set up on this server"})
		return
	}
	if errors.Is(err, postgres.ErrDocumentUnavailable) {
		reply(w, 409, map[string]string{"error": "The document file is missing or no longer matches its checksum"})
		return
	}
	if errors.Is(err, postgres.ErrSOPHashMismatch) {
		reply(w, 409, map[string]string{"error": "Provided document hash does not match canonical document revision"})
		return
	}
	if errors.Is(err, postgres.ErrSOPDocumentInput) || errors.Is(err, postgres.ErrSOPAcknowledgmentInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"sop": "Provide valid training track, matching document hash and notes"}})
		return
	}
	failure(w, err)
}

func registerSOPRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/sops"

	// List SOP Documents
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "sop.read")
		if err != nil {
			failure(w, err)
			return
		}
		docs, err := store.ListSOPDocuments(r.Context(), scope)
		if err != nil {
			sopFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": docs})
	})

	// Add a document: the raw PDF is the request body; title and category travel in the query string.
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "sop.manage")
		if err != nil {
			failure(w, err)
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, postgres.MaxSOPFileBytes+1))
		if err != nil {
			reply(w, 413, map[string]string{"error": "The PDF must be under 10 MB"})
			return
		}
		doc, err := store.AddSOPDocument(r.Context(), scope, r.URL.Query().Get("title"), r.URL.Query().Get("category"), data)
		if err != nil {
			sopFailure(w, err)
			return
		}
		reply(w, 201, doc)
	})

	// Remove a document from the employee list (the record and file are kept as evidence).
	mux.HandleFunc("DELETE "+base+"/{sopId}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, sopID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("sopId")
		if !uuid(org) || !uuid(loc) || !uuid(sopID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "sop.manage")
		if err != nil {
			failure(w, err)
			return
		}
		if err = store.RemoveSOPDocument(r.Context(), scope, sopID); err != nil {
			sopFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})

	// View a document
	mux.HandleFunc("GET "+base+"/{sopId}/file", func(w http.ResponseWriter, r *http.Request) {
		org, loc, sopID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("sopId")
		if !uuid(org) || !uuid(loc) || !uuid(sopID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "sop.read")
		if err != nil {
			failure(w, err)
			return
		}
		data, title, err := store.SOPDocumentFile(r.Context(), scope, sopID)
		if err != nil {
			sopFailure(w, err)
			return
		}
		name := strings.Map(func(c rune) rune {
			if c < 32 || c > 126 || c == '"' || c == '\\' {
				return '-'
			}
			return c
		}, title) + ".pdf"
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	// List SOP Acknowledgments
	mux.HandleFunc("GET "+base+"/acknowledgments", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "sop.read")
		if err != nil {
			failure(w, err)
			return
		}
		empID := r.URL.Query().Get("employeeId")
		acks, err := store.ListSOPAcknowledgments(r.Context(), scope, empID)
		if err != nil {
			sopFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": acks})
	})

	// Acknowledge SOP
	mux.HandleFunc("POST "+base+"/{sopId}/acknowledge", func(w http.ResponseWriter, r *http.Request) {
		org, loc, sopID := r.PathValue("organization"), r.PathValue("location"), r.PathValue("sopId")
		if !uuid(org) || !uuid(loc) || !uuid(sopID) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "sop.acknowledge")
		if err != nil {
			failure(w, err)
			return
		}

		var input struct {
			TrainingTrack string `json:"trainingTrack"`
			Hash          string `json:"hash"`
			Notes         string `json:"notes"`
		}
		if !decode(w, r, &input) {
			return
		}

		track := strings.TrimSpace(input.TrainingTrack)
		switch track {
		case "orientation", "assisted", "independent", "certified":
		default:
			reply(w, 422, map[string]any{"errors": map[string]string{"trainingTrack": "Training track must be orientation, assisted, independent, or certified"}})
			return
		}

		hash := strings.TrimSpace(input.Hash)
		if len(hash) != 64 {
			reply(w, 422, map[string]any{"errors": map[string]string{"hash": "64-character SHA-256 hash required"}})
			return
		}

		ack, err := store.AcknowledgeSOP(r.Context(), scope, scope.ActorID, sopID, track, hash, input.Notes)
		if err != nil {
			sopFailure(w, err)
			return
		}
		reply(w, 201, ack)
	})
}
