package postgres

import (
	"context"
	"dapper.local/platform/internal/scheduling"
	"testing"
	"time"
)

func verifyCentralServices(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()
	// Grant organization-level catalogue.manage and catalogue.read to a.ActorID
	if _, err := s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'catalogue.manage'),($1,$2,'catalogue.read')`, a.ActorID, a.OrganizationID); err != nil {
		t.Fatal(err)
	}

	// 1. Create central main service
	main, err := s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{
		Name: "Central Luxury Detail",
		Kind: "main",
	})
	if err != nil {
		t.Fatalf("failed to create central main: %v", err)
	}
	if main.ID == "" || main.Version != 1 || main.Archived || main.ParentID != nil {
		t.Fatalf("invalid central main service: %+v", main)
	}

	// 2. Reject main with parent
	invalidParent := main.ID
	if _, err = s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{
		Name:     "Invalid Main",
		Kind:     "main",
		ParentID: &invalidParent,
	}); err != ErrCentralServiceInput {
		t.Fatalf("expected ErrCentralServiceInput for main with parent, got %v", err)
	}

	// 3. Reject add-on without parent
	if _, err = s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{
		Name: "Orphan Addon",
		Kind: "addon",
	}); err != ErrCentralServiceInput {
		t.Fatalf("expected ErrCentralServiceInput for addon without parent, got %v", err)
	}

	// 4. Create central add-on service
	addon, err := s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{
		Name:     "Central Engine Bay Polish",
		Kind:     "addon",
		ParentID: &main.ID,
	})
	if err != nil {
		t.Fatalf("failed to create central addon: %v", err)
	}
	if addon.ParentID == nil || *addon.ParentID != main.ID {
		t.Fatalf("expected parent ID %s, got %v", main.ID, addon.ParentID)
	}

	// 5. Update central service with version increment (and a price)
	price := int64(15000)
	bad := int64(-1)
	if _, err = s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{ID: main.ID, Name: main.Name, Kind: "main", Version: 1, PriceSet: true, PriceCents: &bad}); err != ErrCentralServiceInput {
		t.Fatalf("negative price accepted: %v", err)
	}
	updatedMain, err := s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{
		ID:         main.ID,
		Name:       "Central Ultra Luxury Detail",
		Kind:       "main",
		Version:    1,
		PriceSet:   true,
		PriceCents: &price,
	})
	if err != nil {
		t.Fatalf("failed to update central main: %v", err)
	}
	if updatedMain.PriceCents == nil || *updatedMain.PriceCents != 15000 {
		t.Fatalf("central price not stored: %+v", updatedMain.PriceCents)
	}
	if updatedMain.Version != 2 || updatedMain.Name != "Central Ultra Luxury Detail" {
		t.Fatalf("expected version 2 and updated name, got %+v", updatedMain)
	}

	// 6. Stale version edit conflict
	if _, err = s.SaveCentralService(ctx, a.ActorID, a.OrganizationID, CentralServiceInput{
		ID:      main.ID,
		Name:    "Stale Edit",
		Kind:    "main",
		Version: 1,
	}); err != ErrCentralServiceConflict {
		t.Fatalf("expected ErrCentralServiceConflict on stale edit, got %v", err)
	}

	// 7. Cannot archive main while active add-on exists
	if _, err = s.ArchiveCentralService(ctx, a.ActorID, a.OrganizationID, main.ID, 2); err != ErrCentralServiceHasChildren {
		t.Fatalf("expected ErrCentralServiceHasChildren, got %v", err)
	}

	// 8. Archive add-on first
	archivedAddon, err := s.ArchiveCentralService(ctx, a.ActorID, a.OrganizationID, addon.ID, 1)
	if err != nil {
		t.Fatalf("failed to archive addon: %v", err)
	}
	if !archivedAddon.Archived || archivedAddon.Version != 2 {
		t.Fatalf("expected archived addon version 2, got %+v", archivedAddon)
	}

	// 9. Now archive main
	archivedMain, err := s.ArchiveCentralService(ctx, a.ActorID, a.OrganizationID, main.ID, 2)
	if err != nil {
		t.Fatalf("failed to archive main: %v", err)
	}
	if !archivedMain.Archived || archivedMain.Version != 3 {
		t.Fatalf("expected archived main version 3, got %+v", archivedMain)
	}

	// 9b. Restore: an add-on waits for its main; the main comes back with its franchise copies, then the add-on.
	if _, err = s.RestoreCentralService(ctx, a.ActorID, a.OrganizationID, addon.ID, 2); err != ErrCentralServiceConflict {
		t.Fatalf("an add-on was restored while its main is archived: %v", err)
	}
	restoredMain, err := s.RestoreCentralService(ctx, a.ActorID, a.OrganizationID, main.ID, 3)
	if err != nil || restoredMain.Archived || restoredMain.Version != 4 {
		t.Fatalf("main not restored: %+v %v", restoredMain, err)
	}
	var stillArchived int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_services WHERE central_service_id=$1 AND archived`, main.ID).Scan(&stillArchived); err != nil || stillArchived != 0 {
		t.Fatalf("franchise copies stayed archived: %d %v", stillArchived, err)
	}
	if _, err = s.RestoreCentralService(ctx, a.ActorID, a.OrganizationID, addon.ID, 2); err != nil {
		t.Fatalf("add-on not restored after its main: %v", err)
	}
	if _, err = s.RestoreCentralService(ctx, a.ActorID, a.OrganizationID, main.ID, 4); err != ErrCentralServiceConflict {
		t.Fatalf("restoring an active service should conflict: %v", err)
	}

	// 10. List central services
	list, err := s.ListCentralServices(ctx, a.ActorID, a.OrganizationID)
	if err != nil {
		t.Fatalf("failed to list central services: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 services in list, got %d", len(list))
	}

	// 11. Foreign organization access denied
	if _, err = s.ListCentralServices(ctx, b.ActorID, a.OrganizationID); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for foreign actor, got %v", err)
	}
}

func verifyJobWithReservation(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()
	for _, action := range []string{"jobs.create", "jobs.read", "scheduling.manage", "scheduling.reserve", "scheduling.release"} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID, action); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	resource, err := s.CreateSchedulingResource(ctx, a, SchedulingResource{
		Name:     "Reservation Bay",
		Timezone: "America/Toronto",
		Plan:     scheduling.Plan{Capacity: 1, Open: []scheduling.Interval{{Start: start, End: start.Add(8 * time.Hour)}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	reservation, err := s.ReserveCapacity(ctx, a, resource.ID, scheduling.Request{
		Start:    start.Add(time.Hour),
		Duration: 2 * time.Hour,
		Units:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create job linked to reservation
	job, err := s.CreateJob(ctx, a, Job{
		CustomerID:    customerID,
		ServiceName:   "Reservation Service",
		Description:   "Job with reservation",
		ReservationID: &reservation.ID,
	})
	if err != nil {
		t.Fatalf("failed to create job with reservation: %v", err)
	}
	if job.ReservationID == nil || *job.ReservationID != reservation.ID {
		t.Fatalf("expected reservation ID %s, got %v", reservation.ID, job.ReservationID)
	}

	// Read job returns reservationId
	jobs, err := s.Jobs(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, j := range jobs {
		if j.ID == job.ID {
			found = true
			if j.ReservationID == nil || *j.ReservationID != reservation.ID {
				t.Fatalf("expected reservation ID %s on job, got %v", reservation.ID, j.ReservationID)
			}
		}
	}
	if !found {
		t.Fatal("job not found in list")
	}

	// Cannot link foreign reservation
	if _, err = s.CreateJob(ctx, b, Job{
		CustomerID:    customerID,
		ServiceName:   "Foreign Reservation",
		ReservationID: &reservation.ID,
	}); err != ErrJobInput {
		t.Fatalf("expected ErrJobInput for foreign reservation link, got %v", err)
	}
}
