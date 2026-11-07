package postgres

import (
	"context"
	"fmt"
	"strings"
)

// BookingInvoiceText writes the booking's invoice out for an email: its number, every line (the service and each add-on), the discount,
// tax, total, what is paid and what is still due, and where to open it. Empty when the booking has no issued invoice yet.
func (s *Store) BookingInvoiceText(ctx context.Context, bookingID, accountURL string) string {
	var id, number, coupon string
	var discount, tax, total, paid, due int64
	err := s.pool.QueryRow(ctx, `SELECT id::text,invoice_number,coalesce(coupon_code,''),discount_cents,tax_cents,total_cents,paid_cents,due_cents FROM invoices
 WHERE booking_id::text=$1 AND status IN ('issued','partially_paid','paid') ORDER BY created_at DESC LIMIT 1`, bookingID).Scan(&id, &number, &coupon, &discount, &tax, &total, &paid, &due)
	if err != nil {
		return ""
	}
	rows, err := s.pool.Query(ctx, `SELECT description,quantity,total_price_cents FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var b strings.Builder
	fmt.Fprintf(&b, "Invoice %s\n", number)
	first := true
	for rows.Next() {
		var desc string
		var qty int
		var cents int64
		if rows.Scan(&desc, &qty, &cents) != nil {
			return ""
		}
		label := desc
		if !first {
			label = "Add-on: " + desc
		}
		if qty > 1 {
			label = fmt.Sprintf("%s × %d", label, qty)
		}
		fmt.Fprintf(&b, "%s: %s\n", label, emailMoney(cents))
		first = false
	}
	if rows.Err() != nil {
		return ""
	}
	if discount > 0 {
		label := "Discount"
		if coupon != "" {
			label += " (" + coupon + ")"
		}
		fmt.Fprintf(&b, "%s: -%s\n", label, emailMoney(discount))
	}
	if tax > 0 {
		fmt.Fprintf(&b, "Tax: %s\n", emailMoney(tax))
	}
	fmt.Fprintf(&b, "Total: %s\nPaid: %s\nBalance due: %s", emailMoney(total), emailMoney(paid), emailMoney(due))
	if accountURL != "" {
		fmt.Fprintf(&b, "\n\nView or download your invoice in your account: %s", accountURL)
	}
	return b.String()
}

func emailMoney(cents int64) string { return fmt.Sprintf("$%d.%02d", cents/100, cents%100) }
