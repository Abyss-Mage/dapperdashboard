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

func verifyMarketingHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, handler http.Handler, actor, org, loc, token, origin string) {
	t.Helper()

	base := "/api/v1/organizations/" + org + "/locations/" + loc + "/marketing"

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

	// 1. Without grants: 403 Forbidden
	if send("GET", base+"/popups", nil, token).Code != 403 {
		t.Fatal("marketing popup list allowed without grant")
	}

	// 2. Grant marketing.read and marketing.manage
	for _, action := range []string{"marketing.read", "marketing.manage"} {
		if _, err := setup.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, actor, org, loc, action); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Update & read Mailchimp settings
	setReq := postgres.UpdateMailchimpSettingsInput{
		APIKeyConfigured: true,
		ServerPrefix:     "us21",
		DefaultListID:    "dapper_vip_list",
		DoubleOptIn:      true,
		SyncEnabled:      true,
	}
	setResp := send("PUT", base+"/settings", setReq, token)
	if setResp.Code != 200 {
		t.Fatalf("expected 200 for update settings, got %d: %s", setResp.Code, setResp.Body.String())
	}
	getSetResp := send("GET", base+"/settings", nil, token)
	if getSetResp.Code != 200 {
		t.Fatalf("expected 200 for get settings, got %d: %s", getSetResp.Code, getSetResp.Body.String())
	}
	var settings postgres.MailchimpSettings
	_ = json.Unmarshal(getSetResp.Body.Bytes(), &settings)
	if settings.ServerPrefix != "us21" || settings.DefaultListID != "dapper_vip_list" {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	// 4. Create Popup Form
	createFormReq := postgres.CreatePopupFormInput{
		Name:        "Summer Detail Promo",
		Title:       "Save $25 on Ceramic Coating",
		Headline:    "VIP Summer Club",
		Description: "Subscribe to our VIP club and get exclusive detailing perks.",
		Fields: []postgres.PopupFormField{
			{Name: "email", Type: "email", Label: "Email Address", Required: true},
			{Name: "first_name", Type: "text", Label: "First Name", Required: false},
		},
		AudienceID:             "dapper_vip_list",
		Tags:                   []string{"SummerPromo"},
		DisplayDelaySeconds:    5,
		SelectedPages:          []string{"/", "/booking*"},
		RepeatDisplayLimitDays: 7,
		SuccessMessage:         "Check your email for your $25 promo code!",
		Active:                 true,
	}
	createResp := send("POST", base+"/popups", createFormReq, token)
	if createResp.Code != 201 {
		t.Fatalf("expected 201 for create popup, got %d: %s", createResp.Code, createResp.Body.String())
	}
	var createdForm postgres.PopupForm
	_ = json.Unmarshal(createResp.Body.Bytes(), &createdForm)
	if createdForm.ID == "" || createdForm.Name != "Summer Detail Promo" {
		t.Fatalf("unexpected created form: %+v", createdForm)
	}

	// 5. List Popup Forms
	listResp := send("GET", base+"/popups", nil, token)
	if listResp.Code != 200 {
		t.Fatalf("expected 200 for list popups, got %d", listResp.Code)
	}
	var formList []postgres.PopupForm
	_ = json.Unmarshal(listResp.Body.Bytes(), &formList)
	if len(formList) != 1 {
		t.Fatalf("expected 1 popup, got %d", len(formList))
	}

	// 6. Public popup matching (Unauthenticated)
	pubMatchResp := send("GET", "/api/v1/public/marketing/popups?organizationId="+org+"&locationId="+loc+"&path=/", nil, "")
	if pubMatchResp.Code != 200 {
		t.Fatalf("expected 200 for public popup query, got %d", pubMatchResp.Code)
	}
	var pubForms []postgres.PopupForm
	_ = json.Unmarshal(pubMatchResp.Body.Bytes(), &pubForms)
	if len(pubForms) != 1 || pubForms[0].ID != createdForm.ID {
		t.Fatalf("expected matching public popup, got %+v", pubForms)
	}

	// 7. Public submission (Unauthenticated)
	subReq := postgres.PublicPopupSubmissionInput{
		Email:     "lead_customer@example.ca",
		FirstName: "Morgan",
		LastName:  "Chase",
		Metadata:  map[string]any{"source": "landing_page"},
	}
	subResp := send("POST", "/api/v1/public/marketing/popups/"+createdForm.ID+"/submit", subReq, "")
	if subResp.Code != 201 {
		t.Fatalf("expected 201 for public popup submit, got %d: %s", subResp.Code, subResp.Body.String())
	}

	// 8. Staff lists submissions
	subsResp := send("GET", base+"/popups/"+createdForm.ID+"/submissions", nil, token)
	if subsResp.Code != 200 {
		t.Fatalf("expected 200 for list submissions, got %d: %s", subsResp.Code, subsResp.Body.String())
	}
	var submissions []postgres.PopupSubmission
	_ = json.Unmarshal(subsResp.Body.Bytes(), &submissions)
	if len(submissions) != 1 || submissions[0].Email != "lead_customer@example.ca" {
		t.Fatalf("unexpected submissions list: %+v", submissions)
	}

	// 9. Staff lists marketing consents (CASL tracking)
	consentsResp := send("GET", base+"/consents", nil, token)
	if consentsResp.Code != 200 {
		t.Fatalf("expected 200 for list consents, got %d", consentsResp.Code)
	}
	var consentResult struct {
		Consents []postgres.MarketingConsent `json:"consents"`
		Total    int                         `json:"total"`
	}
	_ = json.Unmarshal(consentsResp.Body.Bytes(), &consentResult)
	if consentResult.Total < 1 {
		t.Fatalf("expected at least 1 consent, got %d", consentResult.Total)
	}

	// 10. Update & Delete popup form
	updateReq := postgres.UpdatePopupFormInput{
		Name:                   createdForm.Name,
		Title:                  "Updated Title",
		Description:            createdForm.Description,
		Headline:               createdForm.Headline,
		Fields:                 createdForm.Fields,
		AudienceID:             createdForm.AudienceID,
		Tags:                   createdForm.Tags,
		DisplayDelaySeconds:    createdForm.DisplayDelaySeconds,
		SelectedPages:          createdForm.SelectedPages,
		RepeatDisplayLimitDays: createdForm.RepeatDisplayLimitDays,
		SuccessMessage:         createdForm.SuccessMessage,
		Active:                 false,
	}
	updateResp := send("PUT", base+"/popups/"+createdForm.ID, updateReq, token)
	if updateResp.Code != 200 {
		t.Fatalf("expected 200 for update popup, got %d", updateResp.Code)
	}

	deleteResp := send("DELETE", base+"/popups/"+createdForm.ID, nil, token)
	if deleteResp.Code != 200 {
		t.Fatalf("expected 200 for delete popup, got %d", deleteResp.Code)
	}
}
