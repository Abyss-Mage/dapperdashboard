package email

import (
	"context"
	"errors"
	"sync"
)

// ErrNotConfigured is returned when mail is sent before any SMTP settings exist.
var ErrNotConfigured = errors.New("email delivery is not configured")

// Live is a mail sender whose SMTP settings can change while the server runs, so settings saved in the dashboard take effect at
// once. Settings saved in the dashboard replace the server's own (the fallback) until they are removed.
type Live struct {
	mu       sync.RWMutex
	fallback *Config
	current  *SMTPSender
	saved    bool
}

// NewLive starts with the server's own settings, or none.
func NewLive(fallback *Config) (*Live, error) {
	l := &Live{fallback: fallback}
	if fallback != nil {
		sender, err := NewSMTPSender(*fallback)
		if err != nil {
			return nil, err
		}
		l.current = sender
	}
	return l, nil
}

// Set puts dashboard-saved settings in use.
func (l *Live) Set(cfg Config) error {
	sender, err := NewSMTPSender(cfg)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.current, l.saved = sender, true
	l.mu.Unlock()
	return nil
}

// Clear drops dashboard-saved settings and goes back to the server's own, if it has any.
func (l *Live) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.saved, l.current = false, nil
	if l.fallback != nil {
		if sender, err := NewSMTPSender(*l.fallback); err == nil {
			l.current = sender
		}
	}
}

// Configured reports whether mail can be sent. Saved reports whether the settings came from the dashboard.
func (l *Live) Configured() bool { l.mu.RLock(); defer l.mu.RUnlock(); return l.current != nil }
func (l *Live) Saved() bool      { l.mu.RLock(); defer l.mu.RUnlock(); return l.saved }

func (l *Live) SendEmail(ctx context.Context, recipient, subject, bodyHTML, bodyText string) (string, error) {
	l.mu.RLock()
	sender := l.current
	l.mu.RUnlock()
	if sender == nil {
		return "", ErrNotConfigured
	}
	return sender.SendEmail(ctx, recipient, subject, bodyHTML, bodyText)
}
