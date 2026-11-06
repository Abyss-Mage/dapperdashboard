package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func bookingFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrBookingReservation) || errors.Is(err, postgres.ErrBookingCapacityRequest) || errors.Is(err, postgres.ErrReservationConflict) {
		reply(w, 409, map[string]string{"error": "Booking reservation or retry request conflicts. Review the current booking, service rule and capacity before retrying."})
		return
	}
	if errors.Is(err, postgres.ErrBookingNotFound) {
		reply(w, 404, map[string]string{"error": "Booking not found in this scope"})
		return
	}
	if errors.Is(err, postgres.ErrEmployeeStatusRule) {
		reply(w, 403, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, postgres.ErrBookingTransition) {
		reply(w, 409, map[string]string{"error": "Invalid booking status transition"})
		return
	}
	if errors.Is(err, postgres.ErrBookingInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"booking": "Provide valid customer, service, time, and channel"}})
		return
	}
	failure(w, err)
}

func registerBookingRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/bookings"
	mux.HandleFunc("GET "+base+"/reschedule-options", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid scoped selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.manage")
		if err != nil {
			failure(w, err)
			return
		}
		choices, err := store.BookingRescheduleChoices(r.Context(), scope)
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, choices)
	})
	mux.HandleFunc("POST "+base+"/{booking}/move", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("booking")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid scoped booking selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			BookingTime string `json:"bookingTime"`
		}
		if !decode(w, r, &input) {
			return
		}
		to, perr := time.Parse(time.RFC3339Nano, input.BookingTime)
		if perr != nil {
			reply(w, 422, map[string]any{"errors": map[string]string{"bookingTime": "Provide a timestamp with an explicit UTC offset"}})
			return
		}
		if err = store.CheckEmployeeBookingStatus(r.Context(), scope, id, "rescheduled"); err != nil {
			bookingFailure(w, err)
			return
		}
		booking, err := store.MoveUnreservedBooking(r.Context(), scope, id, to)
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, booking)
	})
	mux.HandleFunc("POST "+base+"/{booking}/reschedule", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("booking")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid scoped booking selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input postgres.RescheduleBookingInput
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.ResourceID) || !uuid(input.RuleID) {
			reply(w, 422, map[string]string{"error": "Valid resource and rule selectors required"})
			return
		}
		booking, err := store.RescheduleBooking(r.Context(), scope, id, input)
		if errors.Is(err, postgres.ErrSchedulingRuleConflict) {
			reply(w, 409, map[string]string{"error": "Service rule changed; review current scheduling choices"})
			return
		}
		if errors.Is(err, postgres.ErrSchedulingRule) || errors.Is(err, postgres.ErrReservationInput) {
			reply(w, 422, map[string]string{"error": "Choose a current matching service rule and valid start time"})
			return
		}
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, booking)
	})
	mux.HandleFunc("GET "+base+"/reservation-options", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid scoped selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.create")
		if err != nil {
			failure(w, err)
			return
		}
		options, err := store.BookingCapacityChoices(r.Context(), scope)
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, options)
	})
	mux.HandleFunc("POST "+base+"/reserve", func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization/location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.create")
		if err != nil {
			failure(w, err)
			return
		}
		var body struct {
			postgres.ReserveBookingInput
			CouponCode string `json:"couponCode"`
		}
		if !decode(w, r, &body) {
			return
		}
		input, coupon := body.ReserveBookingInput, strings.TrimSpace(body.CouponCode)
		if !uuid(input.CustomerID) || !uuid(input.ResourceID) || !uuid(input.RuleID) || (input.VehicleID != nil && !uuid(*input.VehicleID)) {
			reply(w, 422, map[string]string{"error": "Valid scoped customer, owned equipment, resource and rule selectors required"})
			return
		}
		// A coupon is checked before anything is held; a booking with a coupon is invoiced at once so the discount is on the books.
		couponFailed := func(err error) bool {
			var bad postgres.CouponInvalid
			if errors.As(err, &bad) {
				reply(w, 422, map[string]string{"error": bad.Reason})
				return true
			}
			return false
		}
		if coupon != "" {
			if err := store.CheckStaffCoupon(r.Context(), scope, input, coupon); err != nil {
				if !couponFailed(err) {
					bookingFailure(w, err)
				}
				return
			}
		}
		booking, err := store.ReserveBooking(r.Context(), scope, input)
		if errors.Is(err, postgres.ErrReservationConflict) || errors.Is(err, postgres.ErrSchedulingRuleConflict) {
			reply(w, 409, map[string]string{"error": "Selected capacity or service rule changed or is unavailable"})
			return
		}
		if errors.Is(err, postgres.ErrSchedulingRule) || errors.Is(err, postgres.ErrReservationInput) {
			reply(w, 422, map[string]string{"error": "Choose a current active service rule and valid start time"})
			return
		}
		if err != nil {
			bookingFailure(w, err)
			return
		}
		if coupon != "" {
			if err := store.InvoiceStaffBooking(r.Context(), scope, input, booking.ID, coupon); err != nil {
				_, _ = store.UpdateBookingStatus(context.WithoutCancel(r.Context()), scope, booking.ID, "cancelled", "The coupon could not be applied", "")
				if !couponFailed(err) {
					bookingFailure(w, err)
				}
				return
			}
		}
		reply(w, 201, booking)
	})
	mux.HandleFunc("POST "+base+"/{booking}/work-orders", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("booking")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid organization, location and booking selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "jobs.create")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			RequestKey  string `json:"requestKey"`
			Description string `json:"description"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.RequestKey) < 16 || len(input.RequestKey) > 128 {
			reply(w, 422, map[string]string{"error": "Provide a request key of 16–128 characters and reuse it when retrying"})
			return
		}
		job, err := store.CreateBookingJob(r.Context(), scope, id, input.RequestKey, input.Description)
		if errors.Is(err, postgres.ErrBookingJobRequest) {
			reply(w, 409, map[string]string{"error": "Request key was already used for a different work-order request"})
			return
		}
		if errors.Is(err, postgres.ErrJobUnavailable) {
			reply(w, 409, map[string]string{"error": "Cancelled or completed booking cannot create new work"})
			return
		}
		if errors.Is(err, postgres.ErrJobInput) {
			reply(w, 422, map[string]string{"error": "Use a description of at most 2000 characters and a booking with an active reservation if linked"})
			return
		}
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 201, job)
	})

	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.create")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			CustomerID     string   `json:"customerId"`
			VehicleID      *string  `json:"vehicleId"`
			ServiceID      *string  `json:"serviceId"`
			ServiceName    string   `json:"serviceName"`
			AddonIDs       []string `json:"addonIds"`
			ReservationID  *string  `json:"reservationId"`
			Channel        string   `json:"channel"`
			Status         string   `json:"status"`
			BookingTime    string   `json:"bookingTime"`
			Notes          string   `json:"notes"`
			ServiceAddress string   `json:"serviceAddress"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.CustomerID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"customerId": "Valid customer required"}})
			return
		}
		if input.VehicleID != nil && !uuid(*input.VehicleID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"vehicleId": "Valid owned vehicle required"}})
			return
		}
		if input.ReservationID != nil && !uuid(*input.ReservationID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"reservationId": "Valid reservation ID required"}})
			return
		}
		if input.ServiceID != nil && !uuid(*input.ServiceID) {
			reply(w, 422, map[string]any{"errors": map[string]string{"serviceId": "Valid service ID required"}})
			return
		}
		for _, aid := range input.AddonIDs {
			if !uuid(aid) {
				reply(w, 422, map[string]any{"errors": map[string]string{"addonIds": "Valid addon IDs required"}})
				return
			}
		}

		t, err := time.Parse(time.RFC3339Nano, input.BookingTime)
		if err != nil {
			t, err = time.Parse(time.RFC3339, input.BookingTime)
		}
		if err != nil {
			reply(w, 422, map[string]any{"errors": map[string]string{"bookingTime": "Provide timestamp with explicit UTC offset"}})
			return
		}

		item, err := store.CreateBooking(r.Context(), scope, postgres.Booking{
			CustomerID:     input.CustomerID,
			VehicleID:      input.VehicleID,
			ServiceID:      input.ServiceID,
			ServiceName:    input.ServiceName,
			AddonIDs:       input.AddonIDs,
			ReservationID:  input.ReservationID,
			Channel:        input.Channel,
			Status:         input.Status,
			BookingTime:    t,
			Notes:          input.Notes,
			ServiceAddress: input.ServiceAddress,
		})
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 201, item)
	})

	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.read")
		if err != nil {
			failure(w, err)
			return
		}

		from := time.Now().Add(-30 * 24 * time.Hour)
		to := time.Now().Add(60 * 24 * time.Hour)
		if raw := r.URL.Query().Get("from"); raw != "" {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, raw)
			}
			if err != nil {
				reply(w, 422, map[string]any{"errors": map[string]string{"from": "Provide valid RFC3339 from timestamp"}})
				return
			}
			from = parsed
		}
		if raw := r.URL.Query().Get("to"); raw != "" {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, raw)
			}
			if err != nil {
				reply(w, 422, map[string]any{"errors": map[string]string{"to": "Provide valid RFC3339 to timestamp"}})
				return
			}
			to = parsed
		}

		status := r.URL.Query().Get("status")
		// The calendar asks for a whole month (up to 1000 bookings); the list keeps the short default.
		limit := 50
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
		items, err := store.ListBookings(r.Context(), scope, from, to, status, limit)
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "limit": limit})
	})

	mux.HandleFunc("GET "+base+"/{booking}", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("booking")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.read")
		if err != nil {
			failure(w, err)
			return
		}
		item, events, err := store.GetBooking(r.Context(), scope, id)
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"booking": item, "events": events})
	})

	mux.HandleFunc("POST "+base+"/{booking}/status", func(w http.ResponseWriter, r *http.Request) {
		org, loc, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("booking")
		if !uuid(org) || !uuid(loc) || !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid selectors required"})
			return
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, "bookings.manage")
		if err != nil {
			failure(w, err)
			return
		}
		var input struct {
			ToStatus           string `json:"toStatus"`
			ReservationVersion *int   `json:"reservationVersion"`
			CancellationReason string `json:"cancellationReason"`
			Notes              string `json:"notes"`
		}
		if !decode(w, r, &input) {
			return
		}
		var item postgres.Booking
		input.ToStatus = strings.TrimSpace(input.ToStatus)
		if err = store.CheckEmployeeBookingStatus(r.Context(), scope, id, input.ToStatus); err != nil {
			bookingFailure(w, err)
			return
		}
		if input.ToStatus == "cancelled" {
			if input.ReservationVersion == nil || *input.ReservationVersion < 0 {
				reply(w, 422, map[string]string{"error": "Provide the observed reservationVersion, or0 for an unreserved booking"})
				return
			}
			item, err = store.CorrectBookingStatus(r.Context(), scope, id, input.ToStatus, input.CancellationReason, input.Notes, *input.ReservationVersion)
		} else {
			item, err = store.CorrectBookingStatus(r.Context(), scope, id, input.ToStatus, input.CancellationReason, input.Notes)
		}
		if errors.Is(err, postgres.ErrReservationConflict) {
			reply(w, 409, map[string]string{"error": "Reservation changed; refresh the booking before cancelling"})
			return
		}
		if err != nil {
			bookingFailure(w, err)
			return
		}
		reply(w, 200, item)
	})
}
