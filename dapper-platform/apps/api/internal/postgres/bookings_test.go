package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

func verifyBookings(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()

	// Grant required actions to a.ActorID
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES
($1,$2,$3,'catalogue.manage'),
($1,$2,$3,'catalogue.read'),
($1,$2,$3,'bookings.create'),
($1,$2,$3,'bookings.read'),
($1,$2,$3,'bookings.manage') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}

	// 1. Create a service to link
	main, err := s.SaveService(ctx, a, CatalogService{Name: "Signature Executive Detail", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	addon, err := s.SaveService(ctx, a, CatalogService{Name: "Signature Ceramic Rim Coating", Kind: "addon", ParentID: &main.ID})
	if err != nil {
		t.Fatal(err)
	}

	otherMain, err := s.SaveService(ctx, a, CatalogService{Name: "Other synthetic package", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]Booking{
		"wrong parent":    {ServiceID: &otherMain.ID, AddonIDs: []string{addon.ID}},
		"missing main":    {ServiceName: "Unlinked service", AddonIDs: []string{addon.ID}},
		"duplicate addon": {ServiceID: &main.ID, AddonIDs: []string{addon.ID, addon.ID}},
	} {
		candidate.CustomerID = customerID
		candidate.BookingTime = time.Now().Add(48 * time.Hour)
		candidate.Channel = "internal"
		if _, err := s.CreateBooking(ctx, a, candidate); !errors.Is(err, ErrBookingInput) {
			t.Fatalf("%s: expected invalid booking, got %v", name, err)
		}
	}
	// 2. Create booking
	var ownedVehicle, otherVehicle, foreignVehicle, otherCustomer, foreignCustomer string
	defer func() {
		// These disposable fixtures share the surrounding scope test schema.
		if _, err := s.pool.Exec(ctx, `DELETE FROM bookings WHERE vehicle_id=$1::uuid`, ownedVehicle); err != nil {
			t.Error(err)
		}
		if _, err := s.pool.Exec(ctx, `DELETE FROM vehicles WHERE id IN ($1::uuid,$2::uuid,$3::uuid)`, ownedVehicle, otherVehicle, foreignVehicle); err != nil {
			t.Error(err)
		}
		if _, err := s.pool.Exec(ctx, `DELETE FROM customers WHERE id IN ($1::uuid,$2::uuid)`, otherCustomer, foreignCustomer); err != nil {
			t.Error(err)
		}
	}()
	if err := s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Other synthetic owner','other@example.invalid') RETURNING id::text`, a.OrganizationID, a.LocationID).Scan(&otherCustomer); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		org, loc, owner string
		target          *string
	}{
		{a.OrganizationID, a.LocationID, customerID, &ownedVehicle},
		{a.OrganizationID, a.LocationID, otherCustomer, &otherVehicle},
	} {
		if err := s.pool.QueryRow(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description) VALUES($1,$2,$3,'Synthetic booking equipment') RETURNING id::text`, fixture.org, fixture.loc, fixture.owner).Scan(fixture.target); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Foreign synthetic owner','foreign@example.invalid') RETURNING id::text`, b.OrganizationID, b.LocationID).Scan(&foreignCustomer); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `INSERT INTO vehicles(organization_id,location_id,customer_id,description) VALUES($1,$2,$3,'Foreign synthetic equipment') RETURNING id::text`, b.OrganizationID, b.LocationID, foreignCustomer).Scan(&foreignVehicle); err != nil {
		t.Fatal(err)
	}
	for _, vehicleID := range []string{otherVehicle, foreignVehicle} {
		if _, err := s.CreateBooking(ctx, a, Booking{CustomerID: customerID, VehicleID: &vehicleID, ServiceID: &main.ID, Channel: "internal", BookingTime: time.Now()}); !errors.Is(err, ErrBookingInput) {
			t.Fatalf("mismatched vehicle accepted: %v", err)
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,vehicle_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,$4,'Synthetic invalid link','internal','pending',now())`, a.OrganizationID, a.LocationID, customerID, vehicleID); err == nil {
			t.Fatal("database accepted mismatched booking vehicle")
		}
	}
	bookingTime := time.Now().Add(48 * time.Hour).Truncate(time.Minute)
	booking, err := s.CreateBooking(ctx, a, Booking{
		CustomerID:  customerID,
		VehicleID:   &ownedVehicle,
		ServiceID:   &main.ID,
		AddonIDs:    []string{addon.ID},
		Channel:     "online",
		Status:      "confirmed",
		BookingTime: bookingTime,
		Notes:       "Customer requested morning slot",
	})
	if err != nil {
		t.Fatalf("failed to create booking: %v", err)
	}
	if booking.ID == "" || booking.ServiceName != main.Name || len(booking.Addons) != 1 || booking.Status != "confirmed" {
		t.Fatalf("unexpected booking output: %+v", booking)
	}
	if booking.VehicleID == nil || *booking.VehicleID != ownedVehicle {
		t.Fatal("owned booking vehicle was not preserved")
	}

	// 3. List bookings in date range
	from := bookingTime.Add(-2 * time.Hour)
	to := bookingTime.Add(2 * time.Hour)
	list, err := s.ListBookings(ctx, a, from, to, "", 10)
	if err != nil {
		t.Fatalf("failed to list bookings: %v", err)
	}
	if len(list) != 1 || list[0].ID != booking.ID {
		t.Fatalf("expected 1 booking in range, got: %+v", list)
	}

	// 4. Get booking with events
	retrieved, events, err := s.GetBooking(ctx, a, booking.ID)
	if err != nil {
		t.Fatalf("failed to get booking: %v", err)
	}
	if retrieved.ID != booking.ID || len(events) != 1 || events[0].ToStatus != "confirmed" {
		t.Fatalf("unexpected booking detail or events: %+v, events: %+v", retrieved, events)
	}

	// 5. Valid status transition: confirmed -> scheduled
	scheduled, err := s.UpdateBookingStatus(ctx, a, booking.ID, "scheduled", "", "Capacity confirmed")
	if err != nil {
		t.Fatalf("failed to update status to scheduled: %v", err)
	}
	if scheduled.Status != "scheduled" {
		t.Fatalf("expected status scheduled, got: %s", scheduled.Status)
	}

	// 6. Valid status transition: scheduled -> in_progress
	inProg, err := s.UpdateBookingStatus(ctx, a, booking.ID, "in_progress", "", "Vehicle in service bay")
	if err != nil {
		t.Fatalf("failed to update status to in_progress: %v", err)
	}
	if inProg.Status != "in_progress" {
		t.Fatalf("expected status in_progress, got: %s", inProg.Status)
	}

	// 7. Valid status transition: in_progress -> completed
	completed, err := s.UpdateBookingStatus(ctx, a, booking.ID, "completed", "", "Vehicle ready for pickup")
	if err != nil {
		t.Fatalf("failed to update status to completed: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("expected status completed, got: %s", completed.Status)
	}

	// 8. Completed booking cannot transition back to in_progress or cancelled
	if _, err = s.UpdateBookingStatus(ctx, a, booking.ID, "in_progress", "", "Reopen"); err != ErrBookingTransition {
		t.Fatalf("expected ErrBookingTransition when reopening completed booking, got: %v", err)
	}

	// 9. Verify event trail
	_, allEvents, err := s.GetBooking(ctx, a, booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(allEvents) != 4 {
		t.Fatalf("expected 4 status events, got %d: %+v", len(allEvents), allEvents)
	}

	// 10. Cross-scope denial: b cannot access a's booking
	if _, _, err = s.GetBooking(ctx, b, booking.ID); err != ErrBookingNotFound && err != ErrForbidden {
		t.Fatalf("expected ErrBookingNotFound or ErrForbidden for foreign scope b, got: %v", err)
	}
	if _, err = s.UpdateBookingStatus(ctx, b, booking.ID, "cancelled", "Cross-scope", ""); err != ErrBookingNotFound && err != ErrForbidden {
		t.Fatalf("expected ErrBookingNotFound or ErrForbidden for foreign update, got: %v", err)
	}

	// 11. Revoked grant
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='bookings.manage'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateBookingStatus(ctx, a, booking.ID, "cancelled", "", ""); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden after deleting grant, got: %v", err)
	}
}
