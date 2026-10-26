package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"sync"
	"time"
)

type Customer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Location string `json:"location"`
}
type Booking struct {
	ID       string `json:"id"`
	Customer string `json:"customer"`
	Vehicle  string `json:"vehicle"`
	Service  string `json:"service"`
	Time     string `json:"time"`
	Status   string `json:"status"`
	Employee string `json:"employee"`
	Location string `json:"location"`
}
type Store struct {
	mu         sync.Mutex
	Customers  []Customer     `json:"customers"`
	Bookings   []Booking      `json:"bookings"`
	Vehicles   []Vehicle      `json:"vehicles"`
	Operations DemoOperations `json:"operations"`
	path       string
	token      string
}

func newStore(path string) (*Store, error) {
	s := &Store{path: path}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	s.token = hex.EncodeToString(token)
	s.Vehicles = []Vehicle{{ID: "v1", CustomerID: "c1", Name: "2022 Toyota RAV4", Category: "SUV", Location: "peterborough"}, {ID: "v2", CustomerID: "c2", Name: "2021 Honda Civic", Category: "Sedan", Location: "peterborough"}, {ID: "v3", CustomerID: "c3", Name: "2023 Ford F-150", Category: "Pickup", Location: "peterborough"}}
	s.Customers = []Customer{{"c1", "Alex Morgan", "alex@example.test", "", "peterborough"}, {"c2", "Jordan Blake", "jordan@example.test", "", "peterborough"}, {"c3", "Taylor Brooks", "taylor@example.test", "", "peterborough"}}
	s.Bookings = []Booking{{"DD-1001", "Alex Morgan", "2022 Toyota RAV4", "Luxury Full Body", "09:00", "In Progress", "Demo detailer A", "peterborough"}, {"DD-1002", "Jordan Blake", "2021 Honda Civic", "VIP Interior Detail", "10:30", "Scheduled", "Demo detailer B", "peterborough"}, {"DD-1003", "Taylor Brooks", "2023 Ford F-150", "Full Exterior Wash", "13:00", "Scheduled", "Unassigned", "peterborough"}, {"DD-1004", "Alex Morgan", "2022 Toyota RAV4", "Ceramic Coating", "15:00", "Awaiting Quote", "Unassigned", "peterborough"}}
	if b, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(b, s); err != nil {
			return nil, fmt.Errorf("invalid demo store: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// Reconcile earlier demo completion records into the booking projection.
	for _, completion := range s.Operations.Completions {
		for i := range s.Bookings {
			if s.Bookings[i].ID == completion.JobID {
				s.Bookings[i].Status = "Completed"
				s.Bookings[i].Employee = ""
			}
		}
	}
	return s, nil
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Store) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/demo/sops", listSOPs)
	mux.HandleFunc("GET /api/v1/demo/sops/{id}", readSOP)
	mux.HandleFunc("GET /api/v1/demo/operations", s.operations)
	mux.HandleFunc("POST /api/v1/demo/operations", s.operations)
	mux.HandleFunc("GET /api/v1/catalogue/bookingpress", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, bookingpressCatalogue)
	})
	mux.HandleFunc("POST /api/v1/vehicles", s.createVehicle)
	mux.HandleFunc("GET /api/v1/catalogue/reference", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, referenceCatalogue)
	})
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "mode": "local-demo", "productionReady": false})
	})
	mux.HandleFunc("GET /api/v1/demo/session", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "dapper_demo", Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		respond(w, 200, map[string]any{"mode": "synthetic-demo", "name": "Demo administrator"})
	})
	mux.HandleFunc("/api/v1/workspace", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			respond(w, 405, map[string]string{"error": "Method not allowed"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		respond(w, 200, map[string]any{"customers": s.Customers, "vehicles": s.Vehicles, "bookings": s.Bookings, "mode": "synthetic-demo", "location": map[string]string{"id": "peterborough", "name": "Peterborough · The Kingsway", "address": "660 The Kingsway, Peterborough, ON K9J 7C8", "phone": "+1 (705) 977-6217", "email": "info@dapperdetailing.ca", "source": "https://dapperdetailing.ca/contact-us/"}, "services": []string{"Ultimate Protection", "Luxury Full Body", "VIP Interior Detail", "Full Exterior Wash", "Ceramic Coating", "Paint Protection Film", "Aircraft Detailing", "Marine Detailing"}})
	})
	mux.HandleFunc("POST /api/v1/customers", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "http://127.0.0.1:3006" {
			respond(w, 403, map[string]string{"error": "Same-origin request required"})
			return
		}
		var input struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Phone string `json:"phone"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			respond(w, 400, map[string]string{"error": "Invalid customer data"})
			return
		}
		c := Customer{Name: input.Name, Email: input.Email, Phone: input.Phone}
		c.Name = strings.TrimSpace(c.Name)
		c.Email = strings.TrimSpace(c.Email)
		errors := map[string]string{}
		if len(c.Name) < 2 || len(c.Name) > 100 {
			errors["name"] = "Enter a name between 2 and 100 characters"
		}
		if address, err := mail.ParseAddress(c.Email); err != nil || address.Address != c.Email || len(c.Email) > 254 {
			errors["email"] = "Enter a valid email address"
		}
		if len(c.Phone) > 30 {
			errors["phone"] = "Phone number must be 30 characters or fewer"
		}
		if len(errors) > 0 {
			respond(w, 422, map[string]any{"errors": errors})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, existing := range s.Customers {
			if strings.EqualFold(existing.Email, c.Email) {
				respond(w, 409, map[string]any{"errors": map[string]string{"email": "This email already exists in the demo workspace"}})
				return
			}
		}
		c.ID = fmt.Sprintf("c%d", time.Now().UnixNano())
		c.Location = "peterborough"
		s.Customers = append(s.Customers, c)
		data, err := json.MarshalIndent(s, "", "  ")
		if err == nil {
			err = os.WriteFile(s.path+".tmp", data, 0600)
		}
		if err == nil {
			err = os.Rename(s.path+".tmp", s.path)
		}
		if err != nil {
			s.Customers = s.Customers[:len(s.Customers)-1]
			log.Printf("demo persistence failed: %v", err)
			respond(w, 500, map[string]string{"error": "Could not save the demo customer"})
			return
		}
		respond(w, 201, c)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || host != "127.0.0.1" {
			respond(w, 403, map[string]string{"error": "Loopback host required"})
			return
		}
		if r.URL.Path != "/api/v1/health" && r.URL.Path != "/api/v1/demo/session" {
			c, err := r.Cookie("dapper_demo")
			if err != nil || c.Value != s.token {
				respond(w, 401, map[string]string{"error": "Demo session required"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func main() {
	if os.Getenv("DAPPER_DEMO") != "1" {
		log.Fatal("Set DAPPER_DEMO=1 to run this local-only synthetic demo; production authentication is not implemented")
	}
	s, err := newStore("demo-store.json")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: "127.0.0.1:8120", Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Print("Dapper local demo API: http://127.0.0.1:8120")
	log.Fatal(server.ListenAndServe())
}
