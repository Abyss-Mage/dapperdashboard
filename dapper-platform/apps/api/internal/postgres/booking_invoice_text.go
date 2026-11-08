package postgres

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

type invoiceLine struct{ label, amount string }

// bookingInvoice loads the booking's latest issued invoice as lines: every item (the service, then each add-on), the discount, tax,
// total, what is paid and what is still due. ok is false when the booking has no issued invoice yet.
func (s *Store) bookingInvoice(ctx context.Context, bookingID string) (number string, lines []invoiceLine, ok bool) {
	var id, coupon string
	var discount, tax, total, paid, due int64
	err := s.pool.QueryRow(ctx, `SELECT id::text,invoice_number,coalesce(coupon_code,''),discount_cents,tax_cents,total_cents,paid_cents,due_cents FROM invoices
 WHERE booking_id::text=$1 AND status IN ('issued','partially_paid','paid') ORDER BY created_at DESC LIMIT 1`, bookingID).Scan(&id, &number, &coupon, &discount, &tax, &total, &paid, &due)
	if err != nil {
		return "", nil, false
	}
	rows, err := s.pool.Query(ctx, `SELECT description,quantity,total_price_cents FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return "", nil, false
	}
	defer rows.Close()
	for rows.Next() {
		var desc string
		var qty int
		var cents int64
		if rows.Scan(&desc, &qty, &cents) != nil {
			return "", nil, false
		}
		label := desc
		if len(lines) > 0 {
			label = "Add-on: " + desc
		}
		if qty > 1 {
			label = fmt.Sprintf("%s × %d", label, qty)
		}
		lines = append(lines, invoiceLine{label, emailMoney(cents)})
	}
	if rows.Err() != nil {
		return "", nil, false
	}
	if discount > 0 {
		label := "Discount"
		if coupon != "" {
			label += " (" + coupon + ")"
		}
		lines = append(lines, invoiceLine{label, "-" + emailMoney(discount)})
	}
	if tax > 0 {
		lines = append(lines, invoiceLine{"Tax", emailMoney(tax)})
	}
	lines = append(lines, invoiceLine{"Total", emailMoney(total)}, invoiceLine{"Paid", emailMoney(paid)}, invoiceLine{"Balance due", emailMoney(due)})
	return number, lines, true
}

// BookingInvoiceText writes the booking's invoice out for an email, with where to open it. Empty when the booking has no issued invoice yet.
func (s *Store) BookingInvoiceText(ctx context.Context, bookingID, accountURL string) string {
	number, lines, ok := s.bookingInvoice(ctx, bookingID)
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Invoice %s", number)
	for _, l := range lines {
		fmt.Fprintf(&b, "\n%s: %s", l.label, l.amount)
	}
	if accountURL != "" {
		fmt.Fprintf(&b, "\n\nView or download your invoice in your account: %s", accountURL)
	}
	return b.String()
}

func emailMoney(cents int64) string { return fmt.Sprintf("$%d.%02d", cents/100, cents%100) }

var winAnsi = encoding.ReplaceUnsupported(charmap.Windows1252.NewEncoder())

// pdfString puts text in the fonts' WinAnsi encoding (characters outside it become "?") and escapes what PDF strings reserve.
func pdfString(s string) string {
	enc, err := winAnsi.String(s)
	if err != nil {
		enc = "?"
	}
	return strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`, "\r", " ", "\n", " ").Replace(enc)
}
