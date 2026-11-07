package workspace

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/twilio"
)

func messagingFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrMessagingInput):
		reply(w, 422, map[string]string{"error": err.Error()})
	case errors.Is(err, postgres.ErrMessagingUnavailable):
		reply(w, 503, map[string]string{"error": "The server key that keeps these settings safe is not set, so they cannot be saved yet."})
	case errors.Is(err, postgres.ErrMessagingNotSet):
		reply(w, 422, map[string]string{"error": "Save the Twilio settings first."})
	default:
		failure(w, err)
	}
}

// emailReady is false for a missing sender or a Live sender that has no settings yet.
func emailReady(sender postgres.EmailSender) bool {
	if sender == nil {
		return false
	}
	if live, ok := sender.(*email.Live); ok {
		return live.Configured()
	}
	return true
}

// registerMessagingRoutes: owners enter the outgoing email (SMTP) and Twilio text settings in the dashboard. Saved email settings go
// into use at once; Twilio settings are stored and tested (no customer texts are sent yet). Passwords and tokens are never returned.
func registerMessagingRoutes(mux *http.ServeMux, store *postgres.Store, sender postgres.EmailSender) {
	live, _ := sender.(*email.Live)
	sms := twilio.NewClient()
	base := "/api/v1/organizations/{organization}/messaging"
	var mu sync.Mutex
	last := map[string]time.Time{}
	// one test message at a time per person, so a stuck button cannot send a flood
	allowTest := func(w http.ResponseWriter, r *http.Request) bool {
		mu.Lock()
		defer mu.Unlock()
		who := requestSession(r).principalID
		if t, ok := last[who]; ok && time.Since(t) < 8*time.Second {
			reply(w, 429, map[string]string{"error": "Wait a few seconds before sending another test."})
			return false
		}
		last[who] = time.Now()
		return true
	}
	org := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		id := r.PathValue("organization")
		if !uuid(id) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return "", false
		}
		return id, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		view, err := store.MessagingSettings(r.Context(), requestSession(r).principalID, id)
		if err != nil {
			messagingFailure(w, err)
			return
		}
		source := ""
		switch {
		case live != nil && live.Saved():
			source = "saved"
		case live != nil && live.Configured():
			source = "server"
		}
		reply(w, 200, map[string]any{"smtp": view.SMTP, "sms": view.SMS, "emailSource": source, "emailWorking": emailReady(sender)})
	})
	mux.HandleFunc("PUT "+base+"/smtp", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		var in postgres.SMTPSettings
		if !decodeLimited(w, r, &in, 8*1024) {
			return
		}
		actor := requestSession(r).principalID
		// Check the details make a usable sender before saving anything.
		probe := email.Config{Host: strings.TrimSpace(in.Host), Port: in.Port, Username: strings.TrimSpace(in.Username), Password: in.Password, FromAddress: strings.TrimSpace(in.FromAddress), FromName: strings.TrimSpace(in.FromName)}
		if _, err := email.NewSMTPSender(probe); err != nil {
			reply(w, 422, map[string]string{"error": "The email server and From address are not valid."})
			return
		}
		if err := store.SaveSMTPSettings(r.Context(), actor, id, in); err != nil {
			messagingFailure(w, err)
			return
		}
		if live != nil {
			saved, found, err := store.SavedSMTP(r.Context())
			if err != nil || !found {
				reply(w, 500, map[string]string{"error": "The settings were saved but could not be put into use. Restart the server."})
				return
			}
			if err = live.Set(email.Config{Host: saved.Host, Port: saved.Port, Username: saved.Username, Password: saved.Password, FromAddress: saved.FromAddress, FromName: saved.FromName}); err != nil {
				reply(w, 500, map[string]string{"error": "The settings were saved but could not be put into use. Restart the server."})
				return
			}
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/smtp", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		if err := store.ClearSMTPSettings(r.Context(), requestSession(r).principalID, id); err != nil {
			messagingFailure(w, err)
			return
		}
		if live != nil {
			live.Clear()
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	mux.HandleFunc("POST "+base+"/smtp/test", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		if _, err := store.MessagingSettings(r.Context(), requestSession(r).principalID, id); err != nil {
			messagingFailure(w, err)
			return
		}
		var in struct {
			To string `json:"to"`
		}
		if !decodeLimited(w, r, &in, 1024) {
			return
		}
		to := strings.TrimSpace(in.To)
		if addr, err := mail.ParseAddress(to); err != nil || addr.Address != to {
			reply(w, 422, map[string]string{"error": "Enter the email address to send the test to."})
			return
		}
		if !emailReady(sender) {
			reply(w, 422, map[string]string{"error": "Save the email settings first."})
			return
		}
		if !allowTest(w, r) {
			return
		}
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Now().Add(60 * time.Second))
		if _, err := sender.SendEmail(r.Context(), to, "Dapper test email", "<p>This is a test message from Dapper Detailing. Email is working.</p>", "This is a test message from Dapper Detailing. Email is working."); err != nil {
			// The server's own reason (it never contains the password) tells the owner what to fix, for example a mailbox that has SMTP sign-in switched off.
			why := strings.TrimPrefix(err.Error(), email.ErrSMTPSend.Error()+": ")
			if len(why) > 240 {
				why = why[:240]
			}
			reply(w, 502, map[string]string{"error": "The email server did not accept the message (" + why + "). Check the server, port, username, password and From address."})
			return
		}
		reply(w, 200, map[string]bool{"sent": true})
	})
	// The wording of the emails the system sends: the owner edits subject and text, with {{placeholders}}; deleting brings back the built-in wording.
	mux.HandleFunc("GET "+base+"/templates", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		items, err := store.EmailTemplates(r.Context(), requestSession(r).principalID, id)
		if err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("PUT "+base+"/templates/{key}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		var in struct {
			Subject string `json:"subject"`
			Body    string `json:"body"`
			IsHTML  bool   `json:"isHtml"`
		}
		if !decodeLimited(w, r, &in, 16*1024) {
			return
		}
		if err := store.SaveEmailTemplate(r.Context(), requestSession(r).principalID, id, r.PathValue("key"), in.Subject, in.Body, in.IsHTML); err != nil {
			if errors.Is(err, postgres.ErrEmailTemplate) {
				reply(w, 422, map[string]string{"error": "The subject and text are required, and a reset or invitation email must keep its {{link}}."})
				return
			}
			messagingFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/templates/{key}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		if err := store.ResetEmailTemplate(r.Context(), requestSession(r).principalID, id, r.PathValue("key")); err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"reset": true})
	})
	mux.HandleFunc("PUT "+base+"/sms", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		var in postgres.SMSSettings
		if !decodeLimited(w, r, &in, 4*1024) {
			return
		}
		if err := store.SaveSMSSettings(r.Context(), requestSession(r).principalID, id, in); err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("DELETE "+base+"/sms", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		if err := store.ClearSMSSettings(r.Context(), requestSession(r).principalID, id); err != nil {
			messagingFailure(w, err)
			return
		}
		reply(w, 200, map[string]bool{"removed": true})
	})
	// Check the saved Twilio details (to is empty: nothing is sent) or send one test text (to is a phone number).
	mux.HandleFunc("POST "+base+"/sms/test", func(w http.ResponseWriter, r *http.Request) {
		id, ok := org(w, r)
		if !ok {
			return
		}
		if _, err := store.MessagingSettings(r.Context(), requestSession(r).principalID, id); err != nil {
			messagingFailure(w, err)
			return
		}
		var in struct {
			To string `json:"to"`
		}
		if !decodeLimited(w, r, &in, 1024) {
			return
		}
		saved, found, err := store.SavedSMS(r.Context())
		if err != nil || !found {
			messagingFailure(w, postgres.ErrMessagingNotSet)
			return
		}
		cfg := twilio.Config{AccountSID: saved.AccountSID, AuthToken: saved.AuthToken, From: saved.From}
		if !allowTest(w, r) {
			return
		}
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Now().Add(60 * time.Second))
		if strings.TrimSpace(in.To) == "" {
			if err = sms.Verify(r.Context(), cfg); err != nil {
				reply(w, 422, map[string]string{"error": err.Error()})
				return
			}
			reply(w, 200, map[string]any{"verified": true})
			return
		}
		if _, err = sms.Send(r.Context(), cfg, in.To, "Test message from Dapper Detailing. Text messages are working."); err != nil {
			reply(w, 422, map[string]string{"error": err.Error()})
			return
		}
		reply(w, 200, map[string]any{"sent": true})
	})
}
