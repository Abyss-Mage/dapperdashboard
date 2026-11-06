package postgres

import (
	"context"
	"errors"
	"html"
	"strings"
	"unicode/utf8"
)

var ErrEmailTemplate = errors.New("that email wording is not valid")

// EmailTemplate is one system email as the owner sees it: its wording and the {{placeholders}} they may use in it.
type EmailTemplate struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	Placeholders []string `json:"placeholders"`
	Custom       bool     `json:"custom"`
}

type emailDefault struct {
	label, subject, body string
	placeholders         []string
	needsLink            bool
}

// The built-in wording. Plain text with {{placeholders}}; the HTML version is made from it when the email is sent.
var emailDefaults = map[string]emailDefault{
	"password_reset": {
		label:        "Password reset",
		subject:      "Reset your Dapper Detailing password",
		body:         "Someone asked to reset the password for this email address at Dapper Detailing.\n\nUse this link within one hour to choose a new password:\n{{link}}\n\nIf this was not you, ignore this email. Your password has not changed.",
		placeholders: []string{"link"},
		needsLink:    true,
	},
	"invitation": {
		label:        "Team invitation",
		subject:      "Set up your {{organization}} account",
		body:         "You have been invited to join {{organization}}.\n\nChoose your password here: {{link}}\n\nAfter you sign in you will confirm your name and mobile phone, add an authenticator app for two-factor sign-in (required for administrators, optional for employees), and read and acknowledge your company documents before you start.\n\nThis link works once and expires soon. If you were not expecting it, ignore this email.",
		placeholders: []string{"organization", "link"},
		needsLink:    true,
	},
	"booking_confirmation": {
		label:        "Booking confirmation",
		subject:      "Your Dapper Detailing booking is confirmed",
		body:         "Hi {{name}},\n\nThank you for booking with Dapper Detailing Pros. Your booking is confirmed and we are holding your time.\n\nService: {{service}}\nWhen: {{when}}\nTotal, tax included: {{total}}\nPayment: {{payment}}\nReference: {{reference}}\n\nIf anything needs to change, call us on (705) 977-6217.",
		placeholders: []string{"name", "service", "when", "total", "payment", "reference"},
	},
}

// EmailKeys lists the emails in the order the dashboard shows them.
var EmailKeys = []string{"password_reset", "invitation", "booking_confirmation"}

func fill(text string, vars map[string]string, escape bool) string {
	for k, v := range vars {
		if escape {
			v = html.EscapeString(v)
		}
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}

// renderEmail builds the subject, HTML and plain text from the wording. A {{link}} becomes a clickable link in the HTML.
func renderEmail(subject, body string, vars map[string]string) (string, string, string) {
	text := fill(body, vars, false)
	var b strings.Builder
	for _, para := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n") {
		p := html.EscapeString(para)
		for k, v := range vars {
			if k == "link" {
				p = strings.ReplaceAll(p, "{{link}}", `<a href="`+html.EscapeString(v)+`">`+html.EscapeString(v)+`</a>`)
			} else {
				p = strings.ReplaceAll(p, "{{"+k+"}}", html.EscapeString(v))
			}
		}
		b.WriteString("<p>" + strings.ReplaceAll(p, "\n", "<br>") + "</p>")
	}
	return fill(subject, vars, false), b.String(), text
}

// RenderEmail returns the owner's wording for this email if they saved one, else the built-in wording, filled in with the values.
func (s *Store) RenderEmail(ctx context.Context, key string, vars map[string]string) (subject, bodyHTML, bodyText string) {
	d := emailDefaults[key]
	sub, body := d.subject, d.body
	var savedSub, savedBody string
	if err := s.pool.QueryRow(ctx, `SELECT subject,body FROM email_templates WHERE template_key=$1 ORDER BY updated_at DESC LIMIT 1`, key).Scan(&savedSub, &savedBody); err == nil {
		sub, body = savedSub, savedBody
	}
	return renderEmail(sub, body, vars)
}

func (s *Store) EmailTemplates(ctx context.Context, actor, org string) ([]EmailTemplate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT template_key,subject,body FROM email_templates WHERE organization_id=$1`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	saved := map[string][2]string{}
	for rows.Next() {
		var k, sub, body string
		if err = rows.Scan(&k, &sub, &body); err != nil {
			return nil, err
		}
		saved[k] = [2]string{sub, body}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []EmailTemplate{}
	for _, k := range EmailKeys {
		d := emailDefaults[k]
		t := EmailTemplate{Key: k, Label: d.label, Subject: d.subject, Body: d.body, Placeholders: d.placeholders}
		if v, ok := saved[k]; ok {
			t.Subject, t.Body, t.Custom = v[0], v[1], true
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) SaveEmailTemplate(ctx context.Context, actor, org, key, subject, body string) error {
	d, ok := emailDefaults[key]
	subject, body = strings.TrimSpace(subject), strings.TrimSpace(body)
	if !ok || utf8.RuneCountInString(subject) < 1 || utf8.RuneCountInString(subject) > 200 || utf8.RuneCountInString(body) < 1 || utf8.RuneCountInString(body) > 5000 || d.needsLink && !strings.Contains(body, "{{link}}") {
		return ErrEmailTemplate
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO email_templates(organization_id,template_key,subject,body,updated_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(organization_id,template_key) DO UPDATE SET subject=$3,body=$4,updated_by=$5,updated_at=now()`, org, key, subject, body, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ResetEmailTemplate(ctx context.Context, actor, org, key string) error {
	if _, ok := emailDefaults[key]; !ok {
		return ErrEmailTemplate
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.messagingManager(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM email_templates WHERE organization_id=$1 AND template_key=$2`, org, key); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
