package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDemoOperationsRelationshipsAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.json")
	s, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	h := s.handler()
	send := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/demo/operations", strings.NewReader(body))
		r.Host = "127.0.0.1:8120"
		r.Header.Set("Origin", "http://127.0.0.1:3006")
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "dapper_demo", Value: s.token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if send(`{"action":"service.save","name":"Demo add-on","kind":"addon","parentId":"missing"}`).Code != 422 {
		t.Fatal("orphan addon accepted")
	}
	w := send(`{"action":"service.save","name":"Demo main","kind":"main"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result DemoOperations
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	parent := result.Services[0].ID
	body, _ := json.Marshal(operationInput{Action: "service.save", Name: "Demo add-on", Kind: "addon", ParentID: parent})
	if send(string(body)).Code != 200 {
		t.Fatal("valid addon denied")
	}
	body, _ = json.Marshal(operationInput{Action: "service.archive", ID: parent})
	if send(string(body)).Code != 422 {
		t.Fatal("main archived with active addon")
	}
	if send(`{"action":"location.add","name":"Demo location","franchiseId":"unknown"}`).Code != 422 {
		t.Fatal("orphan location accepted")
	}
	if send(`{"action":"job.complete","id":"DD-1004"}`).Code != 422 {
		t.Fatal("quote enquiry completed")
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- send(`{"action":"job.complete","id":"DD-1002"}`).Code }()
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		if code == 200 {
			success++
		}
		if code == 409 {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent completion was not exclusive")
	}
	restored, err := newStore(path)
	if err != nil || len(restored.Operations.Services) != 2 || len(restored.Operations.Completions) != 1 {
		t.Fatal("operations not persisted", err)
	}
	if restored.Bookings[1].Status != "Completed" {
		t.Fatal("completed job status was not persisted")
	}
	if send(`{"action":"job.complete","id":"DD-1003","performer":"forged"}`).Code != 400 {
		t.Fatal("client actor accepted")
	}
}
