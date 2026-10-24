package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

func verifyBookingJobs(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	for _, scope := range []Scope{a, b} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'jobs.create'),($1,$2,$3,'bookings.read') ON CONFLICT DO NOTHING`, scope.ActorID, scope.OrganizationID, scope.LocationID); err != nil {
			t.Fatal(err)
		}
	}
	booking, err := s.CreateBooking(ctx, a, Booking{CustomerID: customer, ServiceName: "Synthetic booking snapshot", Channel: "internal", Status: "pending", BookingTime: time.Now().Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		job Job
		err error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			j, e := s.CreateBookingJob(ctx, a, booking.ID, "synthetic-conversion-key-001", "Synthetic conversion")
			results <- outcome{j, e}
		}()
	}
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.job.ID == "" || first.job.ID != second.job.ID {
		t.Fatalf("concurrent retry did not return one job: %v %v", first.err, second.err)
	}
	if first.job.BookingID == nil || *first.job.BookingID != booking.ID || first.job.CustomerID != customer || first.job.ServiceName != booking.ServiceName {
		t.Fatal("server booking snapshot/link missing")
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE entity_id=$1 AND action='job.created'`, first.job.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate or missing conversion audit", err)
	}
	if _, err := s.CreateBookingJob(ctx, a, booking.ID, "synthetic-conversion-key-001", "Changed payload"); !errors.Is(err, ErrBookingJobRequest) {
		t.Fatalf("changed request accepted: %v", err)
	}
	if _, err := s.CreateBookingJob(ctx, b, booking.ID, "synthetic-foreign-request-01", ""); !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("foreign booking converted: %v", err)
	}
	// Database ownership invariant independently rejects matching-location but
	// wrong-customer links, even for a caller that bypasses the repository.
	var otherCustomer string
	if err := s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Synthetic conversion mismatch','conversion@example.invalid') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&otherCustomer); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE jobs SET customer_id=$1 WHERE id=$2`, otherCustomer, first.job.ID); err == nil {
		t.Fatal("database accepted job/booking customer mismatch")
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM customers WHERE id=$1`, otherCustomer); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT conversion_audit_failure CHECK(action<>'job.created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBookingJob(ctx, a, booking.ID, "synthetic-audit-rollback-01", ""); err == nil {
		t.Fatal("audit failure accepted conversion")
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM booking_job_requests WHERE request_key='synthetic-audit-rollback-01'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed conversion left retry receipt", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE booking_id=$1`, booking.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed conversion left extra job", err)
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT conversion_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE bookings SET status='cancelled' WHERE id=$1`, booking.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBookingJob(ctx, a, booking.ID, "synthetic-cancelled-work-01", ""); !errors.Is(err, ErrJobUnavailable) {
		t.Fatalf("cancelled booking generated work: %v", err)
	}
	// A retry returns its historical receipt rather than creating work again.
	if replay, err := s.CreateBookingJob(ctx, a, booking.ID, "synthetic-conversion-key-001", "Synthetic conversion"); err != nil || replay.ID != first.job.ID {
		t.Fatalf("historical retry failed: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='bookings.read'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBookingJob(ctx, a, booking.ID, "synthetic-conversion-key-001", "Synthetic conversion"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("retry bypassed revoked grant: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'bookings.read')`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
}
