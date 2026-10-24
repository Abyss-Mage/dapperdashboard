package mailchimp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) Do(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestConfigValidation(t *testing.T) {
	_, err := NewClient(Config{}, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty api key, got %v", err)
	}

	_, err = NewClient(Config{APIKey: "invalidkeywithnoprefix"}, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for missing prefix, got %v", err)
	}

	client, err := NewClient(Config{APIKey: "mysecretkey-us21", DefaultListID: "list123"}, nil)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client.serverPrefix != "us21" {
		t.Fatalf("expected server prefix us21, got %s", client.serverPrefix)
	}
}

func TestSubscriberHash(t *testing.T) {
	// MD5 of "test@example.com" lowercase is 55502f40dc8b7c769880b10874abc9d0
	hash := SubscriberHash("Test@Example.COM ")
	expected := "55502f40dc8b7c769880b10874abc9d0"
	if hash != expected {
		t.Fatalf("expected hash %s, got %s", expected, hash)
	}
}

func TestPing(t *testing.T) {
	ctx := context.Background()
	mock := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.String() != "https://us21.api.mailchimp.com/3.0/ping" {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			user, pass, ok := req.BasicAuth()
			if !ok || pass != "dummy-us21" {
				t.Fatalf("expected basic auth with dummy-us21, got user=%s, pass=%s, ok=%v", user, pass, ok)
			}
			body := io.NopCloser(bytes.NewReader([]byte(`{"health_status":"Everything's Chimpy!"}`)))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			}, nil
		},
	}

	client, err := NewClient(Config{APIKey: "dummy-us21"}, mock)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestAddOrUpdateMember(t *testing.T) {
	ctx := context.Background()
	mock := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", req.Method)
			}
			hash := SubscriberHash("customer@dapper.ca")
			expectedURL := "https://us21.api.mailchimp.com/3.0/lists/list_abc/members/" + hash
			if req.URL.String() != expectedURL {
				t.Fatalf("unexpected URL: %s vs %s", req.URL.String(), expectedURL)
			}
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if body["email_address"] != "customer@dapper.ca" {
				t.Fatalf("unexpected email_address: %v", body["email_address"])
			}
			if body["status"] != "subscribed" {
				t.Fatalf("unexpected status: %v", body["status"])
			}
			mf := body["merge_fields"].(map[string]any)
			if mf["FNAME"] != "Alex" || mf["LNAME"] != "Smith" {
				t.Fatalf("unexpected merge_fields: %v", mf)
			}
			respPayload := `{"id":"sub123","email_address":"customer@dapper.ca","status":"subscribed","merge_fields":{"FNAME":"Alex","LNAME":"Smith"},"tags":[{"id":1,"name":"VIP"}],"last_changed":"2026-10-23T12:00:00Z"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(respPayload))),
			}, nil
		},
	}

	client, err := NewClient(Config{APIKey: "dummy-us21", DefaultListID: "list_abc"}, mock)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	sub := Subscriber{
		Email:     "customer@dapper.ca",
		Status:    "subscribed",
		FirstName: "Alex",
		LastName:  "Smith",
		Tags:      []string{"VIP"},
	}
	resp, err := client.AddOrUpdateMember(ctx, "", sub)
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}
	if resp.ID != "sub123" || resp.Status != "subscribed" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestUnsubscribeMember(t *testing.T) {
	ctx := context.Background()
	mock := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body["status"] != "unsubscribed" {
				t.Fatalf("unexpected status: %v", body["status"])
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"unsubscribed"}`))),
			}, nil
		},
	}

	client, err := NewClient(Config{APIKey: "dummy-us21", DefaultListID: "list_abc"}, mock)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if err := client.UnsubscribeMember(ctx, "list_abc", "customer@dapper.ca"); err != nil {
		t.Fatalf("failed to unsubscribe: %v", err)
	}
}

func TestErrorHandling(t *testing.T) {
	ctx := context.Background()

	// 429 Rate limited
	rateLimitMock := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"title":"Rate limit exceeded"}`))),
			}, nil
		},
	}
	client, _ := NewClient(Config{APIKey: "dummy-us21", DefaultListID: "list_abc"}, rateLimitMock)
	_, err := client.AddOrUpdateMember(ctx, "", Subscriber{Email: "user@test.com"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	// 404 Not Found
	notFoundMock := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"title":"Resource Not Found"}`))),
			}, nil
		},
	}
	client404, _ := NewClient(Config{APIKey: "dummy-us21", DefaultListID: "list_abc"}, notFoundMock)
	_, err = client404.GetMember(ctx, "list_abc", "nonexistent@test.com")
	if !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("expected ErrMemberNotFound, got %v", err)
	}

	// Invalid email address
	_, err = client.AddOrUpdateMember(ctx, "", Subscriber{Email: "not-an-email"})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}
