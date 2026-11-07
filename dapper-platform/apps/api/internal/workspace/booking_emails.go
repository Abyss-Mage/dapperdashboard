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

// installBookingEmails emails the customer when a booking is completed, cancelled or moved, and the business when one is cancelled or moved. It runs in the background and only when outgoing email works.
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
		if err != nil {
			return
		}
		vars := map[string]string{"name": f.Name, "phone": f.Phone, "email": f.Email, "service": f.Service, "when": whenText(f.When), "where": f.Where}
		if f.Email != "" {
			subject, body, text := store.RenderEmail(ctx, e.Kind, vars)
			if _, err = sender.SendEmail(ctx, f.Email, subject, body, text); err != nil {
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
