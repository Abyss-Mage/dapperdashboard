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

// MarkBookingEmailDelivered records that the claimed email really went out.
func (s *Store) MarkBookingEmailDelivered(ctx context.Context, bookingID, kind string) error {
	_, err := s.pool.Exec(ctx, `UPDATE booking_email_log SET delivered_at=now() WHERE booking_id=$1::uuid AND kind=$2 AND delivered_at IS NULL`, bookingID, kind)
	return err
}

// UndeliveredBookingEmails lists claimed emails that never went out. The first send gets two minutes to finish; after six hours they are dropped.
func (s *Store) UndeliveredBookingEmails(ctx context.Context) ([]BookingEmailEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT booking_id::text,kind FROM booking_email_log WHERE delivered_at IS NULL AND sent_at BETWEEN now()-interval '6 hours' AND now()-interval '2 minutes' LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BookingEmailEvent
	for rows.Next() {
		var e BookingEmailEvent
		if err = rows.Scan(&e.BookingID, &e.Kind); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// BookingPaymentText is the total and payment line of a booking confirmation, read from the booking's invoice.
func (s *Store) BookingPaymentText(ctx context.Context, bookingID string) (total, payment string) {
	var totalCents, paid int64
	var confirming bool
	if err := s.pool.QueryRow(ctx, `SELECT i.total_cents,i.paid_cents,EXISTS(SELECT 1 FROM square_payment_intents p WHERE p.invoice_id=i.id AND p.state IN ('submitting','uncertain')) FROM invoices i WHERE i.booking_id::text=$1 ORDER BY i.created_at DESC LIMIT 1`, bookingID).Scan(&totalCents, &paid, &confirming); err != nil {
		return "", "Pay on the day"
	}
	switch {
	case paid > 0 && paid >= totalCents:
		payment = "Paid in full online"
	case confirming:
		payment = "Card payment is being confirmed"
	default:
		payment = "Pay on the day"
	}
	return emailMoney(totalCents), payment
}

// AdminNotifyEmails lists where a business alert of this kind ("new", "cancelled" or "rescheduled") goes: the addresses saved for alerts,
// else the From address. Empty when that alert is switched off.
func (s *Store) AdminNotifyEmails(ctx context.Context, kind string) []string {
	var to string
	var on bool
	col := map[string]string{"new": "m.notify_new", "cancelled": "m.notify_cancelled", "rescheduled": "m.notify_rescheduled"}[kind]
	if col == "" {
		return nil
	}
	if err := s.pool.QueryRow(ctx, `SELECT coalesce(nullif(m.smtp_notify_email,''),m.smtp_from_address),`+col+` FROM messaging_settings m JOIN public_sites ps ON ps.organization_id=m.organization_id AND ps.slug=$1 LIMIT 1`, mainSiteSlug()).Scan(&to, &on); err != nil || !on {
		return nil
	}
	list, _ := notifyList(to)
	return list
}
