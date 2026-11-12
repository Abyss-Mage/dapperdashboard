package postgres

import (
	"context"
	"errors"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrEmailTemplate = errors.New("that email wording is not valid")

// EmailTemplate is one system email as the owner sees it: its wording and the {{placeholders}} they may use in it. The wording is plain
// text (blank lines start a paragraph) or, when IsHTML is set, the HTML of the email itself.
type EmailTemplate struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	IsHTML       bool     `json:"isHtml"`
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
		label:        "Booking: to the customer",
		subject:      "Your Dapper Detailing booking is confirmed",
		body:         "Hi {{name}},\n\nThank you for booking with Dapper Detailing Pros. Your booking is confirmed and we are holding your time.\n\nService: {{service}}\nWhen: {{when}}\nWhere: {{where}}\nTotal, tax included: {{total}}\nPayment: {{payment}}\n\n{{invoice}}\n\nIf anything needs to change, call us on (705) 977-6217.",
		placeholders: []string{"name", "service", "when", "where", "total", "payment", "invoice"},
	},
	"booking_admin": {
		label:        "Booking: to the business",
		subject:      "New booking: {{service}} for {{name}}",
		body:         "A new booking was made on the website.\n\nCustomer: {{name}}\nPhone: {{phone}}\nEmail: {{email}}\nService: {{service}}\nWhen: {{when}}\nWhere: {{where}}\nTotal, tax included: {{total}}\nPayment: {{payment}}",
		placeholders: []string{"name", "phone", "email", "service", "when", "where", "total", "payment"},
	},
	"booking_admin_cancelled": {
		label:        "Business alert: booking cancelled",
		subject:      "Booking cancelled: {{service}} for {{name}}",
		body:         "A booking was cancelled.\n\nCustomer: {{name}}\nPhone: {{phone}}\nEmail: {{email}}\nService: {{service}}\nWas booked for: {{when}}\nWhere: {{where}}",
		placeholders: []string{"name", "phone", "email", "service", "when", "where"},
	},
	"booking_admin_rescheduled": {
		label:        "Business alert: booking rescheduled",
		subject:      "Booking moved: {{service}} for {{name}}",
		body:         "A booking was moved to a new time.\n\nCustomer: {{name}}\nPhone: {{phone}}\nEmail: {{email}}\nService: {{service}}\nNew time: {{when}}\nWhere: {{where}}",
		placeholders: []string{"name", "phone", "email", "service", "when", "where"},
	},
	"booking_cancelled": {
		label:        "Booking cancelled",
		subject:      "Your Dapper Detailing booking was cancelled",
		body:         "Hi {{name}},\n\nYour booking has been cancelled.\n\nService: {{service}}\nWas booked for: {{when}}\n\nIf this is a mistake, call us on (705) 977-6217.",
		placeholders: []string{"name", "service", "when"},
	},
	"booking_completed": {
		label:        "Booking completed",
		subject:      "Thank you for choosing Dapper Detailing",
		body:         "Hi {{name}},\n\nYour service is complete. Thank you for choosing Dapper Detailing Pros.\n\nService: {{service}}\nDone on: {{when}}\n\n{{invoice}}\n\nIf anything is not right, call us on (705) 977-6217 and we will make it right.",
		placeholders: []string{"name", "service", "when", "invoice"},
	},
	"quotation_issued": {
		label:        "Quotation sent to customer",
		subject:      "Your Dapper Detailing quotation {{reference}}",
		body:         "Hi {{name}},\n\nYour quotation {{reference}} is attached.\n\nTotal: {{total}}\n{{valid_until}}\n\nTo go ahead or ask a question, call us on (705) 977-6217.",
		placeholders: []string{"name", "reference", "total", "valid_until"},
	},
	"dealer_invoice_issued": {
		label:        "Dealership invoice sent",
		subject:      "Invoice {{number}} from Dapper Detailing Pros",
		body:         "Hi {{name}},\n\nInvoice {{number}} is attached.\n\nTotal: {{total}}\nDue by: {{due_date}}\n\nView, download and pay it online (no sign-in needed): {{link}}\n\nQuestions about this invoice? Call us on (705) 977-6217.",
		placeholders: []string{"name", "number", "total", "due_date", "link"},
		needsLink:    true,
	},
	"booking_rescheduled": {
		label:        "Booking rescheduled",
		subject:      "Your Dapper Detailing booking has a new time",
		body:         "Hi {{name}},\n\nYour booking has been moved to a new time.\n\nService: {{service}}\nNew time: {{when}}\nWhere: {{where}}\n\nIf this does not suit you, call us on (705) 977-6217.",
		placeholders: []string{"name", "service", "when", "where"},
	},
}

// EmailKeys lists the emails in the order the dashboard shows them.
var EmailKeys = []string{"booking_confirmation", "booking_admin", "booking_admin_cancelled", "booking_admin_rescheduled", "booking_completed", "booking_cancelled", "booking_rescheduled", "quotation_issued", "dealer_invoice_issued", "password_reset", "invitation"}

func fill(text string, vars map[string]string, escape bool) string {
	for k, v := range vars {
		if escape {
			v = html.EscapeString(v)
		}
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}

var tag = regexp.MustCompile(`(?s)<[^>]*>`)

// htmlToText is the plain-text twin of an HTML email: line breaks kept, tags dropped.
func htmlToText(h string) string {
	h = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</h[1-6]>|</tr>|</li>`).ReplaceAllString(h, "\n")
	h = regexp.MustCompile(`(?is)<(style|script)[^>]*>.*?</(style|script)>`).ReplaceAllString(h, "")
	t := html.UnescapeString(tag.ReplaceAllString(h, ""))
	return strings.TrimSpace(regexp.MustCompile(`\n{3,}`).ReplaceAllString(t, "\n\n"))
}

// renderEmail builds the subject, HTML and plain text from the wording. A {{link}} becomes a clickable link in the HTML of a plain-text email.
func renderEmail(subject, body string, isHTML bool, vars map[string]string) (string, string, string) {
	if isHTML {
		// Values are escaped; the owner's own HTML is trusted (only the owner can change it).
		out := fill(body, vars, true)
		return fill(subject, vars, false), out, htmlToText(out)
	}
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
	sub, body, isHTML := d.subject, d.body, false
	var savedSub, savedBody string
	var savedHTML bool
	if err := s.pool.QueryRow(ctx, `SELECT a.subject,a.body,a.is_html FROM email_templates a JOIN public_sites ps ON ps.organization_id=a.organization_id AND ps.slug=$2 WHERE a.template_key=$1 LIMIT 1`, key, mainSiteSlug()).Scan(&savedSub, &savedBody, &savedHTML); err == nil {
		sub, body, isHTML = savedSub, savedBody, savedHTML
	}
	return renderEmail(sub, body, isHTML, vars)
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
	rows, err := tx.Query(ctx, `SELECT template_key,subject,body,is_html FROM email_templates WHERE organization_id=$1`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type saved struct {
		sub, body string
		html      bool
	}
	have := map[string]saved{}
	for rows.Next() {
		var k string
		var v saved
		if err = rows.Scan(&k, &v.sub, &v.body, &v.html); err != nil {
			return nil, err
		}
		have[k] = v
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []EmailTemplate{}
	for _, k := range EmailKeys {
		d := emailDefaults[k]
		t := EmailTemplate{Key: k, Label: d.label, Subject: d.subject, Body: d.body, Placeholders: d.placeholders}
		if v, ok := have[k]; ok {
			t.Subject, t.Body, t.IsHTML, t.Custom = v.sub, v.body, v.html, true
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) SaveEmailTemplate(ctx context.Context, actor, org, key, subject, body string, isHTML bool) error {
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
	if _, err = tx.Exec(ctx, `INSERT INTO email_templates(organization_id,template_key,subject,body,is_html,updated_by) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(organization_id,template_key) DO UPDATE SET subject=$3,body=$4,is_html=$5,updated_by=$6,updated_at=now()`, org, key, subject, body, isHTML, actor); err != nil {
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
