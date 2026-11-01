package postgres

import (
	"bytes"
	"errors"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMessagingSettings(t *testing.T) {
	dsn := os.Getenv("DAPPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("real isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer admin.Close()
	namespace := fmt.Sprintf("dapper_msg_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{namespace}.Sanitize()
	if _, err = admin.pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		_, _ = admin.pool.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE")
	}()
	config := admin.pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = namespace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var org, owner, stranger string
	must(pool.QueryRow(ctx, `INSERT INTO organizations(name) VALUES('Synthetic Messaging Org') RETURNING id::text`).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:msg-owner') RETURNING id::text`).Scan(&owner))
	must(pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic:msg-stranger') RETURNING id::text`).Scan(&stranger))
	_, err = pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage')`, owner, org)
	must(err)
	smtp := SMTPSettings{Host: "smtp.example.invalid", Port: 587, Username: "mailer", Password: "synthetic-smtp-password", FromAddress: "hello@example.invalid", FromName: "Synthetic Shop"}
	if err = s.SaveSMTPSettings(ctx, owner, org, smtp); !errors.Is(err, ErrMessagingUnavailable) {
		t.Fatal("saved without a server key", err)
	}
	must(s.SetMFAKey(bytes.Repeat([]byte{7}, 32)))
	if err = s.SaveSMTPSettings(ctx, stranger, org, smtp); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access management saved email settings", err)
	}
	if _, err = s.MessagingSettings(ctx, stranger, org); !errors.Is(err, ErrForbidden) {
		t.Fatal("a member without access management read the settings", err)
	}
	for _, bad := range []SMTPSettings{{Host: "", Port: 587, FromAddress: "a@example.invalid"}, {Host: "h", Port: 0, FromAddress: "a@example.invalid"}, {Host: "h", Port: 587, FromAddress: "not-an-address"}, {Host: "bad host", Port: 587, FromAddress: "a@example.invalid"}} {
		if err = s.SaveSMTPSettings(ctx, owner, org, bad); !errors.Is(err, ErrMessagingInput) {
			t.Fatal("bad email settings accepted", bad, err)
		}
	}
	must(s.SaveSMTPSettings(ctx, owner, org, smtp))
	view, err := s.MessagingSettings(ctx, owner, org)
	must(err)
	if !view.SMTP.Saved || !view.SMTP.PasswordSet || view.SMTP.Host != smtp.Host || view.SMTP.Password != "" || view.SMS.Saved {
		t.Fatalf("view wrong: %+v", view)
	}
	var raw []byte
	must(pool.QueryRow(ctx, `SELECT smtp_password_sealed FROM messaging_settings WHERE organization_id=$1`, org).Scan(&raw))
	if len(raw) == 0 || bytes.Contains(raw, []byte(smtp.Password)) {
		t.Fatal("the password is not sealed in the database")
	}
	saved, found, err := s.SavedSMTP(ctx)
	if err != nil || !found || saved.Password != smtp.Password {
		t.Fatal("the server cannot read the saved email password back", err)
	}
	// a blank password keeps the saved one
	next := smtp
	next.Password, next.Host = "", "smtp2.example.invalid"
	must(s.SaveSMTPSettings(ctx, owner, org, next))
	if saved, _, err = s.SavedSMTP(ctx); err != nil || saved.Host != "smtp2.example.invalid" || saved.Password != smtp.Password {
		t.Fatal("a blank password replaced the saved one", saved, err)
	}

	sms := SMSSettings{AccountSID: "AC00000000000000000000000000000000", From: "+1 (705) 555-0100"}
	if err = s.SaveSMSSettings(ctx, owner, org, sms); !errors.Is(err, ErrMessagingInput) {
		t.Fatal("the first Twilio save needs an Auth Token", err)
	}
	sms.AuthToken = "0123456789abcdef0123456789abcdef"
	if err = s.SaveSMSSettings(ctx, owner, org, SMSSettings{AccountSID: "nope", AuthToken: sms.AuthToken, From: sms.From}); !errors.Is(err, ErrMessagingInput) {
		t.Fatal("a bad Account SID was accepted", err)
	}
	if err = s.SaveSMSSettings(ctx, owner, org, SMSSettings{AccountSID: sms.AccountSID, AuthToken: sms.AuthToken, From: "7055550100"}); !errors.Is(err, ErrMessagingInput) {
		t.Fatal("a sending number without a country code was accepted", err)
	}
	must(s.SaveSMSSettings(ctx, owner, org, sms))
	smsSaved, found, err := s.SavedSMS(ctx)
	if err != nil || !found || smsSaved.AuthToken != sms.AuthToken || smsSaved.From != "+17055550100" {
		t.Fatal("Twilio settings not stored properly", smsSaved, err)
	}
	must(pool.QueryRow(ctx, `SELECT twilio_auth_token_sealed FROM messaging_settings WHERE organization_id=$1`, org).Scan(&raw))
	if len(raw) == 0 || bytes.Contains(raw, []byte(sms.AuthToken)) {
		t.Fatal("the Twilio token is not sealed in the database")
	}
	service := SMSSettings{AccountSID: sms.AccountSID, From: "MG00000000000000000000000000000000"}
	must(s.SaveSMSSettings(ctx, owner, org, service))
	if smsSaved, _, err = s.SavedSMS(ctx); err != nil || smsSaved.From != service.From || smsSaved.AuthToken != sms.AuthToken {
		t.Fatal("a messaging service or the kept token was lost", smsSaved, err)
	}
	view, err = s.MessagingSettings(ctx, owner, org)
	if err != nil || !view.SMS.Saved || !view.SMS.TokenSet || view.SMS.AuthToken != "" || !view.SMTP.Saved {
		t.Fatalf("view wrong after both saved: %+v %v", view, err)
	}
	must(s.ClearSMTPSettings(ctx, owner, org))
	if _, found, err = s.SavedSMTP(ctx); err != nil || found {
		t.Fatal("email settings remain after removing them", err)
	}
	must(s.ClearSMSSettings(ctx, owner, org))
	if _, found, err = s.SavedSMS(ctx); err != nil || found {
		t.Fatal("Twilio settings remain after removing them", err)
	}
}
