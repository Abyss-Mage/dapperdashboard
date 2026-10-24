package email

import (
	"context"
	"testing"
)

func TestSMTPSenderConfig(t *testing.T) {
	_, err := NewSMTPSender(Config{Host: "", FromAddress: "test@dapperdash.ca"})
	if err == nil {
		t.Fatal("expected error for empty host")
	}

	_, err = NewSMTPSender(Config{Host: "smtp.example.com", FromAddress: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid from address")
	}

	sender, err := NewSMTPSender(Config{
		Host:        "smtp.example.com",
		FromAddress: "notifications@dapperdash.ca",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.port != 587 {
		t.Errorf("expected default port 587, got %d", sender.port)
	}
	if sender.fromName != "Dapper Detailing" {
		t.Errorf("expected default fromName 'Dapper Detailing', got %s", sender.fromName)
	}

	ctx := context.Background()
	_, err = sender.SendEmail(ctx, "invalid-recipient", "Subject", "<p>hi</p>", "hi")
	if err == nil {
		t.Fatal("expected error for invalid recipient")
	}
}
