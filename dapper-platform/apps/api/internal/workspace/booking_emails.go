package workspace

import (
	"context"
	"log"
	"time"

	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/postgres"
)

var bookingEmailZone = func() *time.Location {
	if z, err := time.LoadLocation("America/Toronto"); err == nil {
		return z
	}
	return time.UTC
}()

func whenText(t time.Time) string { return t.In(bookingEmailZone).Format("Monday, January 2 at 3:04 PM") }

// sendWithInvoicePDF sends a customer email with the booking's invoice attached as a PDF, or without it when there is no issued
// invoice yet or the sender cannot attach files (test senders).
func sendWithInvoicePDF(ctx context.Context, store *postgres.Store, sender postgres.EmailSender, bookingID, to, subject, body, text string) (string, error) {
	if s, ok := sender.(interface {
		SendEmailWithAttachment(context.Context, string, string, string, string, *email.Attachment) (string, error)
	}); ok {
		if name, pdf := store.BookingInvoicePDF(ctx, bookingID); pdf != nil {
			return s.SendEmailWithAttachment(ctx, to, subject, body, text, &email.Attachment{Name: name, ContentType: "application/pdf", Data: pdf})
		}
	}
	return sender.SendEmail(ctx, to, subject, body, text)
}

// installBookingEmails emails the customer when a booking is completed, cancelled or moved, and the business when one is cancelled or moved. It runs in the background and only when outgoing email works.
func installBookingEmails(store *postgres.Store, sender postgres.EmailSender, origin string) {
	store.SetBookingEventHook(func(e postgres.BookingEmailEvent) {
		if !emailReady(sender) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		if first, err := store.ClaimBookingEmail(ctx, e.BookingID, e.Kind); err != nil || !first {
			return
		}
		f, err := store.BookingEmailFacts(ctx, e.BookingID)
		if err != nil {
			return
		}
		vars := map[string]string{"name": f.Name, "phone": f.Phone, "email": f.Email, "service": f.Service, "when": whenText(f.When), "where": f.Where, "invoice": store.BookingInvoiceText(ctx, e.BookingID, origin+"/account")}
		if f.Email != "" {
			subject, body, text := store.RenderEmail(ctx, e.Kind, vars)
			if _, err = sendWithInvoicePDF(ctx, store, sender, e.BookingID, f.Email, subject, body, text); err != nil {
				log.Print("A booking email could not be sent")
			}
		}
		// The business hears about cancellations and new times too, when those alerts are switched on.
		alert := map[string]string{"booking_cancelled": "cancelled", "booking_rescheduled": "rescheduled"}[e.Kind]
		if alert == "" {
			return
		}
		subject, body, text := store.RenderEmail(ctx, "booking_admin_"+alert, vars)
		for _, to := range store.AdminNotifyEmails(ctx, alert) {
			if _, err = sender.SendEmail(ctx, to, subject, body, text); err != nil {
				log.Print("A business booking alert could not be sent")
			}
		}
	})
}
