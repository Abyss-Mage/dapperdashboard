package email

import (
	"context"
	"errors"
	"testing"
)

func TestLiveSettingsReplaceAndFallBack(t *testing.T) {
	empty, err := NewLive(nil)
	if err != nil || empty.Configured() || empty.Saved() {
		t.Fatal("an empty sender claims to be configured", err)
	}
	if _, err = empty.SendEmail(context.Background(), "a@example.invalid", "s", "<p>x</p>", "x"); !errors.Is(err, ErrNotConfigured) {
		t.Fatal("sending without settings did not say so", err)
	}
	if err = empty.Set(Config{Host: "", FromAddress: "a@example.invalid"}); err == nil {
		t.Fatal("settings without a server were accepted")
	}
	if empty.Configured() {
		t.Fatal("a refused setting changed the sender")
	}
	server := &Config{Host: "smtp.server.invalid", Port: 587, FromAddress: "server@example.invalid"}
	live, err := NewLive(server)
	if err != nil || !live.Configured() || live.Saved() {
		t.Fatal("the server's own settings were not used", err)
	}
	if err = live.Set(Config{Host: "smtp.saved.invalid", Port: 465, FromAddress: "saved@example.invalid"}); err != nil || !live.Saved() {
		t.Fatal("saved settings were not put in use", err)
	}
	live.Clear()
	if !live.Configured() || live.Saved() {
		t.Fatal("removing saved settings did not fall back to the server's own")
	}
	if _, err = NewLive(&Config{Host: "", FromAddress: "x"}); err == nil {
		t.Fatal("unusable server settings were accepted")
	}
}
