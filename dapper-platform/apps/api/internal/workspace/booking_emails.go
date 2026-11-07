package workspace

import (
	"context"
	"log"
	"time"

	"dapper.local/platform/internal/postgres"
)

var bookingEmailZone = func() *time.Location {
	if z, err := time.LoadLocation("America/Toronto"); err == nil {
		return z
	}
	return time.UTC
}()

func whenText(t time.Time) string { return t.In(bookingEmailZone).Format("Monday, January 2 at 3:04 PM") }

// installBookingEmails sends the customer an email when a booking is cancelled or moved. It runs in the background and only when outgoing email works.
func installBookingEmails(store *postgres.Store, sender postgres.EmailSender) {
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
		if err != nil || f.Email == "" {
			return
		}
		subject, body, text := store.RenderEmail(ctx, e.Kind, map[string]string{"name": f.Name, "service": f.Service, "when": whenText(f.When), "where": f.Where})
		if _, err = sender.SendEmail(ctx, f.Email, subject, body, text); err != nil {
			log.Print("A booking email could not be sent")
		}
	})
}
