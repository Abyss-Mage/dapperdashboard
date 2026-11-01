package twilio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	testSID   = "AC00000000000000000000000000000000"
	testToken = "0123456789abcdef0123456789abcdef"
)

func TestSendAndVerify(t *testing.T) {
	var gotForm url.Values
	var gotUser, gotPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		gotUser, gotPass = user, pass
		if pass != testToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/2010-04-01/Accounts/"+testSID+".json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sid":"` + testSID + `"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/2010-04-01/Accounts/"+testSID+"/Messages.json":
			_ = r.ParseForm()
			gotForm = r.PostForm
			if gotForm.Get("To") == "+15555550199" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"code":21211,"message":"The 'To' number is not a valid phone number."}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sid":"SM123"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{BaseURL: server.URL, HTTP: server.Client()}
	ctx := context.Background()
	cfg := Config{AccountSID: testSID, AuthToken: testToken, From: "+17055550100"}

	if err := client.Verify(ctx, cfg); err != nil || gotUser != testSID {
		t.Fatal("working settings did not verify", err, gotUser)
	}
	bad := cfg
	bad.AuthToken = "ffffffffffffffffffffffffffffffff"
	if err := client.Verify(ctx, bad); !errors.Is(err, ErrAuth) {
		t.Fatal("a wrong token was accepted", err)
	}
	if err := client.Verify(ctx, Config{AccountSID: "nope", AuthToken: testToken, From: "+17055550100"}); !errors.Is(err, ErrConfig) {
		t.Fatal("a malformed Account SID was accepted", err)
	}

	sid, err := client.Send(ctx, cfg, "+1 (705) 555-0123", "Hello from the test")
	if err != nil || sid != "SM123" {
		t.Fatal("send failed", sid, err)
	}
	if _, err = client.Send(ctx, cfg, "7055550123", "x"); !errors.Is(err, ErrPhone) {
		t.Fatal("a number without a country code was sent", err)
	}
	sid, err = client.Send(ctx, cfg, "+1 (705) 555-0123", "Hello")
	if err != nil || gotForm.Get("To") != "+17055550123" || gotForm.Get("From") != "+17055550100" || gotForm.Get("Body") != "Hello" || gotForm.Get("MessagingServiceSid") != "" {
		t.Fatal("form wrong", err, gotForm)
	}
	service := cfg
	service.From = "MG00000000000000000000000000000000"
	if _, err = client.Send(ctx, service, "+17055550123", "Hello"); err != nil || gotForm.Get("MessagingServiceSid") != service.From || gotForm.Get("From") != "" {
		t.Fatal("messaging service not used", err, gotForm)
	}
	if _, err = client.Send(ctx, cfg, "+15555550199", "Hello"); err == nil || !strings.Contains(err.Error(), "not a valid phone number") {
		t.Fatal("Twilio's refusal was not passed on", err)
	}
	if _, err = client.Send(ctx, cfg, "+17055550123", strings.Repeat("x", 1601)); !errors.Is(err, ErrConfig) {
		t.Fatal("a message over 1600 characters was sent", err)
	}
	if gotPass != testToken {
		t.Fatal("the token was not sent as the basic-auth password")
	}
}
