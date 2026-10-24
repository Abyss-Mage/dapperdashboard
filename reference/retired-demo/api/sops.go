package main

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"net/http"
)

// User-provided operational references, not signed employee records.
//
//go:embed data/sops/*.pdf
var sopFiles embed.FS

type sopRecord struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	SHA256 string `json:"sha256"`
	Pages  int    `json:"pages"`
}

func sopRecords() []sopRecord {
	records := []sopRecord{{ID: "onboarding_policy", Title: "Onboarding Policy", Pages: 6}, {ID: "interior_detail_sop", Title: "Interior Detail SOP", Pages: 7}, {ID: "full_body_detail_sop", Title: "Full Body Detail SOP", Pages: 6}, {ID: "ceramic_coating_sop", Title: "Ceramic Coating SOP", Pages: 5}, {ID: "addon_services_sop", Title: "Add-on Services SOP", Pages: 6}}
	for i := range records {
		data, _ := sopFiles.ReadFile("data/sops/" + records[i].ID + ".pdf")
		records[i].SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		records[i].Date = "2026-04-03"
	}
	return records
}
func listSOPs(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]any{"documents": sopRecords(), "source": "user-supplied-reference", "acknowledgmentsEnabled": false})
}
func readSOP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found := false
	for _, record := range sopRecords() {
		if record.ID == id {
			found = true
		}
	}
	if !found {
		respond(w, 404, map[string]string{"error": "Document not found"})
		return
	}
	data, err := sopFiles.ReadFile("data/sops/" + id + ".pdf")
	if err != nil {
		respond(w, 503, map[string]string{"error": "Document unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `inline; filename="`+id+`.pdf"`)
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
