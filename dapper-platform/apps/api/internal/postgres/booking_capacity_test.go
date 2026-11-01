package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"errors"
	"testing"
	"time"
)

func verifyBookingCapacity(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	for _, action := range []string{"bookings.create", "bookings.manage", "scheduling.manage", "scheduling.reserve", "scheduling.release", "catalogue.manage"} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID, action); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	resource, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{Name: "Isolated atomic booking resource", Timezone: "America/Toronto", Plan: scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}}})
	if err != nil {
		t.Fatal(err)
	}
	main, err := s.SaveService(ctx, a, CatalogService{Name: "Isolated atomic main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	addon, err := s.SaveService(ctx, a, CatalogService{Name: "Isolated atomic add-on", Kind: "addon", ParentID: &main.ID})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := s.SaveSchedulingRule(ctx, a, SchedulingRule{ResourceID: resource.ID, ServiceID: main.ID, AddonIDs: []string{addon.ID}, DurationMinutes: 60, BeforeMinutes: 15, AfterMinutes: 15, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	input := ReserveBookingInput{RequestKey: "isolated-booking-capacity-001", CustomerID: customer, ResourceID: resource.ID, RuleID: rule.ID, RuleVersion: rule.Version, Start: start.Add(15 * time.Minute), Channel: "phone", Notes: "Private booking request note"}
	if _, err = s.ReserveBooking(ctx, b, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign scope reserved booking", err)
	}
	stale := input
	stale.RuleVersion++
	if _, err = s.ReserveBooking(ctx, a, stale); !errors.Is(err, ErrSchedulingRuleConflict) {
		t.Fatal("stale scheduling approval accepted", err)
	}
	type result struct {
		booking Booking
		err     error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() { v, e := s.ReserveBooking(ctx, a, input); results <- result{v, e} }()
	}
	one, two := <-results, <-results
	if one.err != nil || two.err != nil || one.booking.ID == "" || one.booking.ID != two.booking.ID || one.booking.ReservationID == nil || one.booking.Status != "confirmed" || one.booking.ServiceID == nil || *one.booking.ServiceID != main.ID || len(one.booking.Addons) != 1 || one.booking.CustomerEmail != "" || one.booking.Notes != "" {
		t.Fatal("atomic retry failed, bypassed pending or exposed private fields", one.err, two.err)
	}
	changed := input
	changed.Notes = "Conflicting request"
	if _, err = s.ReserveBooking(ctx, a, changed); !errors.Is(err, ErrBookingCapacityRequest) {
		t.Fatal("payload key conflict accepted", err)
	}
	var otherActor string
	if err = s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-second-booking-operator') RETURNING id::text`).Scan(&otherActor); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"bookings.create", "scheduling.reserve"} {
		if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, otherActor, a.OrganizationID, a.LocationID, action); err != nil {
			t.Fatal(err)
		}
	}
	other := Scope{ActorID: otherActor, OrganizationID: a.OrganizationID, LocationID: a.LocationID}
	if _, err = s.ReserveBooking(ctx, other, input); !errors.Is(err, ErrBookingCapacityRequest) {
		t.Fatal("other actor replayed booking receipt", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE booking_events ADD CONSTRAINT isolated_booking_audit_failure CHECK(to_status<>'confirmed') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	rollback := input
	rollback.RequestKey = "isolated-booking-audit-failure"
	rollback.Start = start.Add(3 * time.Hour)
	if _, err = s.ReserveBooking(ctx, a, rollback); err == nil {
		t.Fatal("booking audit failure left capacity committed")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM capacity_reservations WHERE resource_id=$1`, resource.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("audit failure orphaned reservation", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM booking_capacity_requests WHERE request_key=$1`, rollback.RequestKey).Scan(&count); err != nil || count != 0 {
		t.Fatal("audit failure left retry receipt", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE booking_events DROP CONSTRAINT isolated_booking_audit_failure`); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"isolated-capacity-contention-a", "isolated-capacity-contention-b"} {
		candidate := input
		candidate.RequestKey = key
		candidate.Start = start.Add(3 * time.Hour)
		go func(v ReserveBookingInput) { b, e := s.ReserveBooking(ctx, a, v); results <- result{b, e} }(candidate)
	}
	wins, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil {
			wins++
		} else if errors.Is(r.err, ErrReservationConflict) {
			conflicts++
		} else {
			t.Fatal("unexpected capacity race result", r.err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("overlapping atomic booking requests both won")
	}
	reservation, err := s.ReserveServiceCapacity(ctx, a, resource.ID, rule.ID, rule.Version, start.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	linked := Booking{CustomerID: customer, ServiceID: &main.ID, AddonIDs: []string{addon.ID}, ReservationID: &reservation.ID, Channel: "internal", BookingTime: reservation.Request.Start.Add(time.Minute)}
	if _, err = s.CreateBooking(ctx, a, linked); !errors.Is(err, ErrBookingReservation) {
		t.Fatal("mismatched reservation start accepted", err)
	}
	linked.BookingTime = reservation.Request.Start
	linked.AddonIDs = nil
	if _, err = s.CreateBooking(ctx, a, linked); !errors.Is(err, ErrBookingReservation) {
		t.Fatal("mismatched reservation service combination accepted", err)
	}
	linked.AddonIDs = []string{addon.ID}
	valid, err := s.CreateBooking(ctx, a, linked)
	if err != nil || valid.Status != "confirmed" {
		t.Fatal("matching reservation/default pending failed", err)
	}
	if _, err = s.CreateBooking(ctx, a, linked); !errors.Is(err, ErrBookingReservation) {
		t.Fatal("reservation reused by a second booking", err)
	}
	if _, err = s.RescheduleServiceCapacity(ctx, a, resource.ID, rule.ID, reservation.ID, rule.Version, reservation.Version, start.Add(6*time.Hour+15*time.Minute)); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("standalone move detached booking time", err)
	}
	if _, err = s.ReleaseCapacity(ctx, a, resource.ID, reservation.ID, reservation.Version); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("active booking reservation released independently", err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE bookings SET booking_time=booking_time+interval '1 minute' WHERE id=$1`, valid.ID); err == nil {
		t.Fatal("database allowed inconsistent booking time")
	}
	// A booking that holds a slot can be moved by hand: it gives the slot back and keeps the new time.
	newTime := valid.BookingTime.Add(48 * time.Hour)
	if moved, e := s.MoveUnreservedBooking(ctx, a, valid.ID, newTime); e != nil || moved.ReservationID != nil || !moved.BookingTime.Equal(newTime) || moved.Status != "confirmed" {
		t.Fatal("reserved booking could not be moved by hand", e, moved.ReservationID)
	}
	var released int
	if e := s.pool.QueryRow(ctx, `SELECT count(*) FROM capacity_reservations WHERE id=$1 AND cancelled_at IS NOT NULL`, reservation.ID).Scan(&released); e != nil || released != 1 {
		t.Fatal("moving by hand did not release the slot", e)
	}
	if _, err = s.UpdateBookingStatus(ctx, a, valid.ID, "cancelled", "Isolated operator cancellation", ""); err != nil {
		t.Fatal(err)
	}
	var cancelledVersion int
	if err = s.pool.QueryRow(ctx, `SELECT version FROM capacity_reservations WHERE id=$1 AND cancelled_at IS NOT NULL`, reservation.ID).Scan(&cancelledVersion); err != nil || cancelledVersion != reservation.Version+1 {
		t.Fatal("booking cancellation did not release its reservation atomically", err)
	}
	// Staff can correct a cancelled booking: it reopens without holding the released slot, can be completed, then cancelled again.
	if reopened, e := s.CorrectBookingStatus(ctx, a, valid.ID, "confirmed", "", ""); e != nil || reopened.Status != "confirmed" {
		t.Fatal("cancelled booking could not be reopened", e)
	}
	var stillBound *string
	if e := s.pool.QueryRow(ctx, `SELECT reservation_id::text FROM bookings WHERE id=$1`, valid.ID).Scan(&stillBound); e != nil || stillBound != nil {
		t.Fatal("reopened booking still points at its released reservation", e)
	}
	if done, e := s.CorrectBookingStatus(ctx, a, valid.ID, "completed", "", ""); e != nil || done.Status != "completed" {
		t.Fatal("reopened booking could not be completed", e)
	}
	if back, e := s.CorrectBookingStatus(ctx, a, valid.ID, "confirmed", "", ""); e != nil || back.Status != "confirmed" {
		t.Fatal("completed booking could not be set back to approved", e)
	}
	if _, e := s.CorrectBookingStatus(ctx, a, valid.ID, "completed", "", ""); e != nil {
		t.Fatal(e)
	}
	if gone, e := s.CorrectBookingStatus(ctx, a, valid.ID, "cancelled", "Marked by mistake", "", 0); e != nil || gone.Status != "cancelled" {
		t.Fatal("completed booking could not be cancelled", e)
	}
	if _, e := s.CorrectBookingStatus(ctx, a, valid.ID, "bogus", "", ""); !errors.Is(e, ErrBookingTransition) {
		t.Fatal("unknown status accepted for a cancelled booking", e)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.reserve'`, otherActor); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReserveBooking(ctx, other, input); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked permission replayed reservation", err)
	}
	if _, err = s.UpdateBookingStatus(ctx, a, one.booking.ID, "cancelled", "Requested cancellation", "Cancellation audit test", 0); !errors.Is(err, ErrReservationConflict) {
		t.Fatal("stale cancellation version accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='scheduling.release'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateBookingStatus(ctx, a, one.booking.ID, "cancelled", "Requested cancellation", "Cancellation audit test", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("linked cancellation bypassed release grant", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'scheduling.release')`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT isolated_cancel_release_failure CHECK(action<>'scheduling.released') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateBookingStatus(ctx, a, one.booking.ID, "cancelled", "Requested cancellation", "Cancellation audit test", 1); err == nil {
		t.Fatal("release audit failure committed partial cancellation")
	}
	var status string
	var retainedVersion int
	if err = s.pool.QueryRow(ctx, `SELECT b.status,r.version FROM bookings b JOIN capacity_reservations r ON r.id=b.reservation_id WHERE b.id=$1 AND r.cancelled_at IS NULL`, one.booking.ID).Scan(&status, &retainedVersion); err != nil || status != "confirmed" || retainedVersion != 1 {
		t.Fatal("failed cancellation lost booking/capacity state", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT isolated_cancel_release_failure`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		go func() {
			v, e := s.UpdateBookingStatus(ctx, a, one.booking.ID, "cancelled", "Requested cancellation", "Cancellation audit test", 1)
			results <- result{v, e}
		}()
	}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil || r.booking.ID != one.booking.ID || r.booking.Status != "cancelled" || r.booking.ReservationVersion == nil || *r.booking.ReservationVersion != 2 || r.booking.Notes != "" || r.booking.CustomerEmail != "" {
			t.Fatal("concurrent cancellation retry failed or leaked", r.err)
		}
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM booking_events WHERE booking_id=$1 AND to_status='cancelled'`, one.booking.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("cancellation retry duplicated event", err)
	}
	if _, err = s.UpdateBookingStatus(ctx, a, one.booking.ID, "cancelled", "Different reason", "Cancellation audit test", 1); !errors.Is(err, ErrBookingCapacityRequest) {
		t.Fatal("changed cancellation replay silently accepted", err)
	}
	replacement := input
	replacement.RequestKey = "isolated-after-cancel-replacement"
	replacementBooking, replacementErr := s.ReserveBooking(ctx, a, replacement)
	if replacementErr != nil {
		t.Fatal("atomic cancellation did not free occupied capacity", err)
	}
	jobOutcome := make(chan error, 1)
	cancelOutcome := make(chan error, 1)
	go func() {
		_, e := s.CreateBookingJob(ctx, a, replacementBooking.ID, "isolated-job-cancel-race-001", "Isolated conversion/cancellation race")
		jobOutcome <- e
	}()
	go func() {
		_, e := s.UpdateBookingStatus(ctx, a, replacementBooking.ID, "cancelled", "Concurrent operator cancellation", "Concurrent cancellation", 1)
		cancelOutcome <- e
	}()
	jobErr, cancelErr := <-jobOutcome, <-cancelOutcome
	if cancelErr != nil || jobErr != nil && !errors.Is(jobErr, ErrJobUnavailable) {
		t.Fatal("conversion/cancellation lock ordering failed", jobErr, cancelErr)
	}
}
