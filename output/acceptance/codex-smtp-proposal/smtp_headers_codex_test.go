package email

import (
	"context"
	"testing"
)

func TestCodexSMTPHeaderValidation(t *testing.T) {
	for _, cfg := range []Config{{Host: "localhost", FromAddress: "sender@example.test\r\nBcc: other@example.test"}, {Host: "localhost", FromAddress: "sender@example.test", FromName: "Name\r\nBcc: other@example.test"}, {Host: "localhost", FromAddress: "Display <sender@example.test>"}, {Host: "localhost", FromAddress: "sender@example.test", InsecureSkipVerify: true}} {
		if _, err := NewSMTPSender(cfg); err == nil {
			t.Fatal("accepted unsafe configuration")
		}
	}
	sender, err := NewSMTPSender(Config{Host: "127.0.0.1", Port: 1, FromAddress: "sender@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, probe := range []struct{ recipient, subject string }{{"recipient@example.test", "Subject\r\nBcc: other@example.test"}, {"recipient@example.test\r\nBcc: other@example.test", "Subject"}, {"Display <recipient@example.test>", "Subject"}} {
		if _, err := sender.SendEmail(context.Background(), probe.recipient, probe.subject, "", ""); err != ErrInvalidEmail {
			t.Fatalf("expected local rejection before network: %v", err)
		}
	}
}
