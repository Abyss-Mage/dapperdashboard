package workspace

import (
	"dapper.local/platform/internal/postgres"
	"errors"
	"net/http"
)

func employeeFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrEmployeeContact), errors.Is(err, postgres.ErrEmployeeLocalTime):
		reply(w, 422, map[string]string{"error": err.Error()})
	case errors.Is(err, postgres.ErrReservationConflict):
		reply(w, 409, map[string]string{"error": "That time is not available for this service: it is outside opening hours, inside a closure, or the resource is already full. Choose another time."})
	case errors.Is(err, postgres.ErrBookingCapacityRequest), errors.Is(err, postgres.ErrBookingReservation), errors.Is(err, postgres.ErrSchedulingRuleConflict):
		reply(w, 409, map[string]string{"error": "The time, service configuration or retry request changed. Review and choose another time before retrying."})
	case errors.Is(err, postgres.ErrSchedulingRule), errors.Is(err, postgres.ErrBookingInput), errors.Is(err, postgres.ErrReservationInput):
		reply(w, 422, map[string]string{"error": "Choose a configured service combination and valid worksite time."})
	case errors.Is(err, postgres.ErrEmployeeAssignment):
		reply(w, 409, map[string]string{"error": "Ask your administrator to assign your dealership before starting work"})
	case errors.Is(err, postgres.ErrEmployeeVersion):
		reply(w, 409, map[string]string{"error": "This record changed. Refresh and review before saving again"})
	case errors.Is(err, postgres.ErrEmployeeActiveShift):
		reply(w, 409, map[string]string{"error": "Clock out before the administrator changes the assigned worksite"})
	case errors.Is(err, postgres.ErrEmployeeInput):
		reply(w, 422, map[string]string{"error": "Check the worksite, timezone and non-overlapping next-week time ranges"})
	default:
		attendanceFailure(w, err)
	}
}
func registerEmployeeWorkspaceRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/employee/calendar", func(w http.ResponseWriter, r *http.Request) {
		for key := range r.URL.Query() {
			if key != "from" && key != "to" {
				reply(w, 400, map[string]string{"error": "Only calendar dates are accepted; worksite is assigned by the server"})
				return
			}
		}
		data, err := store.EmployeeCalendarBookings(r.Context(), requestSession(r).principalID, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("GET /api/v1/employee/walk-in/options", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query()) != 0 {
			reply(w, 400, map[string]string{"error": "Worksite selectors are not accepted"})
			return
		}
		data, err := store.EmployeeWalkInOptions(r.Context(), requestSession(r).principalID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("POST /api/v1/employee/walk-in", func(w http.ResponseWriter, r *http.Request) {
		var input postgres.EmployeeWalkInInput
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.RuleID) || !uuid(input.ResourceID) {
			reply(w, 422, map[string]string{"error": "Choose a configured service and resource"})
			return
		}
		data, err := store.CreateEmployeeWalkIn(r.Context(), requestSession(r).principalID, input)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 201, data)
	})
	mux.HandleFunc("GET /api/v1/employee/services", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.EmployeeServices(r.Context(), requestSession(r).principalID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/v1/employee/bookings/{booking}/invoices", func(w http.ResponseWriter, r *http.Request) {
		b := r.PathValue("booking")
		if !uuid(b) {
			reply(w, 400, map[string]string{"error": "Valid booking required"})
			return
		}
		items, err := store.EmployeeBookingInvoices(r.Context(), requestSession(r).principalID, b)
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/employee/bookings/{booking}/cash", func(w http.ResponseWriter, r *http.Request) {
		b := r.PathValue("booking")
		if !uuid(b) {
			reply(w, 400, map[string]string{"error": "Valid booking required"})
			return
		}
		var input struct {
			InvoiceID      string `json:"invoiceId"`
			AmountCents    int64  `json:"amountCents"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.InvoiceID) {
			reply(w, 422, map[string]string{"error": "Choose a reviewed invoice"})
			return
		}
		p, i, err := store.EmployeeCashReceipt(r.Context(), requestSession(r).principalID, b, postgres.RecordPaymentInput{InvoiceID: input.InvoiceID, AmountCents: input.AmountCents, IdempotencyKey: input.IdempotencyKey})
		if err != nil {
			invoiceFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"payment": p, "invoice": i})
	})
	mux.HandleFunc("GET /api/v1/organizations/{organization}/employee-worksites", func(w http.ResponseWriter, r *http.Request) {
		org := r.PathValue("organization")
		if !uuid(org) {
			reply(w, 400, map[string]string{"error": "Valid organization required"})
			return
		}
		data, err := store.EmployeeWorksiteAdministration(r.Context(), requestSession(r).principalID, org)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("GET /api/v1/employee/workspace", func(w http.ResponseWriter, r *http.Request) {
		// No location selectors, actor override, or inferred first authorized worksite.
		if len(r.URL.Query()) != 0 {
			reply(w, 400, map[string]string{"error": "Employee scope is assigned by your administrator"})
			return
		}
		data, err := store.EmployeeWorkspace(r.Context(), requestSession(r).principalID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("PUT /api/v1/employee/availability", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			WeekStart string                          `json:"weekStart"`
			Intervals []postgres.AvailabilityInterval `json:"intervals"`
			Notes     string                          `json:"notes"`
			Version   int                             `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		data, err := store.SubmitEmployeeAvailability(r.Context(), requestSession(r).principalID, postgres.EmployeeAvailability{WeekStart: input.WeekStart, Intervals: input.Intervals, Notes: input.Notes, Version: input.Version})
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	for _, action := range []string{"clock-in", "clock-out", "start-break", "end-break"} {
		mux.HandleFunc("POST /api/v1/employee/attendance/"+action, func(w http.ResponseWriter, r *http.Request) {
			var input struct {
				Location *locationInput `json:"location"`
			}
			if !decode(w, r, &input) {
				return
			}
			fix, ok := input.Location.fix(w)
			if !ok {
				return
			}
			data, err := store.EmployeeAttendanceAction(r.Context(), requestSession(r).principalID, action, fix)
			if err != nil {
				employeeFailure(w, err)
				return
			}
			reply(w, 200, data)
		})
	}
	mux.HandleFunc("PUT /api/v1/organizations/{organization}/employee-worksites/{principal}", func(w http.ResponseWriter, r *http.Request) {
		org, p := r.PathValue("organization"), r.PathValue("principal")
		if !uuid(org) || !uuid(p) {
			reply(w, 400, map[string]string{"error": "Valid organization and employee required"})
			return
		}
		var input struct {
			LocationID   string `json:"locationId"`
			DealershipID string `json:"dealershipId"`
			Timezone     string `json:"timezone"`
			Version      int    `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.LocationID) || (input.DealershipID != "" && !uuid(input.DealershipID)) || input.Version < 0 {
			reply(w, 422, map[string]string{"error": "Choose a dealership, location and current assignment version"})
			return
		}
		data, err := store.AssignEmployeeWorksite(r.Context(), requestSession(r).principalID, org, p, input.LocationID, input.DealershipID, input.Timezone, input.Version)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("PUT /api/v1/organizations/{organization}/employee-bookings/{booking}", func(w http.ResponseWriter, r *http.Request) {
		org, b := r.PathValue("organization"), r.PathValue("booking")
		if !uuid(org) || !uuid(b) {
			reply(w, 400, map[string]string{"error": "Valid organization and booking required"})
			return
		}
		var input struct {
			LocationID   string `json:"locationId"`
			DealershipID string `json:"dealershipId"`
		}
		if !decode(w, r, &input) {
			return
		}
		if !uuid(input.LocationID) || (input.DealershipID != "" && !uuid(input.DealershipID)) {
			reply(w, 422, map[string]string{"error": "Choose the booking's dealership worksite"})
			return
		}
		if err := store.LinkEmployeeBooking(r.Context(), requestSession(r).principalID, org, input.LocationID, input.DealershipID, b); err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"linked": true})
	})
}
