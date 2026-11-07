package postgres

import (
	"bytes"
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

// BookingInvoicePDF is the same invoice as a one-page PDF to attach to the email, with its file name. Nil when there is no issued invoice.
func (s *Store) BookingInvoicePDF(ctx context.Context, bookingID string) (string, []byte) {
	number, lines, ok := s.bookingInvoice(ctx, bookingID)
	if !ok {
		return "", nil
	}
	return "Invoice-" + number + ".pdf", invoicePDF(number, lines)
}

func emailMoney(cents int64) string { return fmt.Sprintf("$%d.%02d", cents/100, cents%100) }

// invoicePDF lays the invoice out on one Letter page in the built-in Helvetica fonts, so no font files or PDF library are needed.
// shortcut: one page holds about 40 lines; an invoice with more items would run off the bottom, so add page breaks if that ever happens.
func invoicePDF(number string, lines []invoiceLine) []byte {
	var c bytes.Buffer
	text := func(font string, size, x, y float64, s string) {
		fmt.Fprintf(&c, "BT /%s %.0f Tf %.0f %.0f Td (%s) Tj ET\n", font, size, x, y, pdfString(s))
	}
	text("F2", 20, 56, 730, "Dapper Detailing Pros")
	text("F1", 10, 56, 712, "(705) 977-6217")
	text("F2", 14, 56, 670, "Invoice "+number)
	fmt.Fprintf(&c, "0.8 G 56 655 m 556 655 l S 0 G\n")
	y := 632.0
	for i, l := range lines {
		font := "F1"
		if i >= len(lines)-3 { // Total, Paid and Balance due stand out
			font = "F2"
			if i == len(lines)-3 {
				fmt.Fprintf(&c, "0.8 G 56 %.0f m 556 %.0f l S 0 G\n", y+14, y+14)
			}
		}
		text(font, 11, 56, y, l.label)
		text(font, 11, 460, y, l.amount)
		y -= 20
	}
	text("F1", 9, 56, 60, "Thank you for choosing Dapper Detailing Pros.")

	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", c.Len(), c.String()),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}

var winAnsi = encoding.ReplaceUnsupported(charmap.Windows1252.NewEncoder())

// pdfString puts text in the fonts' WinAnsi encoding (characters outside it become "?") and escapes what PDF strings reserve.
func pdfString(s string) string {
	enc, err := winAnsi.String(s)
	if err != nil {
		enc = "?"
	}
	return strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`, "\r", " ", "\n", " ").Replace(enc)
}
