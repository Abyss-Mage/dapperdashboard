package workspace

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"dapper.local/platform/internal/postgres"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyInvitationHTTP(t *testing.T, ctx context.Context, store *postgres.Store, setup *pgxpool.Pool, actor, org, otherOrg, token string) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	origin := "https://127.0.0.1:8443"
	service, err := postgres.NewInvitationService(store, origin, key)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithInvitations(store, origin, service)
	send := func(method, path, body, bearer, requestOrigin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", requestOrigin)
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/api/v1/organizations/" + org + "/access/invitations"
	payload := `{"email":"isolated-http-invited@example.invalid","expiresInMinutes":60}`
	if send("POST", path, payload, "", origin).Code != 401 {
		t.Fatal("anonymous invitation creation accepted")
	}
	if send("POST", strings.Replace(path, org, otherOrg, 1), payload, token, origin).Code != 403 {
		t.Fatal("foreign invitation creation accepted")
	}
	if send("POST", path, payload, token, "https://foreign.invalid").Code != 403 {
		t.Fatal("foreign-origin invitation accepted")
	}
	if send("POST", path, `{"email":"isolated-http-invited@example.invalid"}`, token, origin).Code != 422 {
		t.Fatal("invitation missing explicit expiry accepted")
	}
	response := send("POST", path, payload, token, origin)
	var invitation postgres.MemberInvitation
	if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &invitation) != nil || invitation.DeliveryState != "queued" {
		t.Fatal("HTTP invitation queue failed", response.Code)
	}
	if strings.Contains(response.Body.String(), "token") || strings.Contains(response.Body.String(), "enrollmentURL") {
		t.Fatal("HTTP invitation exposed enrollment secret")
	}
	if send("POST", path, payload, token, origin).Code != 409 {
		t.Fatal("duplicate HTTP invitation accepted")
	}
	list := send("GET", path, "", token, origin)
	if list.Code != 200 || !strings.Contains(list.Body.String(), invitation.ID) || strings.Contains(list.Body.String(), "ciphertext") {
		t.Fatal("HTTP invitation metadata list failed")
	}
	var encrypted []byte
	if err = setup.QueryRow(ctx, `SELECT payload_ciphertext FROM member_invitation_deliveries WHERE invitation_id=$1`, invitation.ID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	seal, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	n := seal.NonceSize()
	plain, err := seal.Open(nil, encrypted[:n], encrypted[n:], []byte(org+"/"+invitation.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer clear(plain)
	var delivery map[string]string
	if err = json.Unmarshal(plain, &delivery); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(delivery["enrollmentURL"])
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	acceptBody, _ := json.Marshal(map[string]string{"token": fragment.Get("invitation"), "password": "Isolated HTTP enrollment password"})
	if send("POST", "/api/v1/auth/enroll", string(acceptBody), "", "https://foreign.invalid").Code != 403 {
		t.Fatal("foreign-origin anonymous enrollment accepted")
	}
	accepted := send("POST", "/api/v1/auth/enroll", string(acceptBody), "", origin)
	if accepted.Code != 204 || len(accepted.Result().Cookies()) != 0 {
		t.Fatal("enrollment failed or issued an implicit login session", accepted.Code)
	}
	if send("POST", "/api/v1/auth/enroll", string(acceptBody), "", origin).Code != 410 {
		t.Fatal("HTTP invitation token replay accepted")
	}
	var member string
	if err = setup.QueryRow(ctx, `SELECT principal_id::text FROM password_credentials WHERE email=$1`, invitation.Email).Scan(&member); err != nil {
		t.Fatal("HTTP enrollment credential missing")
	}
	var grants int
	if err = setup.QueryRow(ctx, `SELECT (SELECT count(*) FROM organization_grants WHERE principal_id=$1)+(SELECT count(*) FROM location_grants WHERE principal_id=$1)`, member).Scan(&grants); err != nil || grants != 0 {
		t.Fatal("HTTP enrollment granted business access", err)
	}
	for i := 0; i < 31; i++ {
		last := send("POST", "/api/v1/auth/enroll", `{"token":"invalid","password":""}`, "", origin)
		if i == 30 && last.Code != 429 {
			t.Fatal("public enrollment attempt limiter missing")
		}
	}

	// Test Dispatch Endpoint
	if send("POST", path+"/dispatch", "", token, origin).Code != 503 {
		t.Fatal("dispatch accepted without email sender configured")
	}
	if send("POST", path+"/dispatch", "", "", origin).Code != 401 {
		t.Fatal("anonymous dispatch accepted")
	}
	if send("POST", strings.Replace(path, org, otherOrg, 1)+"/dispatch", "", token, origin).Code != 403 {
		t.Fatal("foreign org dispatch accepted")
	}

	mockSender := &testSender{}
	configuredHandler := NewHandlerWithFullServices(store, origin, service, nil, &mockEmailSenderWrapper{sender: mockSender}, "", "")
	sendWithConfigured := func(method, p, b string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, p, strings.NewReader(b))
		r.Host = "127.0.0.1:8121"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		configuredHandler.ServeHTTP(w, r)
		return w
	}
	newPayload := `{"email":"dispatch-target@example.invalid","expiresInMinutes":30}`
	invResp := sendWithConfigured("POST", path, newPayload)
	if invResp.Code != 202 {
		t.Fatalf("failed to queue invitation: %d", invResp.Code)
	}
	var newInv postgres.MemberInvitation
	_ = json.Unmarshal(invResp.Body.Bytes(), &newInv)

	dispResp := sendWithConfigured("POST", path+"/dispatch", `{"limit":10}`)
	if dispResp.Code != 200 {
		t.Fatalf("dispatch failed: %d, body: %s", dispResp.Code, dispResp.Body.String())
	}
	var dispResult struct {
		Processed int                       `json:"processed"`
		Results   []postgres.DeliveryResult `json:"results"`
	}
	if err = json.Unmarshal(dispResp.Body.Bytes(), &dispResult); err != nil || dispResult.Processed != 1 {
		t.Fatalf("expected 1 processed delivery, got %d (err: %v)", dispResult.Processed, err)
	}
	var deliveryState, provRef string
	if err = setup.QueryRow(ctx, `SELECT state, provider_reference FROM member_invitation_deliveries WHERE invitation_id=$1`, newInv.ID).Scan(&deliveryState, &provRef); err != nil {
		t.Fatalf("failed to query delivery row: %v", err)
	}
	if deliveryState != "sent" || !strings.HasPrefix(provRef, "mock-email-") {
		t.Fatalf("expected state sent and mock-email prefix, got state: %s, ref: %s", deliveryState, provRef)
	}
}

type testSender struct{ called bool }

type mockEmailSenderWrapper struct {
	sender *testSender
}

func (m *mockEmailSenderWrapper) SendEmail(ctx context.Context, recipient, subject, bodyHTML, bodyText string) (string, error) {
	m.sender.called = true
	return "mock-email-ref-12345", nil
}
