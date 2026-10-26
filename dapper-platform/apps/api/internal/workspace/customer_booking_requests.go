package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func customerRequestFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrCustomerBookingRequest) {
		reply(w, 422, map[string]string{"error": "Review the service information and choose a current offer/time; notes must be at most 2000 characters"})
		return
	}
	if errors.Is(err, postgres.ErrCustomerBookingRequestConflict) {
		reply(w, 409, map[string]string{"error": "This request key was already used with different details. Retry the original request or start a new one."})
		return
	}
	if errors.Is(err, postgres.ErrReservationConflict) {
		reply(w, 409, map[string]string{"error": "This start time is unavailable. Check current availability and choose another time."})
		return
	}
	if errors.Is(err, postgres.ErrAvailabilityLimit) {
		reply(w, 503, map[string]string{"error": "The selected time cannot be checked right now. Please contact Dapper."})
		return
	}
	portalOfferFailure(w, err)
}
func registerCustomerBookingRequestRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("POST /api/v1/portal/accounts/{customer}/vehicles/{vehicle}/booking-requests", func(w http.ResponseWriter, r *http.Request) {
		customer, vehicle := r.PathValue("customer"), r.PathValue("vehicle")
		if !uuid(customer) || !uuid(vehicle) {
			reply(w, 400, map[string]string{"error": "Valid owned customer and equipment selectors required"})
			return
		}
		var input postgres.CustomerBookingRequestInput
		if !decodeLimited(w, r, &input, 16*1024) {
			return
		}
		if !uuid(input.OfferID) {
			reply(w, 422, map[string]string{"error": "Choose a current service offer"})
			return
		}
		saved, err := store.SubmitCustomerBookingRequest(r.Context(), requestSession(r).principalID, customer, vehicle, input)
		if err != nil {
			customerRequestFailure(w, err)
			return
		}
		reply(w, 201, saved)
	})
	mux.HandleFunc("GET /api/v1/portal/accounts/{customer}/booking-requests", func(w http.ResponseWriter, r *http.Request) {
		customer := r.PathValue("customer")
		if !uuid(customer) {
			reply(w, 400, map[string]string{"error": "Valid owned customer selector required"})
			return
		}
		items, err := store.CustomerBookingRequests(r.Context(), requestSession(r).principalID, customer)
		if err != nil {
			customerRequestFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
	mux.HandleFunc("GET /api/v1/organizations/{organization}/locations/{location}/customer-booking-requests", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid scoped selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.read")
		if err != nil {
			failure(w, err)
			return
		}
		items, err := store.StaffCustomerBookingRequests(r.Context(), scope)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": 100})
	})
}
