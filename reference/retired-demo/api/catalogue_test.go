package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

func TestReferenceCatalogueBoundaries(t *testing.T) {
	s, err := newStore(filepath.Join(t.TempDir(), "demo.json"))
	if err != nil {
		t.Fatal(err)
	}
	request := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/catalogue/reference", nil)
		r.Host = "127.0.0.1:8120"
		if auth {
			r.AddCookie(&http.Cookie{Name: "dapper_demo", Value: s.token})
		}
		w := httptest.NewRecorder()
		s.handler().ServeHTTP(w, r)
		return w
	}
	if request(false).Code != 401 {
		t.Fatal("catalogue bypassed demo session")
	}
	w := request(true)
	var body struct {
		ApprovalStatus     string `json:"approvalStatus"`
		TransactionEnabled bool   `json:"transactionEnabled"`
		Pages              []struct {
			Source string `json:"source"`
			Hash   string `json:"sourceSha256"`
		} `json:"pages"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatal("invalid catalogue response")
	}
	if body.TransactionEnabled || body.ApprovalStatus != "reference_only_pending_client_approval" {
		t.Fatal("reference data presented as approved")
	}
	if len(body.Pages) == 0 {
		t.Fatal("empty catalogue")
	}
	for _, page := range body.Pages {
		u, err := url.Parse(page.Source)
		if err != nil || u.Scheme != "https" || u.Host != "dapperdetailing.ca" || len(page.Hash) != 64 {
			t.Fatal("missing trusted source provenance")
		}
	}
}

func TestBookingpressCatalogueIsReadOnlyAndSourceLinked(t *testing.T) {
	s, err := newStore(filepath.Join(t.TempDir(), "demo.json"))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/catalogue/bookingpress", nil)
		r.Host = "127.0.0.1:8120"
		if auth {
			r.AddCookie(&http.Cookie{Name: "dapper_demo", Value: s.token})
		}
		w := httptest.NewRecorder()
		s.handler().ServeHTTP(w, r)
		return w
	}
	if request("GET", false).Code != 401 {
		t.Fatal("catalogue session bypass")
	}
	if request("POST", true).Code != 405 {
		t.Fatal("catalogue mutation accepted")
	}
	var body struct {
		ApprovalStatus     string `json:"approvalStatus"`
		TransactionEnabled bool   `json:"transactionEnabled"`
		SourceSha256       string `json:"sourceSha256"`
		Categories         []struct {
			SourceID string `json:"sourceId"`
		} `json:"categories"`
		Services []struct {
			SourceID   string `json:"sourceId"`
			CategoryID string `json:"categorySourceId"`
			PriceRaw   string `json:"priceRaw"`
		} `json:"services"`
		Extras []struct {
			SourceID              string   `json:"sourceId"`
			ParentServiceSourceID string   `json:"parentServiceSourceId"`
			PriceRaw              string   `json:"priceRaw"`
			DurationRaw           string   `json:"durationRaw"`
			ReviewFlags           []string `json:"reviewFlags"`
		} `json:"extras"`
	}
	w := request("GET", true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatal("invalid catalogue response")
	}
	if body.TransactionEnabled || body.ApprovalStatus != "reference_only_pending_client_approval" || len(body.SourceSha256) != 64 {
		t.Fatal("approval/provenance boundary lost")
	}
	categories := map[string]bool{}
	for _, c := range body.Categories {
		categories[c.SourceID] = true
	}
	seen := map[string]bool{}
	for _, service := range body.Services {
		if service.SourceID == "" || seen[service.SourceID] || !categories[service.CategoryID] || service.PriceRaw == "" {
			t.Fatal("source identity/category/decimal value lost")
		}
		seen[service.SourceID] = true
	}
	if len(seen) == 0 {
		t.Fatal("empty source catalogue")
	}
	seenExtras := map[string]bool{}
	for _, extra := range body.Extras {
		if extra.SourceID == "" || seenExtras[extra.SourceID] || !seen[extra.ParentServiceSourceID] || extra.PriceRaw == "" {
			t.Fatal("extra identity/parent/price value lost")
		}
		seenExtras[extra.SourceID] = true
	}
	if len(seenExtras) == 0 {
		t.Fatal("empty source extras")
	}
}
