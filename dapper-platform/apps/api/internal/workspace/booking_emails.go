package workspace

import (
	"context"
	"log"
	"time"

	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/postgres"
)

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

// bookingAlerts maps a customer email to the business alert that goes with it: its template and which alert setting switches it on.
var bookingAlerts = map[string]struct{ template, setting string }{
	"booking_confirmation": {"booking_admin", "new"},
	"booking_cancelled":    {"booking_admin_cancelled", "cancelled"},
	"booking_rescheduled":  {"booking_admin_rescheduled", "rescheduled"},
}

// deliverBookingEmail writes one booking email from what is saved on the booking and sends it to the address saved on the customer, never one
// typed into the form. An error means the customer's email did not go out; the business alert follows it, so a retry does not repeat the alert.
func deliverBookingEmail(ctx context.Context, store *postgres.Store, sender postgres.EmailSender, origin, bookingID, kind string) error {
	if kind == "booking_confirmation" {
		store.IssueBookingDraft(ctx, bookingID)
	}
	vars, to, err := store.BookingEmailVars(ctx, bookingID, origin)
	if err != nil {
		return err
	}
	vars["invoice"] = store.BookingInvoiceText(ctx, bookingID, origin+"/account")
	if to != "" {
		subject, body, text := store.RenderEmail(ctx, kind, vars)
		if kind == "booking_confirmation" || kind == "booking_completed" {
			_, err = sendWithInvoicePDF(ctx, store, sender, bookingID, to, subject, body, text)
		} else {
			_, err = sender.SendEmail(ctx, to, subject, body, text)
		}
		if err != nil {
			log.Printf("Booking email %s for booking %s could not be sent; it will be tried again", kind, bookingID)
			return err
		}
	}
	if alert, ok := bookingAlerts[kind]; ok {
		subject, body, text := store.RenderEmail(ctx, alert.template, vars)
		for _, to := range store.AdminNotifyEmails(ctx, alert.setting) {
			if _, err = sender.SendEmail(ctx, to, subject, body, text); err != nil {
				log.Print("A business booking alert could not be sent")
			}
		}
	}
	return nil
}

func markBookingEmailDelivered(store *postgres.Store, bookingID, kind string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.MarkBookingEmailDelivered(ctx, bookingID, kind); err != nil {
		log.Printf("Booking email %s for booking %s was sent but could not be marked", kind, bookingID)
	}
}

// sendBookingEmail claims the email, so two code paths reporting one change send it once, then sends it.
func sendBookingEmail(store *postgres.Store, sender postgres.EmailSender, origin, bookingID, kind string) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if first, err := store.ClaimBookingEmail(ctx, bookingID, kind); err != nil || !first {
		return
	}
	if deliverBookingEmail(ctx, store, sender, origin, bookingID, kind) == nil {
		markBookingEmailDelivered(store, bookingID, kind)
	}
}

// installBookingEmails emails the customer when a booking is completed, cancelled or moved, and the business when one is cancelled or moved. It runs in the background and only when outgoing email works.
func installBookingEmails(store *postgres.Store, sender postgres.EmailSender, origin string) {
	store.SetBookingEventHook(func(e postgres.BookingEmailEvent) {
		if emailReady(sender) {
			sendBookingEmail(store, sender, origin, e.BookingID, e.Kind)
		}
	})
}

// RetryBookingEmails tries again, every 10 minutes, the booking emails that failed or were cut off by a restart.
func RetryBookingEmails(store *postgres.Store, sender postgres.EmailSender, origin string) {
	for {
		time.Sleep(10 * time.Minute)
		if !emailReady(sender) {
			continue
		}
		events, err := store.UndeliveredBookingEmails(context.Background())
		if err != nil {
			log.Print("Failed booking emails could not be listed; will retry")
			continue
		}
		for _, e := range events {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			if deliverBookingEmail(ctx, store, sender, origin, e.BookingID, e.Kind) == nil {
				markBookingEmailDelivered(store, e.BookingID, e.Kind)
			}
			cancel()
		}
	}
}
