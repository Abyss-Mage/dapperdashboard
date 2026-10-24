package workspace

import (
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"net/http"
	"time"
)

type windowInput struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type resourceInput struct {
	Name     string        `json:"name"`
	Timezone string        `json:"timezone"`
	Capacity int           `json:"capacity"`
	Open     []windowInput `json:"open"`
	Closed   []windowInput `json:"closed"`
}
type reservationInput struct {
	Start    time.Time `json:"start"`
	Duration *int      `json:"durationMinutes"`
	Before   *int      `json:"beforeMinutes"`
	After    *int      `json:"afterMinutes"`
	Units    int       `json:"units"`
	Version  int       `json:"version"`
}

func resourceView(item postgres.SchedulingResource) map[string]any {
	windows := func(input []scheduling.Interval) []windowInput {
		result := []windowInput{}
		for _, i := range input {
			result = append(result, windowInput{i.Start, i.End})
		}
		return result
	}
	return map[string]any{"id": item.ID, "name": item.Name, "timezone": item.Timezone, "capacity": item.Plan.Capacity, "open": windows(item.Plan.Open), "closed": windows(item.Plan.Closed), "weeklyHours": item.WeeklyHours}
}
func reservationView(item postgres.CapacityReservation) map[string]any {
	return map[string]any{
		"id": item.ID, "resourceId": item.ResourceID, "start": item.Request.Start, "durationMinutes": int(item.Request.Duration / time.Minute), "beforeMinutes": int(item.Request.Before / time.Minute), "afterMinutes": int(item.Request.After / time.Minute), "units": item.Request.Units, "occupiedStart": item.Occupied.Start, "occupiedEnd": item.Occupied.End, "version": item.Version, "cancelledAt": item.CancelledAt, "cancelledBy": item.CancelledBy, "bookingConfirmed": false, "schedulingRule": item.Rule,
	}
}
func schedulingFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrReservationInput) {
		reply(w, 422, map[string]any{"errors": map[string]string{"scheduling": "Provide valid explicit dated rules, timezone, capacity and whole-microsecond timestamps"}})
		return
	}
	if errors.Is(err, postgres.ErrReservationConflict) {
		reply(w, 409, map[string]string{"error": "Resource or reservation unavailable, changed, closed or at capacity"})
		return
	}
	failure(w, err)
}
func registerSchedulingRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/scheduling/resources"
	for _, route := range []string{"GET " + base, "POST " + base, "POST " + base + "/{resource}/reservations", "POST " + base + "/{resource}/reservations/{reservation}/reschedule", "POST " + base + "/{resource}/reservations/{reservation}/release"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			org, location, resource, id := r.PathValue("organization"), r.PathValue("location"), r.PathValue("resource"), r.PathValue("reservation")
			if !uuid(org) || !uuid(location) || resource != "" && !uuid(resource) || id != "" && !uuid(id) {
				reply(w, 400, map[string]string{"error": "Valid scope, resource and reservation selectors required"})
				return
			}
			action := "scheduling.manage"
			if resource != "" {
				action = "scheduling.reserve"
			}
			release := r.Pattern == "POST "+base+"/{resource}/reservations/{reservation}/release"
			if release {
				action = "scheduling.release"
			}
			scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, location, action)
			if err != nil {
				failure(w, err)
				return
			}
			if resource == "" {
				if r.Method == "GET" {
					items, err := store.SchedulingResources(r.Context(), scope)
					if err != nil {
						failure(w, err)
						return
					}
					views := []map[string]any{}
					for _, item := range items {
						views = append(views, resourceView(item))
					}
					reply(w, 200, map[string]any{"items": views, "limit": 100, "bookingConfirmed": false})
					return
				}
				var input resourceInput
				if !decode(w, r, &input) {
					return
				}
				plan := scheduling.Plan{Capacity: input.Capacity}
				for _, i := range input.Open {
					plan.Open = append(plan.Open, scheduling.Interval{Start: i.Start, End: i.End})
				}
				for _, i := range input.Closed {
					plan.Closed = append(plan.Closed, scheduling.Interval{Start: i.Start, End: i.End})
				}
				item, err := store.CreateSchedulingResource(r.Context(), scope, postgres.SchedulingResource{Name: input.Name, Timezone: input.Timezone, Plan: plan})
				if err != nil {
					schedulingFailure(w, err)
					return
				}
				reply(w, 201, resourceView(item))
				return
			}
			if release {
				var input struct {
					Version int `json:"version"`
				}
				if !decode(w, r, &input) {
					return
				}
				item, err := store.ReleaseCapacity(r.Context(), scope, resource, id, input.Version)
				if err != nil {
					schedulingFailure(w, err)
					return
				}
				reply(w, 200, reservationView(item))
				return
			}
			var input reservationInput
			if !decode(w, r, &input) {
				return
			}
			fields := map[string]string{}
			if input.Start.IsZero() {
				fields["start"] = "Provide a timestamp with an explicit UTC offset"
			}
			for _, field := range []struct {
				name    string
				value   *int
				minimum int
			}{{"durationMinutes", input.Duration, 1}, {"beforeMinutes", input.Before, 0}, {"afterMinutes", input.After, 0}} {
				if field.value == nil || *field.value < field.minimum || *field.value > 10080 {
					fields[field.name] = "Provide explicit whole minutes within the supported request range"
				}
			}
			if input.Units < 1 || input.Units > 1000000 {
				fields["units"] = "Provide explicit positive capacity units"
			}
			if id != "" && input.Version < 1 || id == "" && input.Version != 0 {
				fields["version"] = "Use the current version for moves; omit version for new reservations"
			}
			if len(fields) > 0 {
				reply(w, 422, map[string]any{"errors": fields})
				return
			}
			request := scheduling.Request{Start: input.Start, Duration: time.Duration(*input.Duration) * time.Minute, Before: time.Duration(*input.Before) * time.Minute, After: time.Duration(*input.After) * time.Minute, Units: input.Units}
			var item postgres.CapacityReservation
			if id == "" {
				item, err = store.ReserveCapacity(r.Context(), scope, resource, request)
			} else {
				item, err = store.RescheduleCapacity(r.Context(), scope, resource, id, input.Version, request)
			}
			if err != nil {
				schedulingFailure(w, err)
				return
			}
			status := 201
			if id != "" {
				status = 200
			}
			reply(w, status, reservationView(item))
		})
	}
}
