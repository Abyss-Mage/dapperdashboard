package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func syntheticJPEG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for x := 0; x < 32; x++ {
		for y := 0; y < 24; y++ {
			img.Set(x, y, color.RGBA{shade, uint8(x * 6), uint8(y * 9), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// verifyEmployeeUploads exercises the employee photo-post store and the supervisor review/audit path against the
// shared synthetic employee fixture. partnerA/partnerB are two partner dealership accounts at the location;
// visible/hidden/unlinked are bookings linked to the employee's dealership, another dealership, and none.
func verifyEmployeeUploads(t *testing.T, ctx context.Context, s *Store, employee string, owner Scope, visible, hidden, unlinked, partnerA, partnerB string) {
	t.Helper()
	lat, lon, acc := 44.3091, -78.3197, 12.0
	base := func(key string) UploadPostInput {
		return UploadPostInput{Kind: "dealership_tag", DealershipID: partnerA, RequestKey: key, Latitude: &lat, Longitude: &lon, Accuracy: &acc, CapturedAt: time.Now(), Photos: [][]byte{syntheticJPEG(t, 10), syntheticJPEG(t, 90)}}
	}
	if _, err := s.CreateEmployeeUploadPost(ctx, employee, base("synthetic-upload-key-0001")); !errors.Is(err, ErrUploadsUnavailable) {
		t.Fatal("upload accepted without configured storage", err)
	}
	dir := filepath.Join(t.TempDir(), "uploads")
	s.SetUploadDirectory(dir)
	defer s.SetUploadDirectory("")

	post, err := s.CreateEmployeeUploadPost(ctx, employee, base("synthetic-upload-key-0001"))
	if err != nil || post.Kind != "dealership_tag" || len(post.Photos) != 2 || post.Latitude == nil || post.DealershipName == "" || post.Photos[0].Width != 32 || post.Photos[1].Ordinal != 2 || post.BookingID != "" {
		t.Fatalf("dealership tag post %+v %v", post, err)
	}
	bytesBack, err := s.EmployeeUploadPhoto(ctx, employee, post.Photos[0].ID)
	if err != nil || !bytes.Equal(bytesBack, syntheticJPEG(t, 10)) {
		t.Fatal("stored photo did not round trip", err)
	}
	var digest []byte
	if err = s.pool.QueryRow(ctx, `SELECT sha256 FROM employee_upload_photos WHERE id=$1`, post.Photos[0].ID).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	if want := sha256.Sum256(bytesBack); !bytes.Equal(digest, want[:]) {
		t.Fatal("recorded digest does not match the stored bytes")
	}
	replay, err := s.CreateEmployeeUploadPost(ctx, employee, base("synthetic-upload-key-0001"))
	if err != nil || replay.ID != post.ID {
		t.Fatalf("replay changed identity %+v %v", replay, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("replay wrote extra files: %d", len(entries))
	}

	// Dealership tags may name another active dealership account; a made-up one is refused.
	choices, err := s.EmployeeUploadDealerships(ctx, employee)
	if err != nil || len(choices) != 2 {
		t.Fatalf("dealership choices %+v %v", choices, err)
	}
	other := base("synthetic-upload-key-0003")
	other.DealershipID = partnerB
	otherPost, err := s.CreateEmployeeUploadPost(ctx, employee, other)
	var stored string
	if err != nil || s.pool.QueryRow(ctx, `SELECT dealership_id::text FROM employee_upload_posts WHERE id=$1`, otherPost.ID).Scan(&stored) != nil || stored != partnerB {
		t.Fatalf("tag did not keep the chosen dealership %v", err)
	}
	var internalAccount string
	if err = s.pool.QueryRow(ctx, `WITH c AS (INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,'Synthetic internal customer','internal@example.invalid','+15555550101') RETURNING id) INSERT INTO dealership_accounts(organization_id,location_id,customer_id,account_name,account_code,account_type,contact_name,contact_email) SELECT $1,$2,id,'Synthetic Main Office','SYN-INTERNAL','commercial','Synthetic contact','internal@example.invalid' FROM c RETURNING id::text`, owner.OrganizationID, owner.LocationID).Scan(&internalAccount); err != nil {
		t.Fatal(err)
	}
	if choices, _ = s.EmployeeUploadDealerships(ctx, employee); len(choices) != 2 {
		t.Fatal("an internal worksite account was offered as a dealership", len(choices))
	}
	internalTag := base("synthetic-upload-key-0005")
	internalTag.DealershipID = internalAccount
	if _, err = s.CreateEmployeeUploadPost(ctx, employee, internalTag); !errors.Is(err, ErrUploadInput) {
		t.Fatal("a worksite/internal account was accepted as a dealership", err)
	}
	bogus := base("synthetic-upload-key-0004")
	bogus.DealershipID = "00000000-0000-4000-8000-000000000000"
	if _, err = s.CreateEmployeeUploadPost(ctx, employee, bogus); !errors.Is(err, ErrUploadInput) {
		t.Fatal("unknown dealership accepted", err)
	}

	// Customer-car posts are tied to a dealership-linked booking; the customer comes from the booking.
	car := base("synthetic-upload-key-0002")
	car.Kind, car.BookingID, car.InspectionType, car.DealershipID, car.Photos = "customer_car", visible, "pre", "", [][]byte{syntheticJPEG(t, 200)}
	carPost, err := s.CreateEmployeeUploadPost(ctx, employee, car)
	if err != nil || carPost.BookingID != visible || carPost.CustomerName == "" || carPost.InspectionType != "pre" || carPost.ServiceName == "" || carPost.BookingTime == nil {
		t.Fatalf("customer car post %+v %v", carPost, err)
	}
	for name, mutate := range map[string]func(*UploadPostInput){
		"foreign-dealership-booking": func(p *UploadPostInput) { p.BookingID = hidden },
		"unlinked-booking":           func(p *UploadPostInput) { p.BookingID = unlinked },
	} {
		p := car
		p.RequestKey = "synthetic-upload-key-" + name
		mutate(&p)
		if _, err = s.CreateEmployeeUploadPost(ctx, employee, p); !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if tags, _ := s.ListEmployeeUploads(ctx, employee, "dealership_tag", ""); len(tags) != 2 {
		t.Fatal("kind filter returned the wrong posts", len(tags))
	}
	if all, _ := s.ListEmployeeUploads(ctx, employee, "", ""); len(all) != 3 || all[0].ID != carPost.ID {
		t.Fatal("history is not newest first", len(all))
	}

	invalid := map[string]func(*UploadPostInput){
		"not a jpeg": func(p *UploadPostInput) { p.Photos = [][]byte{[]byte("<script>alert(1)</script>")} },
		"no photos":  func(p *UploadPostInput) { p.Photos = nil },
		"eleven photos": func(p *UploadPostInput) {
			p.Photos = make([][]byte, 11)
			for i := range p.Photos {
				p.Photos[i] = syntheticJPEG(t, 1)
			}
		},
		"short key":           func(p *UploadPostInput) { p.RequestKey = "short" },
		"car without booking": func(p *UploadPostInput) { p.Kind, p.InspectionType = "customer_car", "pre" },
		"car with typed name": func(p *UploadPostInput) {
			p.Kind, p.InspectionType, p.BookingID, p.CustomerName = "customer_car", "pre", visible, "Typed Name"
		},
		"car with dealership": func(p *UploadPostInput) {
			p.Kind, p.InspectionType, p.BookingID, p.DealershipID = "customer_car", "pre", visible, partnerB
		},
		"tag with booking":       func(p *UploadPostInput) { p.BookingID = visible },
		"tag without dealership": func(p *UploadPostInput) { p.DealershipID = "" },
		"tag with customer":      func(p *UploadPostInput) { p.CustomerName = "Someone" },
		"latitude without lon":   func(p *UploadPostInput) { p.Longitude = nil },
		"latitude out of range":  func(p *UploadPostInput) { v := 95.0; p.Latitude = &v },
		"stale capture time":     func(p *UploadPostInput) { p.CapturedAt = time.Now().Add(-48 * time.Hour) },
		"unknown kind":           func(p *UploadPostInput) { p.Kind = "selfie" },
	}
	n := 10
	for name, mutate := range invalid {
		n++
		p := base("synthetic-upload-key-00" + string(rune('A'+n)) + "-invalid")
		mutate(&p)
		if _, err = s.CreateEmployeeUploadPost(ctx, employee, p); !errors.Is(err, ErrUploadInput) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}

	// Only the owner reads through the employee path.
	var stranger string
	if err = s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-upload-stranger@example.invalid') RETURNING id::text`).Scan(&stranger); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EmployeeUploadPhoto(ctx, stranger, post.Photos[0].ID); !errors.Is(err, ErrUploadNotFound) {
		t.Fatal("another principal read a private photo", err)
	}
	if list, _ := s.ListEmployeeUploads(ctx, stranger, "", ""); len(list) != 0 {
		t.Fatal("another principal listed private posts")
	}

	// Supervisor review: needs attendance.read, every viewed photo is logged, and a logging failure returns nothing.
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'attendance.read'),($1,$2,$3,'attendance.manage') ON CONFLICT DO NOTHING`, owner.ActorID, owner.OrganizationID, owner.LocationID); err != nil {
		t.Fatal(err)
	}
	strangerScope := Scope{OrganizationID: owner.OrganizationID, LocationID: owner.LocationID, ActorID: stranger}
	if _, err = s.ListLocationUploads(ctx, strangerScope, ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("ungranted principal listed uploads", err)
	}
	if _, err = s.ReviewUploadPhoto(ctx, strangerScope, post.Photos[0].ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("ungranted principal read a photo", err)
	}
	if _, err = s.ListUploadViews(ctx, strangerScope); !errors.Is(err, ErrForbidden) {
		t.Fatal("ungranted principal read the view log", err)
	}
	reviewed, err := s.ListLocationUploads(ctx, owner, "")
	if err != nil || len(reviewed) != 3 || reviewed[0].Employee == "" {
		t.Fatalf("review list %d %v", len(reviewed), err)
	}
	if got, err := s.ListLocationUploads(ctx, owner, "customer_car"); err != nil || len(got) != 1 || got[0].BookingID != visible {
		t.Fatalf("review kind filter %v", err)
	}
	if viewed, err := s.ReviewUploadPhoto(ctx, owner, post.Photos[0].ID); err != nil || !bytes.Equal(viewed, syntheticJPEG(t, 10)) {
		t.Fatal("supervisor could not review a photo", err)
	}
	views, err := s.ListUploadViews(ctx, owner)
	if err != nil || len(views) != 1 || views[0].PhotoID != post.Photos[0].ID || views[0].Viewer == "" || views[0].Employee == views[0].Viewer {
		t.Fatalf("view log %+v %v", views, err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE employee_upload_views SET viewer_id=viewer_id`); err == nil {
		t.Fatal("view history was mutable")
	}
	if _, err = s.pool.Exec(ctx, `CREATE FUNCTION codex_fail_view_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic view audit failure'; END $$;CREATE TRIGGER codex_fail_view_audit BEFORE INSERT ON employee_upload_views FOR EACH ROW EXECUTE FUNCTION codex_fail_view_audit()`); err != nil {
		t.Fatal(err)
	}
	if viewed, err := s.ReviewUploadPhoto(ctx, owner, post.Photos[1].ID); err == nil || viewed != nil {
		t.Fatal("photo returned although the view could not be logged")
	}
	if _, err = s.pool.Exec(ctx, `DROP TRIGGER codex_fail_view_audit ON employee_upload_views;DROP FUNCTION codex_fail_view_audit()`); err != nil {
		t.Fatal(err)
	}
	if views, _ = s.ListUploadViews(ctx, owner); len(views) != 1 {
		t.Fatal("failed view left a log entry", len(views))
	}
	if _, err = s.ReviewUploadPhoto(ctx, owner, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrUploadNotFound) {
		t.Fatal("unknown photo", err)
	}

	hours, err := s.EmployeeHours(ctx, employee)
	if err != nil || hours.TodaySeconds < 0 || hours.WeekSeconds < hours.TodaySeconds || hours.MonthSeconds < hours.WeekSeconds || hours.Timezone == "" {
		t.Fatalf("hours %+v %v", hours, err)
	}
	if _, err = s.EmployeeBreaks(ctx, employee); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateEmployeeUploadPost(ctx, stranger, base("synthetic-upload-key-stranger")); !errors.Is(err, ErrEmployeeAssignment) {
		t.Fatal("unassigned principal posted", err)
	}

	// Approved retention: key tags are kept 30 days, customer-car photos permanently.
	if post.RetentionUntil == nil || time.Until(*post.RetentionUntil) < 29*24*time.Hour || time.Until(*post.RetentionUntil) > 31*24*time.Hour {
		t.Fatalf("a key tag must expire in about 30 days: %v", post.RetentionUntil)
	}
	if carPost.RetentionUntil != nil {
		t.Fatal("customer-car photos must never expire")
	}
	if n, err := s.PurgeExpiredUploads(ctx, 100); err != nil || n != 0 {
		t.Fatalf("nothing is due yet, purge removed %d: %v", n, err)
	}
	filesBefore, _ := os.ReadDir(dir)
	if _, err = s.pool.Exec(ctx, `UPDATE employee_upload_posts SET retention_until=clock_timestamp()-interval '1 minute' WHERE id=$1`, post.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.PurgeExpiredUploads(ctx, 100); err != nil || n != 1 {
		t.Fatalf("expected the one expired tag to be purged, got %d: %v", n, err)
	}
	if filesAfter, _ := os.ReadDir(dir); len(filesBefore)-len(filesAfter) != 2 {
		t.Fatalf("purge must remove exactly the expired post's two files: %d -> %d", len(filesBefore), len(filesAfter))
	}
	if _, err = s.EmployeeUploadPhoto(ctx, employee, post.Photos[0].ID); !errors.Is(err, ErrUploadNotFound) {
		t.Fatal("an expired photo is still readable", err)
	}
	if _, err = s.EmployeeUploadPhoto(ctx, employee, carPost.Photos[0].ID); err != nil {
		t.Fatal("a customer-car photo was affected by the purge", err)
	}
	if tags, _ := s.ListEmployeeUploads(ctx, employee, "dealership_tag", ""); len(tags) != 1 {
		t.Fatalf("only the unexpired tag should remain: %d", len(tags))
	}
	var purgeRecords, recordedPhotos, remainingViews int
	if err = s.pool.QueryRow(ctx, `SELECT count(*),COALESCE(sum(photo_count),0),(SELECT count(*) FROM employee_upload_views) FROM employee_upload_purges`).Scan(&purgeRecords, &recordedPhotos, &remainingViews); err != nil || purgeRecords != 1 || recordedPhotos != 2 || remainingViews != 1 {
		t.Fatalf("purge record/audit: %d records, %d photos, %d views: %v", purgeRecords, recordedPhotos, remainingViews, err)
	}
	if n, err := s.PurgeExpiredUploads(ctx, 100); err != nil || n != 0 {
		t.Fatalf("a second purge must find nothing: %d %v", n, err)
	}
}
