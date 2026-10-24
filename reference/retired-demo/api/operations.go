package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Demo-only workflow exploration. Production owner/action grants are separate.
type ManagedService struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	ParentID string `json:"parentId"`
	Archived bool   `json:"archived"`
}
type DemoPlace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	FranchiseID string `json:"franchiseId"`
}
type DemoJobEvent struct {
	JobID       string `json:"jobId"`
	Performer   string `json:"performer"`
	CompletedAt string `json:"completedAt"`
}
type DemoOperations struct {
	Services    []ManagedService `json:"services"`
	Franchises  []DemoPlace      `json:"franchises"`
	Locations   []DemoPlace      `json:"locations"`
	Completions []DemoJobEvent   `json:"completions"`
}
type operationInput struct {
	Action      string `json:"action"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ParentID    string `json:"parentId"`
	FranchiseID string `json:"franchiseId"`
}

func (s *Store) operations(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == "GET" {
		respond(w, 200, s.Operations)
		return
	}
	if r.Header.Get("Origin") != "http://127.0.0.1:3006" {
		respond(w, 403, map[string]string{"error": "Same-origin request required"})
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		respond(w, 415, map[string]string{"error": "Use application/json"})
		return
	}
	var input operationInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respond(w, 400, map[string]string{"error": "Invalid operation data"})
		return
	}
	before, _ := json.Marshal(s.Operations)
	bookingsBefore := append([]Booking(nil), s.Bookings...)
	reject := func(message string) { respond(w, 422, map[string]string{"error": message}) }
	input.Name = strings.TrimSpace(input.Name)
	id := fmt.Sprintf("demo-%d", time.Now().UnixNano())
	switch input.Action {
	case "service.save":
		if len(input.Name) < 2 || len(input.Name) > 120 {
			reject("Enter a service name between 2 and 120 characters")
			return
		}
		if input.Kind != "main" && input.Kind != "addon" {
			reject("Choose main service or add-on")
			return
		}
		if input.Kind == "main" && input.ParentID != "" {
			reject("Main services cannot have a parent")
			return
		}
		if input.Kind == "addon" {
			found := false
			for _, service := range s.Operations.Services {
				if service.ID == input.ParentID && service.Kind == "main" && !service.Archived {
					found = true
				}
			}
			if !found {
				reject("Choose an active main service for this add-on")
				return
			}
		}
		if input.ID != "" {
			index := -1
			for i, service := range s.Operations.Services {
				if service.ID == input.ID && !service.Archived {
					index = i
				}
			}
			if index < 0 {
				reject("Service no longer available")
				return
			}
			for _, service := range s.Operations.Services {
				if service.ParentID == input.ID && !service.Archived && input.Kind != "main" {
					reject("Move active add-ons before changing this main service")
					return
				}
			}
			s.Operations.Services[index] = ManagedService{ID: input.ID, Name: input.Name, Kind: input.Kind, ParentID: input.ParentID}
		} else {
			s.Operations.Services = append(s.Operations.Services, ManagedService{ID: id, Name: input.Name, Kind: input.Kind, ParentID: input.ParentID})
		}
	case "service.archive":
		index := -1
		for i, service := range s.Operations.Services {
			if service.ID == input.ID && !service.Archived {
				index = i
			}
			if service.ParentID == input.ID && !service.Archived {
				reject("Remove or move active add-ons first")
				return
			}
		}
		if index < 0 {
			reject("Service no longer available")
			return
		}
		s.Operations.Services[index].Archived = true
	case "franchise.add", "location.add":
		if len(input.Name) < 2 || len(input.Name) > 120 {
			reject("Enter a name between 2 and 120 characters")
			return
		}
		if input.Action == "franchise.add" {
			s.Operations.Franchises = append(s.Operations.Franchises, DemoPlace{ID: id, Name: input.Name})
		} else {
			found := false
			for _, franchise := range s.Operations.Franchises {
				if franchise.ID == input.FranchiseID {
					found = true
				}
			}
			if !found {
				reject("Choose an existing franchise")
				return
			}
			s.Operations.Locations = append(s.Operations.Locations, DemoPlace{ID: id, Name: input.Name, FranchiseID: input.FranchiseID})
		}
	case "job.complete":
		found := false
		for _, job := range s.Bookings {
			if job.ID == input.ID {
				found = true
				if job.Status == "Awaiting Quote" {
					reject("Quote enquiries are not available jobs")
					return
				}
			}
		}
		if !found {
			reject("Job unavailable")
			return
		}
		for _, event := range s.Operations.Completions {
			if event.JobID == input.ID {
				respond(w, 409, map[string]string{"error": "This job has already been completed"})
				return
			}
		}
		s.Operations.Completions = append(s.Operations.Completions, DemoJobEvent{JobID: input.ID, Performer: "Synthetic demo employee", CompletedAt: time.Now().UTC().Format(time.RFC3339)})
		for i := range s.Bookings {
			if s.Bookings[i].ID == input.ID {
				s.Bookings[i].Status = "Completed"
				s.Bookings[i].Employee = ""
			}
		}
	default:
		reject("Unsupported operation")
		return
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		err = os.WriteFile(s.path+".tmp", data, 0600)
	}
	if err == nil {
		err = os.Rename(s.path+".tmp", s.path)
	}
	if err != nil {
		_ = json.Unmarshal(before, &s.Operations)
		s.Bookings = bookingsBefore
		respond(w, 500, map[string]string{"error": "Could not save changes. Please retry"})
		return
	}
	respond(w, 200, s.Operations)
}
