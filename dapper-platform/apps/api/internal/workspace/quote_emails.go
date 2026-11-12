package workspace

import (
	"context"
	"log"

	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/postgres"
)

// emailIssuedQuote sends the customer their quotation as a PDF and says what happened: "sent", "no_email", "not_configured" or "failed".
// The quotation is already issued by now, so a failed email is reported to staff and never undoes it.
func emailIssuedQuote(ctx context.Context, store *postgres.Store, sender postgres.EmailSender, q postgres.Quote) (status, to string) {
	if !emailReady(sender) {
		return "not_configured", ""
	}
	to, vars, err := store.QuoteEmailFacts(ctx, q)
	if err != nil {
		log.Printf("Quotation %s: the customer's email could not be read", q.ID)
		return "failed", ""
	}
	if to == "" {
		return "no_email", ""
	}
	subject, body, text := store.RenderEmail(ctx, "quotation_issued", vars)
	if s, ok := sender.(interface {
		SendEmailWithAttachment(context.Context, string, string, string, string, *email.Attachment) (string, error)
	}); ok {
		name, pdf := store.QuotePDF(ctx, q)
		_, err = s.SendEmailWithAttachment(ctx, to, subject, body, text, &email.Attachment{Name: name, ContentType: "application/pdf", Data: pdf})
	} else {
		_, err = sender.SendEmail(ctx, to, subject, body, text)
	}
	if err != nil {
		log.Printf("Quotation %s could not be emailed", q.ID)
		return "failed", to
	}
	return "sent", to
}
