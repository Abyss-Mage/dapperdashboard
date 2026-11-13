package postgres

import (
	"strings"
	"context"
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

// AnnounceBooking reports a booking made by the team, so the customer gets the confirmation and the business its alert, like a website
// booking. A booking still waiting for approval is announced when it is approved instead.
func (s *Store) AnnounceBooking(ctx context.Context, b Booking) {
	if b.Status == "confirmed" {
		s.emitBooking(ctx, "booking_confirmation", b.ID)
	}
}

// IssueBookingDraft issues the booking's draft invoice, so the confirmation can carry the invoice with the balance due (a website booking's
// invoice is already issued). It acts as the website's own account, which may issue invoices (only at a location with a published website); any failure (no business details set up yet)
// just leaves the draft, and the email goes without the attachment.
func (s *Store) IssueBookingDraft(ctx context.Context, bookingID string) {
	var id, org, loc string
	if err := s.pool.QueryRow(ctx, `SELECT i.id::text,i.organization_id::text,i.location_id::text FROM invoices i WHERE i.booking_id::text=$1 AND i.status='draft' AND EXISTS(SELECT 1 FROM public_sites ps WHERE ps.organization_id=i.organization_id AND ps.location_id=i.location_id AND ps.published) ORDER BY i.created_at DESC LIMIT 1`, bookingID).Scan(&id, &org, &loc); err != nil {
		return
	}
	scope, err := s.websiteScopeAt(ctx, org, loc)
	if err != nil {
		return
	}
	_, _ = s.IssueInvoice(ctx, scope, id)
}

// statusOf is a booking's current status ("" if it cannot be read), so an approval can be told apart from a reopened booking.
func (s *Store) statusOf(ctx context.Context, id string) string {
	var st string
	_ = s.pool.QueryRow(ctx, `SELECT status FROM bookings WHERE id::text=$1`, id).Scan(&st)
	return st
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
	var method string
	// The booking's live invoice (a voided draft is not the bill), and how the money on it was actually taken.
	if err := s.pool.QueryRow(ctx, `SELECT i.total_cents,i.paid_cents,EXISTS(SELECT 1 FROM square_payment_intents p WHERE p.invoice_id=i.id AND p.state IN ('submitting','uncertain')),
 coalesce((SELECT p.method FROM payments p WHERE p.invoice_id=i.id AND p.status IN ('completed','refunded') ORDER BY p.recorded_at DESC LIMIT 1),'')
 FROM invoices i WHERE i.booking_id::text=$1 AND i.status<>'void' ORDER BY i.created_at DESC LIMIT 1`, bookingID).Scan(&totalCents, &paid, &confirming, &method); err != nil {
		return "", "Pay on the day"
	}
	how := map[string]string{"square_card": "by card online", "square_terminal": "by card (Square Reader)", "cash": "in cash", "bank_transfer": "by e-transfer", "cheque": "by cheque"}[method]
	switch {
	case paid > 0 && paid >= totalCents:
		payment = "Paid in full " + how
	case paid > 0:
		payment = "Part paid " + how + ", " + emailMoney(totalCents-paid) + " due"
	case confirming:
		payment = "Card payment is being confirmed"
	default:
		payment = "Pay on the day"
	}
	return emailMoney(totalCents), strings.TrimSpace(strings.ReplaceAll(payment, "  ", " "))
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
