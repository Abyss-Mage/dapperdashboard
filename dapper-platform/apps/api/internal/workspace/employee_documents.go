package workspace

import (
	"errors"
	"net/http"

	"dapper.local/platform/internal/postgres"
)

func documentFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrSOPDocumentNotFound):
		reply(w, 404, map[string]string{"error": "Document not found"})
	case errors.Is(err, postgres.ErrDocumentUnavailable):
		reply(w, 409, map[string]string{"error": "The document file is not available or no longer matches its registered checksum. Contact your administrator."})
	case errors.Is(err, postgres.ErrDocumentSignature):
		reply(w, 422, map[string]string{"error": "Type your full name to acknowledge"})
	case errors.Is(err, postgres.ErrDocumentNotAck):
		reply(w, 404, map[string]string{"error": "Acknowledge the document first to get a receipt"})
	default:
		employeeFailure(w, err)
	}
}

// registerEmployeeDocumentRoutes serves the organisation's registered documents to an assigned employee. The PDF is
// served only if it still matches its registered SHA-256; acknowledgments and receipts are the caller's own.
func registerEmployeeDocumentRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/employee/documents", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.EmployeeDocuments(r.Context(), requestSession(r).principalID)
		if err != nil {
			documentFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/v1/employee/documents/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("id")) {
			reply(w, 400, map[string]string{"error": "Valid document required"})
			return
		}
		data, name, err := store.EmployeeDocumentFile(r.Context(), requestSession(r).principalID, r.PathValue("id"))
		if err != nil {
			documentFailure(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	mux.HandleFunc("POST /api/v1/employee/documents/{id}/acknowledge", func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("id")) {
			reply(w, 400, map[string]string{"error": "Valid document required"})
			return
		}
		var input struct {
			SignedName string `json:"signedName"`
		}
		if !decode(w, r, &input) {
			return
		}
		doc, err := store.EmployeeAcknowledgeDocument(r.Context(), requestSession(r).principalID, r.PathValue("id"), input.SignedName)
		if err != nil {
			documentFailure(w, err)
			return
		}
		reply(w, 200, doc)
	})
	mux.HandleFunc("GET /api/v1/employee/documents/{id}/receipt", func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("id")) {
			reply(w, 400, map[string]string{"error": "Valid document required"})
			return
		}
		data, name, err := store.EmployeeDocumentReceipt(r.Context(), requestSession(r).principalID, r.PathValue("id"))
		if err != nil {
			documentFailure(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
}
