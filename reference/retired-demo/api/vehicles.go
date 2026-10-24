package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Category is descriptive demo data, not an approved service/pricing category.
// Name supports equipment descriptions as well as automobile makes/models.
type Vehicle struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Identifier string `json:"identifier"`
	Location   string `json:"location"`
}

func (s *Store) persistLocked() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(s.path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(s.path+".tmp", s.path)
}

func (s *Store) createVehicle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "http://127.0.0.1:3006" {
		respond(w, 403, map[string]string{"error": "Same-origin request required"})
		return
	}
	var input struct {
		CustomerID string `json:"customerId"`
		Name       string `json:"name"`
		Category   string `json:"category"`
		Identifier string `json:"identifier"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respond(w, 400, map[string]string{"error": "Invalid vehicle data"})
		return
	}
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(input.Category)
	input.Identifier = strings.TrimSpace(input.Identifier)
	errors := map[string]string{}
	if len(input.Name) < 2 || len(input.Name) > 120 {
		errors["name"] = "Enter a vehicle or equipment description between 2 and 120 characters"
	}
	if len(input.Category) > 60 {
		errors["category"] = "Category must be 60 characters or fewer"
	}
	if len(input.Identifier) > 60 {
		errors["identifier"] = "Identifier must be 60 characters or fewer"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var customer *Customer
	for i := range s.Customers {
		if s.Customers[i].ID == input.CustomerID {
			customer = &s.Customers[i]
			break
		}
	}
	if customer == nil {
		errors["customerId"] = "Choose an existing demo customer"
	}
	if len(errors) > 0 {
		respond(w, 422, map[string]any{"errors": errors})
		return
	}
	for _, v := range s.Vehicles {
		if input.Identifier != "" && v.CustomerID == input.CustomerID && strings.EqualFold(v.Identifier, input.Identifier) {
			respond(w, 409, map[string]any{"errors": map[string]string{"identifier": "This identifier already exists for this demo customer"}})
			return
		}
	}
	v := Vehicle{ID: fmt.Sprintf("v%d", time.Now().UnixNano()), CustomerID: customer.ID, Name: input.Name, Category: input.Category, Identifier: input.Identifier, Location: customer.Location}
	s.Vehicles = append(s.Vehicles, v)
	if err := s.persistLocked(); err != nil {
		s.Vehicles = s.Vehicles[:len(s.Vehicles)-1]
		log.Printf("demo vehicle persistence failed: %v", err)
		respond(w, 500, map[string]string{"error": "Could not save the demo vehicle"})
		return
	}
	respond(w, 201, v)
}
