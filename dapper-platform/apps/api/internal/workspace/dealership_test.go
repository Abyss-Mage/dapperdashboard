package workspace

import (
	"context"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyDealershipHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, customer, token, origin string) {
	t.Helper()

	base := "/api/v1/organizations/" + org + "/locations/" + loc + "/dealerships"

	send := func(method, path string, input any, bearer string) *httptest.ResponseRecorder {
		var body []byte
		if input != nil {
			body, _ = json.Marshal(input)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	// 1. Without grants: 403
	if send("GET", base, nil, token).Code != 403 {
		t.Fatal("dealership list allowed without grant")
	}

	// 2. Grant dealership.read and dealership.manage
	for _, action := range []string{"dealership.read", "dealership.manage"} {
		if _, err := setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Create dealership account
	createInput := postgres.CreateDealershipInput{
		CustomerID:       customer,
		AccountName:      "BMW Midtown Fleet",
		AccountCode:      "BMW-MIDTOWN-01",
		AccountType:      "dealership",
		ContactName:      "Sarah Connor",
		ContactEmail:     "sconnor@bmw-midtown.invalid",
		ContactPhone:     "416-555-0199",
		BillingTermsDays: 30,
		TaxExempt:        false,
		Notes:            "Priority corporate fleet detailing account",
	}

	createRec := send("POST", base, createInput, token)
	if createRec.Code != 201 {
		t.Fatalf("create dealership failed with status %d: %s", createRec.Code, createRec.Body.String())
	}
	var created postgres.DealershipAccount
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.AccountCode != "BMW-MIDTOWN-01" {
		t.Fatalf("unexpected dealership account: %+v", created)
	}

	// 4. Duplicate account code returns 409
	dupRec := send("POST", base, createInput, token)
	if dupRec.Code != 409 {
		t.Fatalf("expected 409 for duplicate account code, got %d", dupRec.Code)
	}

	// 5. Add tag
	tagRec := send("POST", base+"/"+created.ID+"/tags", map[string]string{
		"tagName":  "VIP-LOT",
		"tagColor": "#ff6600",
	}, token)
	if tagRec.Code != 201 {
		t.Fatalf("add tag failed with status %d: %s", tagRec.Code, tagRec.Body.String())
	}

	// 6. Get single account with tag
	getRec := send("GET", base+"/"+created.ID, nil, token)
	if getRec.Code != 200 {
		t.Fatalf("get dealership failed with status %d: %s", getRec.Code, getRec.Body.String())
	}
	var fetched postgres.DealershipAccount
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if len(fetched.Tags) != 1 || fetched.Tags[0].TagName != "VIP-LOT" {
		t.Fatalf("tags not returned on dealership: %+v", fetched.Tags)
	}

	// 7. Create fleet intake record
	intakeInput := postgres.CreateFleetIntakeInput{
		VehicleDescription: "2026 BMW M3 Sedan",
		Category:           "Luxury Sports",
		StockNumber:        "STK-9901-M3",
		VIN:                "WBA33AY08NFP9901",
		PONumber:           "PO-2026-4412",
		LotLocation:        "Bay 3 Rear",
		ServiceName:        "Dealership Delivery Detail",
		IntakeNotes:        "Customer delivery at 4pm; rush wash and interior seal",
	}

	intakeRec := send("POST", base+"/"+created.ID+"/intake", intakeInput, token)
	if intakeRec.Code != 201 {
		t.Fatalf("create fleet intake failed with status %d: %s", intakeRec.Code, intakeRec.Body.String())
	}
	var intake postgres.DealershipFleetIntake
	if err := json.Unmarshal(intakeRec.Body.Bytes(), &intake); err != nil {
		t.Fatal(err)
	}
	if intake.ID == "" || intake.StockNumber != "STK-9901-M3" || intake.Status != "intake" {
		t.Fatalf("unexpected fleet intake: %+v", intake)
	}

	// 8. Duplicate stock number returns 409
	dupIntakeRec := send("POST", base+"/"+created.ID+"/intake", intakeInput, token)
	if dupIntakeRec.Code != 409 {
		t.Fatalf("expected 409 for duplicate stock number, got %d", dupIntakeRec.Code)
	}

	// 9. Update status: intake -> in_progress -> completed -> released
	statusRec := send("PUT", base+"/"+created.ID+"/intake/"+intake.ID+"/status", map[string]string{
		"status": "in_progress",
		"notes":  "Detailing started by team",
	}, token)
	if statusRec.Code != 200 {
		t.Fatalf("update status to in_progress failed: %d", statusRec.Code)
	}

	statusRec2 := send("PUT", base+"/"+created.ID+"/intake/"+intake.ID+"/status", map[string]string{
		"status": "completed",
		"notes":  "Passed inspection",
	}, token)
	if statusRec2.Code != 200 {
		t.Fatalf("update status to completed failed: %d", statusRec2.Code)
	}

	statusRec3 := send("PUT", base+"/"+created.ID+"/intake/"+intake.ID+"/status", map[string]string{
		"status": "released",
		"notes":  "Loaded onto dealer transport",
	}, token)
	if statusRec3.Code != 200 {
		t.Fatalf("update status to released failed: %d", statusRec3.Code)
	}

	// 10. List intake with status filter
	listRec := send("GET", base+"/"+created.ID+"/intake?status=released", nil, token)
	if listRec.Code != 200 {
		t.Fatalf("list intake failed: %d", listRec.Code)
	}
	var listResp struct {
		Items []postgres.DealershipFleetIntake `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].Status != "released" {
		t.Fatalf("expected 1 released intake record, got %d", len(listResp.Items))
	}

	// 11. Dealership report
	reportRec := send("GET", base+"/"+created.ID+"/report", nil, token)
	if reportRec.Code != 200 {
		t.Fatalf("get dealership report failed: %d: %s", reportRec.Code, reportRec.Body.String())
	}
	var report postgres.DealershipReport
	if err := json.Unmarshal(reportRec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.TotalReceived != 1 || report.TotalCompleted != 0 || report.TotalReleased != 1 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
}
