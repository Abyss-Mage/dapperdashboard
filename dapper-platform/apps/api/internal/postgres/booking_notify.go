package postgres

import (
	"context"
	"strings"
	"time"
)

// Booking emails. The store only announces what happened ("cancelled", "rescheduled"); the web layer, which owns the mail sender, writes and sends the
// email. Internal clean-ups (a booking released because its payment failed) use QuietBookingEvents so nobody is emailed about them.

type bookingEventKey struct{}

// QuietBookingEvents marks work that must not send booking emails.
func QuietBookingEvents(ctx context.Context) context.Context {
	return context.WithValue(ctx, bookingEventKey{}, true)
}

type BookingEmailEvent struct{ Kind, BookingID string }

func (s *Store) SetBookingEventHook(f func(BookingEmailEvent)) { s.bookingHook = f }

func (s *Store) emitBooking(ctx context.Context, kind, bookingID string) {
	if s.bookingHook == nil || ctx.Value(bookingEventKey{}) != nil || bookingID == "" {
		return
	}
	go s.bookingHook(BookingEmailEvent{Kind: kind, BookingID: bookingID})
}

// BookingEmailFacts is what a booking email says about the booking.
type BookingEmailFacts struct {
	Name, Email, Phone, Service, Where string
	When                               time.Time
}

func (s *Store) BookingEmailFacts(ctx context.Context, bookingID string) (BookingEmailFacts, error) {
	var f BookingEmailFacts
	var address string
	err := s.pool.QueryRow(ctx, `SELECT c.name,c.email,c.phone,b.service_name,b.booking_time,coalesce(b.service_address,'') FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id::text=$1`, bookingID).Scan(&f.Name, &f.Email, &f.Phone, &f.Service, &f.When, &address)
	f.Where = "At the Dapper shop"
	if strings.TrimSpace(address) != "" {
		f.Where = address
	}
	return f, err
}

// ClaimBookingEmail is true for the first caller to claim this kind of email for the booking. A booking is told "cancelled" once; other kinds
// are not repeated within two minutes, so two code paths reporting one change send one email.
func (s *Store) ClaimBookingEmail(ctx context.Context, bookingID, kind string) (bool, error) {
	window := "2 minutes"
	if kind == "booking_cancelled" {
		window = "100 years"
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO booking_email_log(booking_id,kind) SELECT $1::uuid,$2 WHERE NOT EXISTS(SELECT 1 FROM booking_email_log WHERE booking_id=$1::uuid AND kind=$2 AND sent_at>now()-$3::interval)`, bookingID, kind, window)
	return err == nil && tag.RowsAffected() == 1, err
}

// AdminNotifyEmail is where new-booking alerts go: the address saved for them, else the From address of the email settings.
func (s *Store) AdminNotifyEmail(ctx context.Context) string {
	var to string
	_ = s.pool.QueryRow(ctx, `SELECT coalesce(nullif(m.smtp_notify_email,''),m.smtp_from_address) FROM messaging_settings m JOIN public_sites ps ON ps.organization_id=m.organization_id AND ps.slug=$1 LIMIT 1`, mainSiteSlug()).Scan(&to)
	return to
}
