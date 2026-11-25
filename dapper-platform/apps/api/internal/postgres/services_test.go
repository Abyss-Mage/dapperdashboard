package postgres

import (
	"errors"
	"context"
	"testing"
)

func verifyCatalogServices(t *testing.T, ctx context.Context, s *Store, a, b Scope, customerID string) {
	t.Helper()
	for _, scope := range []Scope{a, b} {
		for _, action := range []string{"catalogue.read", "catalogue.manage"} {
			if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4)`, scope.ActorID, scope.OrganizationID, scope.LocationID, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	main, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	// error404 #112 #113: a franchise's own service keeps the price it is given; a negative price is refused.
	priced := int64(4900)
	extra, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic own extra", Kind: "main", PriceCents: &priced})
	if err != nil || extra.PriceCents == nil || *extra.PriceCents != 4900 {
		t.Fatal("franchise service price not saved", extra.PriceCents, err)
	}
	// A timed service gets a booking rule per bay; archived and never used, it still deletes for good with its rule.
	if _, err = s.pool.Exec(ctx, `INSERT INTO scheduling_resources(organization_id,location_id,name,capacity,timezone,rules) VALUES($1,$2,'Synthetic bay',1,'America/Toronto','{}'::jsonb)`, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	minutes := 89
	timed, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic window tint", Kind: "main", PriceCents: &priced, DurationMinutes: &minutes})
	if err != nil {
		t.Fatal(err)
	}
	if timed, err = s.ArchiveService(ctx, a, timed.ID, timed.Version); err != nil {
		t.Fatal(err)
	}
	if gone, e := s.DeleteService(ctx, a, timed.ID, timed.Version); e != nil || !gone.Permanent {
		t.Fatal("an archived, unused timed service was not deleted", gone, e)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND name='Synthetic bay'`, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	// error404 #133: a sale is the price while it runs, the regular price comes back after its last day, and it must be below the price.
	sale, past, tooHigh := int64(3900), "2000-01-01", int64(4900)
	onSale, err := s.SaveService(ctx, a, CatalogService{ID: extra.ID, Version: extra.Version, Name: extra.Name, Kind: "main", PriceCents: &priced, SaleSet: true, SalePriceCents: &sale})
	if err != nil || onSale.NowPriceCents == nil || *onSale.NowPriceCents != 3900 || *onSale.PriceCents != 4900 {
		t.Fatal("sale not charged", onSale.NowPriceCents, err)
	}
	var charged int64
	if err = s.pool.QueryRow(ctx, `SELECT service_effective_price($1)`, onSale.ID).Scan(&charged); err != nil || charged != 3900 {
		t.Fatal("bookings would not be charged the sale price", charged, err)
	}
	kept, err := s.SaveService(ctx, a, CatalogService{ID: onSale.ID, Version: onSale.Version, Name: "Synthetic own extra renamed", Kind: "main", PriceCents: &priced})
	if err != nil || kept.SalePriceCents == nil || *kept.NowPriceCents != 3900 {
		t.Fatal("an edit without the sale dropped it", err)
	}
	// error404 #150: an edit that leaves the price out (a bulk or text edit of names and times) keeps it.
	if kept, err = s.SaveService(ctx, a, CatalogService{ID: kept.ID, Version: kept.Version, Name: kept.Name, Kind: "main", KeepPrice: true}); err != nil || kept.PriceCents == nil || *kept.PriceCents != 4900 {
		t.Fatal("an edit without a price erased it", err)
	}
	ended, err := s.SaveService(ctx, a, CatalogService{ID: kept.ID, Version: kept.Version, Name: kept.Name, Kind: "main", PriceCents: &priced, SaleSet: true, SalePriceCents: &sale, SaleEndsOn: &past})
	if err != nil || *ended.NowPriceCents != 4900 {
		t.Fatal("an ended sale was still charged", err)
	}
	if _, err = s.SaveService(ctx, a, CatalogService{ID: ended.ID, Version: ended.Version, Name: ended.Name, Kind: "main", PriceCents: &priced, SaleSet: true, SalePriceCents: &tooHigh}); !errors.Is(err, ErrServiceInput) {
		t.Fatal("a sale at the regular price was accepted", err)
	}
	// error404 #166: a service can be booked under a category the owner names; other edits keep it, and a one-letter name is refused.
	tires, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic Tire Changeover", Kind: "main", BookingGroup: "Tire services", GroupSet: true})
	if err != nil || tires.BookingGroup != "Tire services" {
		t.Fatal("booking category not saved", tires.BookingGroup, err)
	}
	if tires, err = s.SaveService(ctx, a, CatalogService{ID: tires.ID, Version: tires.Version, Name: "Synthetic Tire Changeover - On Rim", Kind: "main"}); err != nil || tires.BookingGroup != "Tire services" {
		t.Fatal("an edit without the category dropped it", tires.BookingGroup, err)
	}
	if _, err = s.SaveService(ctx, a, CatalogService{ID: tires.ID, Version: tires.Version, Name: tires.Name, Kind: "main", BookingGroup: "x", GroupSet: true}); !errors.Is(err, ErrServiceInput) {
		t.Fatal("a one-letter booking category was accepted", err)
	}
	// error404 #167: an open booking's package and add-ons can be changed; its unpaid invoice is redone; a paid one is refused.
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'bookings.manage'),($1,$2,$3,'invoices.create') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	upPrice, extraPrice := int64(20000), int64(3000)
	upgrade, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic Upgrade Package", Kind: "main", PriceCents: &upPrice})
	if err != nil {
		t.Fatal(err)
	}
	extraOne, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic Pet Hair", Kind: "addon", ParentID: &upgrade.ID, PriceCents: &extraPrice})
	if err != nil {
		t.Fatal(err)
	}
	var bookingID string
	if err = s.pool.QueryRow(ctx, `INSERT INTO bookings(organization_id,location_id,customer_id,service_id,service_name,channel,status,booking_time) VALUES($1,$2,$3,$4,'Synthetic own extra','internal','confirmed',now()+interval '3 days') RETURNING id::text`, a.OrganizationID, a.LocationID, customerID, extra.ID).Scan(&bookingID); err != nil {
		t.Fatal(err)
	}
	autoInvoiceEnabled = true
	changed, _, err := s.ChangeBookingService(ctx, a, bookingID, upgrade.ID, []string{extraOne.ID})
	autoInvoiceEnabled = false
	if err != nil || changed.ServiceName != "Synthetic Upgrade Package" || len(changed.Addons) != 1 || changed.Addons[0].Name != "Synthetic Pet Hair" {
		t.Fatalf("booking not changed: %+v %v", changed, err)
	}
	var draftSubtotal int64
	if err = s.pool.QueryRow(ctx, `SELECT subtotal_cents FROM invoices WHERE booking_id=$1 AND status='draft'`, bookingID).Scan(&draftSubtotal); err != nil || draftSubtotal != 23000 {
		t.Fatal("the new draft invoice is not at the new prices", draftSubtotal, err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE invoices SET paid_cents=1000,due_cents=total_cents-1000 WHERE booking_id=$1 AND status='draft'`, bookingID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.ChangeBookingService(ctx, a, bookingID, extra.ID, nil); !errors.Is(err, ErrBookingPaid) {
		t.Fatal("a booking with a payment was changed", err)
	}
	if _, _, err = s.ChangeBookingService(ctx, a, bookingID, extraOne.ID, nil); !errors.Is(err, ErrBookingPaid) && !errors.Is(err, ErrBookingInput) {
		t.Fatal("an add-on was accepted as the package", err)
	}
	negative := int64(-1)
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Synthetic negative", Kind: "main", PriceCents: &negative}); !errors.Is(err, ErrServiceInput) {
		t.Fatal("a negative price was accepted", err)
	}
	child, err := s.SaveService(ctx, a, CatalogService{Name: "Synthetic add-on", Kind: "addon", ParentID: &main.ID})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, ServiceName: "Client-supplied stale name", AddonIDs: []string{child.ID}, Addons: []JobAddon{{ServiceID: child.ID, Name: "Forged name"}}})
	if err != nil || linked.ServiceName != main.Name || linked.ServiceID == nil || *linked.ServiceID != main.ID {
		t.Fatal("main service identity/snapshot not resolved", err)
	}
	if len(linked.Addons) != 1 || linked.Addons[0].Name != child.Name || linked.Addons[0].ServiceID != child.ID {
		t.Fatal("add-on snapshot not resolved by server")
	}
	for _, ids := range [][]string{{child.ID, child.ID}, {main.ID}} {
		if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: ids}); err != ErrJobAddons {
			t.Fatal("duplicate or main-as-add-on accepted", err)
		}
	}
	otherMain, err := s.SaveService(ctx, a, CatalogService{Name: "Another main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &otherMain.ID, AddonIDs: []string{child.ID}}); err != ErrJobAddons {
		t.Fatal("unrelated main add-on accepted", err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceName: "Manual", AddonIDs: []string{child.ID}}); err != ErrJobAddons {
		t.Fatal("unlinked add-on accepted", err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &child.ID}); err != ErrJobInput {
		t.Fatal("standalone add-on job accepted")
	}
	if _, err = s.SaveService(ctx, b, CatalogService{Name: "Foreign parent", Kind: "addon", ParentID: &main.ID}); err != ErrServiceInput {
		t.Fatal("foreign parent accepted", err)
	}
	if items, err := s.Services(ctx, b); err != nil || len(items) != 0 {
		t.Fatal("foreign catalogue leaked")
	}
	foreignMain, err := s.SaveService(ctx, b, CatalogService{Name: "Foreign main", Kind: "main"})
	if err != nil {
		t.Fatal(err)
	}
	foreignAddon, err := s.SaveService(ctx, b, CatalogService{Name: "Foreign add-on", Kind: "addon", ParentID: &foreignMain.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: []string{foreignAddon.ID}}); err != ErrJobAddons {
		t.Fatal("cross-scope add-on accepted", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_addon_job_audit_failure CHECK(action<>'job.created') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: []string{child.ID}, Description: "Add-on rollback fixture"}); err == nil {
		t.Fatal("add-on work order committed without audit")
	}
	var rolledBack int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE description='Add-on rollback fixture'`).Scan(&rolledBack); err != nil || rolledBack != 0 {
		t.Fatal("add-on job audit rollback failed", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_addon_job_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, b, main); err != ErrServiceConflict {
		t.Fatal("foreign service updated")
	}
	main.Name = "Updated main"
	updated, err := s.SaveService(ctx, a, main)
	if err != nil || updated.Version != 2 {
		t.Fatal("revision missing", err)
	}
	if _, err = s.SaveService(ctx, a, main); err != ErrServiceConflict {
		t.Fatal("stale editor overwrote newer revision")
	}
	if _, err = s.ArchiveService(ctx, a, child.ID, child.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID, AddonIDs: []string{child.ID}}); err != ErrJobAddons {
		t.Fatal("archived add-on accepted", err)
	}
	archived, err := s.ArchiveService(ctx, a, main.ID, updated.Version)
	if err != nil || !archived.Archived || archived.Version != 3 {
		t.Fatal("archive revision missing", err)
	}
	jobs, err := s.Jobs(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, job := range jobs {
		if job.ID == linked.ID {
			found = job.ServiceName == "Synthetic main" && len(job.Addons) == 1 && job.Addons[0].Name == "Synthetic add-on"
		}
	}
	if !found {
		t.Fatal("service rename changed historical work-order snapshot")
	}
	if _, err = s.CreateJob(ctx, a, Job{CustomerID: customerID, ServiceID: &main.ID}); err != ErrJobInput {
		t.Fatal("archived main accepted for new work")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_service_revisions WHERE service_id=$1`, main.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal("service revision history lost")
	}
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Archived parent", Kind: "addon", ParentID: &main.ID}); err != ErrServiceInput {
		t.Fatal("archived parent accepted")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events ADD CONSTRAINT test_service_audit_failure CHECK(action<>'service.revised') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Rollback fixture", Kind: "main"}); err == nil {
		t.Fatal("service committed without audit")
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_services WHERE name='Rollback fixture'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("service audit rollback failed")
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE audit_events DROP CONSTRAINT test_service_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='catalogue.manage'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, a, CatalogService{Name: "Revoked fixture", Kind: "main"}); err != ErrForbidden {
		t.Fatal("revoked manager retained access")
	}
}
