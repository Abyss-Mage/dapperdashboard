package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// reverseCompletionPayments undoes the cash or e-transfer payment the Complete dialog recorded for a booking (idempotency key
// "complete-<booking>") when staff move the booking back out of Completed, so its invoice is owed again. The payment is kept as
// a refunded row with an adjustment record, as a Square refund is, and its idempotency key is freed so completing the booking
// again records a new payment. Card payments taken through Square are left alone: those are refunded in Square. The returned
// sentence is shown to staff.
func reverseCompletionPayments(ctx context.Context, tx pgx.Tx, scope Scope, booking string) (string, error) {
	return reverseManualPayments(ctx, tx, scope, booking, true, "booking reopened")
}

// reverseManualPayments takes back the cash, e-transfer and cheque payments on a booking's invoices: only the one the Complete dialog
// recorded when onlyCompletion is set (a reopened booking), else all of them (a cancelled booking). why goes into the records.
func reverseManualPayments(ctx context.Context, tx pgx.Tx, scope Scope, booking string, onlyCompletion bool, why string) (string, error) {
	type paid struct {
		id, invoice, method string
		amount, refunded    int64
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.invoice_id::text,p.method,p.amount_cents,p.refunded_cents FROM payments p
JOIN invoices i ON i.id=p.invoice_id AND i.organization_id=p.organization_id AND i.location_id=p.location_id
WHERE p.organization_id=$1 AND p.location_id=$2 AND i.booking_id=$3 AND (NOT $5 OR p.idempotency_key=$4) AND p.status='completed'
 AND p.method IN ('cash','bank_transfer','cheque') AND p.refunded_cents<p.amount_cents FOR UPDATE OF p`, scope.OrganizationID, scope.LocationID, booking, "complete-"+booking, onlyCompletion)
	if err != nil {
		return "", err
	}
	var list []paid
	for rows.Next() {
		var p paid
		if err = rows.Scan(&p.id, &p.invoice, &p.method, &p.amount, &p.refunded); err != nil {
			rows.Close()
			return "", err
		}
		list = append(list, p)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return "", err
	}
	notes := []string{}
	for _, p := range list {
		delta := p.amount - p.refunded
		if _, err = tx.Exec(ctx, `UPDATE payments SET refunded_cents=amount_cents,status='refunded',idempotency_key=idempotency_key||':reversed:'||id::text,
 notes=btrim(notes||' Reversed: '||$2||'.') WHERE id=$1`, p.id, why); err != nil {
			return "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO payment_refund_adjustments(organization_id,location_id,payment_id,captured_cents,previous_refunded_cents,refunded_cents,delta_cents) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			scope.OrganizationID, scope.LocationID, p.id, p.amount, p.refunded, p.amount, delta); err != nil {
			return "", err
		}
		var status string
		var paidCents, total int64
		if err = tx.QueryRow(ctx, `SELECT status,paid_cents,total_cents FROM invoices WHERE id=$1 FOR UPDATE`, p.invoice).Scan(&status, &paidCents, &total); err != nil {
			return "", err
		}
		paidCents -= delta
		if paidCents < 0 {
			paidCents = 0
		}
		next := "partially_paid"
		if paidCents == 0 {
			next = "issued"
		} else if paidCents >= total {
			next = "paid"
		}
		if _, err = tx.Exec(ctx, `UPDATE invoices SET paid_cents=$1,due_cents=$2,status=$3,paid_at=CASE WHEN $3='paid' THEN paid_at ELSE NULL END,updated_at=now() WHERE id=$4`, paidCents, total-paidCents, next, p.invoice); err != nil {
			return "", err
		}
		method := map[string]string{"cash": "cash", "bank_transfer": "e-transfer", "cheque": "cheque"}[p.method]
		if err = recordInvoiceEvent(ctx, tx, scope, p.invoice, &status, next, fmt.Sprintf("Payment reversed: %s (%s %s)", why, method, centsText(delta))); err != nil {
			return "", err
		}
		if onlyCompletion {
			notes = append(notes, fmt.Sprintf("The %s payment of %s was reversed; the invoice is owed again.", method, centsText(delta)))
		} else {
			notes = append(notes, fmt.Sprintf("The %s payment of %s was reversed.", method, centsText(delta)))
		}
	}
	var card bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE p.organization_id=$1 AND p.location_id=$2 AND i.booking_id=$3
 AND p.status='completed' AND p.method IN ('square_card','square_terminal'))`, scope.OrganizationID, scope.LocationID, booking).Scan(&card); err != nil {
		return "", err
	}
	if card {
		notes = append(notes, "Card payments stay recorded; refund them in Square.")
	}
	return strings.Join(notes, " "), nil
}

// settleCancelledBooking takes back the money recorded by hand on a cancelled booking and voids its invoices that have nothing paid on
// them, so a cancelled booking leaves nothing owed and no takings in the reports. Card payments stay; those are refunded in Square. The
// returned sentence is shown to staff and kept in the booking's history.
func (s *Store) settleCancelledBooking(ctx context.Context, scope Scope, booking string) string {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ""
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return ""
	}
	notice, err := reverseManualPayments(ctx, tx, scope, booking, false, "booking cancelled")
	if err != nil || tx.Commit(ctx) != nil {
		return ""
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 AND status IN ('draft','issued') AND paid_cents=0`, scope.OrganizationID, scope.LocationID, booking)
	if err == nil {
		var ids []string
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		for _, id := range ids {
			_, _ = s.VoidInvoice(ctx, scope, id, "Booking cancelled")
		}
	}
	if notice != "" {
		_, _ = s.pool.Exec(ctx, `INSERT INTO booking_events(booking_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,'cancelled','cancelled',$5)`,
			booking, scope.OrganizationID, scope.LocationID, scope.ActorID, notice)
	}
	return notice
}

func centsText(c int64) string {
	return fmt.Sprintf("$%d.%02d", c/100, c%100)
}
