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
	type paid struct {
		id, invoice, method string
		amount, refunded    int64
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.invoice_id::text,p.method,p.amount_cents,p.refunded_cents FROM payments p
JOIN invoices i ON i.id=p.invoice_id AND i.organization_id=p.organization_id AND i.location_id=p.location_id
WHERE p.organization_id=$1 AND p.location_id=$2 AND i.booking_id=$3 AND p.idempotency_key=$4 AND p.status='completed'
 AND p.method IN ('cash','bank_transfer') AND p.refunded_cents<p.amount_cents FOR UPDATE OF p`, scope.OrganizationID, scope.LocationID, booking, "complete-"+booking)
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
 notes=btrim(notes||' Reversed: booking reopened.') WHERE id=$1`, p.id); err != nil {
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
		method := "cash"
		if p.method == "bank_transfer" {
			method = "e-transfer"
		}
		if err = recordInvoiceEvent(ctx, tx, scope, p.invoice, &status, next, fmt.Sprintf("Payment reversed: booking reopened (%s %s)", method, centsText(delta))); err != nil {
			return "", err
		}
		notes = append(notes, fmt.Sprintf("The %s payment of %s was reversed; the invoice is owed again.", method, centsText(delta)))
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

func centsText(c int64) string {
	return fmt.Sprintf("$%d.%02d", c/100, c%100)
}
