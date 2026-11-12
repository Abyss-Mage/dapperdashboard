package postgres

import (
	"bytes"
	"compress/zlib"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// The emailed invoice: the built-in printed layout (InvoicePrint.tsx) drawn as a PDF, using the same template, so colour, wording, logo and
// business details match. A template's custom HTML layout cannot be drawn here; those invoices get this layout with the template's wording.

//go:embed invoice_logo.webp
var defaultInvoiceLogo []byte

type invoiceDocumentItem struct {
	description               string
	quantity                  int
	unitPriceCents, lineCents int64
	vin, stock, detail        string
}

type invoiceDocument struct {
	number, status, date, coupon, taxLabel    string
	serviceDate, serviceTime, payment         string
	quote                                     bool
	validUntil, note                          string
	dueDate                                   string
	overdue                                   bool
	b2b                                       bool   // a dealership invoice: a table of item, description, qty, rate, amount
	po, discountLabel, clientAddress          string
	issuer                                    QuoteIssuer
	customer                                  InvoiceCustomerSnapshot
	items                                     []invoiceDocumentItem
	subtotal, discount, tax, total, paid, due int64
	template                                  InvoicePrintTemplate
	franchise                                 bool
	logo                                      []byte
}

// BookingInvoicePDF is the booking's latest issued invoice as a PDF to attach to the email, with its file name. Nil when there is none.
func (s *Store) BookingInvoicePDF(ctx context.Context, bookingID string) (string, []byte) {
	d, ok := s.bookingInvoiceDocument(ctx, bookingID)
	if !ok {
		return "", nil
	}
	return "Invoice-" + d.number + ".pdf", invoicePDF(d)
}

func (s *Store) bookingInvoiceDocument(ctx context.Context, bookingID string) (invoiceDocument, bool) {
	var d invoiceDocument
	var id, org, loc string
	var when time.Time
	var serviceAt *time.Time
	var methods string
	var issuerRaw, customerRaw []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text,organization_id::text,location_id::text,invoice_number,status,coalesce(issued_at,created_at),subtotal_cents,discount_cents,coalesce(coupon_code,''),tax_cents,total_cents,paid_cents,due_cents,coalesce(issuer_snapshot,'{}'::jsonb),coalesce(customer_snapshot,'{}'::jsonb),
 (SELECT b.booking_time FROM bookings b WHERE b.id=invoices.booking_id),coalesce((SELECT string_agg(DISTINCT p.method,',') FROM payments p WHERE p.invoice_id=invoices.id AND p.status='completed'),'') FROM invoices
 WHERE booking_id::text=$1 AND status IN ('issued','partially_paid','paid') ORDER BY created_at DESC LIMIT 1`, bookingID).Scan(&id, &org, &loc, &d.number, &d.status, &when, &d.subtotal, &d.discount, &d.coupon, &d.tax, &d.total, &d.paid, &d.due, &issuerRaw, &customerRaw, &serviceAt, &methods)
	if err != nil {
		return d, false
	}
	rows, err := s.pool.Query(ctx, `SELECT description,quantity,unit_price_cents,total_price_cents FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return d, false
	}
	defer rows.Close()
	for rows.Next() {
		var it invoiceDocumentItem
		if rows.Scan(&it.description, &it.quantity, &it.unitPriceCents, &it.lineCents) != nil {
			return d, false
		}
		d.items = append(d.items, it)
	}
	if rows.Err() != nil {
		return d, false
	}
	// The issued snapshot is the legal record; the rest is presentation, so a failure below still sends the invoice in the default look.
	_ = json.Unmarshal(issuerRaw, &d.issuer)
	_ = json.Unmarshal(customerRaw, &d.customer)
	d.date = documentDate(when)
	d.serviceDate, d.serviceTime = documentService(serviceAt)
	d.payment = invoicePaymentWords(methods)
	s.dressDocument(ctx, &d, org, loc, invoicePrint)
	return d, true
}

// InvoicePDF draws an invoice the caller has already been allowed to read, with its file name.
func (s *Store) InvoicePDF(ctx context.Context, inv Invoice) (string, []byte) {
	d := invoiceDocument{number: inv.InvoiceNumber, status: inv.Status, coupon: inv.CouponCode, subtotal: inv.SubtotalCents, discount: inv.DiscountCents, tax: inv.TaxCents, total: inv.TotalCents, paid: inv.PaidCents, due: inv.DueCents,
		customer: InvoiceCustomerSnapshot{Name: inv.CustomerName, Email: inv.CustomerEmail}, payment: invoicePaymentWords(inv.PaymentMethods)}
	if inv.IssuerSnapshot != nil {
		d.issuer = QuoteIssuer{LegalName: inv.IssuerSnapshot.LegalName, Address: inv.IssuerSnapshot.Address, TaxRegistration: inv.IssuerSnapshot.TaxRegistration}
	}
	if inv.CustomerSnapshot != nil {
		d.customer = *inv.CustomerSnapshot
	}
	for _, it := range inv.Items {
		d.items = append(d.items, invoiceDocumentItem{description: it.Description, quantity: it.Quantity, unitPriceCents: it.UnitPriceCents, lineCents: it.TotalPriceCents})
	}
	when := inv.CreatedAt
	if inv.IssuedAt != nil {
		when = *inv.IssuedAt
	}
	d.date = documentDate(when)
	d.serviceDate, d.serviceTime = documentService(inv.ServiceAt)
	kind := invoicePrint
	// A dealership invoice prints with the dealership template, its due date and each car's VIN / stock number (presentation only: on a
	// failed lookup it prints as a plain invoice).
	var dealer bool
	var invoiceDate, dueDate *time.Time
	var pct *float64
	if s.pool.QueryRow(ctx, `SELECT dealership_account_id IS NOT NULL,invoice_date,due_date,po_number,discount_percent::float8,coalesce((SELECT d.address FROM dealership_accounts d WHERE d.id=invoices.dealership_account_id),'') FROM invoices WHERE id=$1`, inv.ID).Scan(&dealer, &invoiceDate, &dueDate, &d.po, &pct, &d.clientAddress) == nil && dealer {
		kind, d.b2b = dealerPrint, true
		switch {
		case dueDate != nil:
			d.dueDate = dueDate.UTC().Format("02/Jan/2006")
		case inv.DueAt != nil:
			d.dueDate = documentDateShort(*inv.DueAt)
		}
		d.date = documentDateShort(when)
		if invoiceDate != nil {
			d.date = invoiceDate.UTC().Format("02/Jan/2006")
		}
		d.note = inv.Notes
		if pct != nil {
			d.discountLabel = "Discount (" + strconv.FormatFloat(*pct, 'f', -1, 64) + "%)"
		}
		if inv.Status == "draft" {
			d.number = "Draft"
		}
		d.overdue = overdue(inv.Status, inv.DueAt)
		if rows, err := s.pool.Query(ctx, `SELECT vin,stock_number,detail FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, inv.ID); err == nil {
			for i := 0; rows.Next() && i < len(d.items); i++ {
				_ = rows.Scan(&d.items[i].vin, &d.items[i].stock, &d.items[i].detail)
			}
			rows.Close()
		}
	}
	s.dressDocument(ctx, &d, inv.OrganizationID, inv.LocationID, kind)
	return documentFileName("Invoice", d.number), invoicePDF(d)
}

// QuotePDF draws a quotation the caller has already been allowed to read, with its file name.
func (s *Store) QuotePDF(ctx context.Context, q Quote) (string, []byte) {
	d := invoiceDocument{quote: true, number: q.ReferenceNumber, status: q.Status, subtotal: q.SubtotalCents, tax: q.TaxCents, total: q.TotalCents, due: q.TotalCents, note: q.CustomerNote,
		customer: InvoiceCustomerSnapshot{Name: q.CustomerName}}
	// A quotation stores only who it is for, so the contact details come from the customer's record. Presentation only: on failure they are left off.
	_ = s.pool.QueryRow(ctx, `SELECT coalesce(email,''),coalesce(phone,'') FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3`, q.OrganizationID, q.LocationID, q.CustomerID).Scan(&d.customer.Email, &d.customer.Phone)
	for _, it := range q.Items {
		d.items = append(d.items, invoiceDocumentItem{description: it.Description, quantity: it.Quantity, unitPriceCents: it.UnitPriceCents, lineCents: it.LineTotalCents})
	}
	when := q.CreatedAt
	if q.IssuedAt != nil {
		when = *q.IssuedAt
	}
	d.date = documentDate(when)
	if q.ExpiresAt != nil {
		d.validUntil = documentDate(*q.ExpiresAt)
	}
	s.dressDocument(ctx, &d, q.OrganizationID, q.LocationID, quotePrint)
	return documentFileName("Quotation", d.number), invoicePDF(d)
}

// QuoteEmailFacts is where an issued quotation's email goes (the address on the customer's record, empty when there is none) and its wording's values.
func (s *Store) QuoteEmailFacts(ctx context.Context, q Quote) (string, map[string]string, error) {
	var to string
	if err := s.pool.QueryRow(ctx, `SELECT btrim(coalesce(email,'')) FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3`, q.OrganizationID, q.LocationID, q.CustomerID).Scan(&to); err != nil {
		return "", nil, err
	}
	vars := map[string]string{"name": q.CustomerName, "reference": q.ReferenceNumber, "total": invoiceMoney(q.TotalCents), "valid_until": ""}
	if q.ExpiresAt != nil {
		vars["valid_until"] = "Valid until: " + documentDate(*q.ExpiresAt)
	}
	return to, vars, nil
}

// dressDocument adds the franchise's look and details. It is presentation only, so a failure here still gives a document in the default look.
func (s *Store) dressDocument(ctx context.Context, d *invoiceDocument, org, loc string, kind printKind) {
	state, err := kind.effective(ctx, s.pool, org, loc)
	if err != nil {
		state = InvoicePrintTemplateState{Template: DefaultInvoicePrintTemplate()}
	}
	_ = printParties(ctx, s.pool, org, loc, &state)
	if d.issuer.LegalName == "" && state.Issuer != nil {
		d.issuer = *state.Issuer
	}
	d.template = state.Template
	outlet, _ := outletOf(ctx, s.pool, org, loc)
	d.franchise = outlet == "franchise"
	d.taxLabel = invoiceTaxLabel(d.tax, d.subtotal-d.discount, state.TaxName)
	if d.template.ShowLogo {
		d.logo = defaultInvoiceLogo
		if _, own, _, found, e := s.readLocationLogo(ctx, org, loc); e == nil && found {
			d.logo = own
		}
	}
}

func documentZone() *time.Location {
	zone, err := time.LoadLocation("America/Toronto") // the zone the rest of the booking email uses
	if err != nil {
		return time.UTC
	}
	return zone
}

func documentDate(t time.Time) string { return t.In(documentZone()).Format("January 2, 2006") }

// documentDateShort is the dealership invoice's date style (21/Nov/2025), as in the owner's template.
func documentDateShort(t time.Time) string { return t.In(documentZone()).Format("02/Jan/2006") }

func documentService(at *time.Time) (date, clock string) {
	if at == nil {
		return "", ""
	}
	return documentDate(*at), at.In(documentZone()).Format("3:04 PM")
}

var unsafeFileName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func documentFileName(kind, number string) string {
	return kind + "-" + unsafeFileName.ReplaceAllString(number, "-") + ".pdf"
}

func invoiceTaxLabel(tax, base int64, name string) string {
	if tax <= 0 || base <= 0 {
		return "Tax"
	}
	pct := strconv.FormatFloat(math.Round(float64(tax)/float64(base)*10000)/100, 'f', -1, 64) + "%"
	if name != "" {
		return name + " (" + pct + ")"
	}
	return "Tax (" + pct + ")"
}

func invoiceMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	return fmt.Sprintf("%s$%s.%02d", sign, whole, cents%100)
}

// invoicePaymentWords turns the stored payment methods ("cash,square_card") into what the customer reads.
func invoicePaymentWords(methods string) string {
	words := map[string]string{"cash": "Cash", "square_card": "Card", "square_terminal": "Card", "bank_transfer": "Bank transfer", "cheque": "Cheque"}
	var out []string
	for _, m := range strings.Split(methods, ",") {
		if w := words[m]; w != "" && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return strings.Join(out, ", ")
}

// Advance widths of the built-in fonts for characters 32 to 126, in 1/1000 of the font size; needed to right-align amounts and wrap text.
var helveticaWidths = [95]int{278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, 556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556, 1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556, 333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584}
var helveticaBoldWidths = [95]int{278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278, 556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611, 975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556, 333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611, 611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584}

const pdfRegular, pdfBold = "F1", "F2"

type pdfColor [3]float64

var (
	pdfInk   = pdfColor{0, 0, 0}
	pdfMute  = pdfColor{.4, .4, .4}
	pdfLine  = pdfColor{.898, .898, .898}
	pdfPanel = pdfColor{.98, .98, .98}
	pdfLabel = pdfColor{.533, .533, .533}
)

func pdfAccent(hex string) pdfColor {
	if !templateColor.MatchString(hex) {
		return pdfInk
	}
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	return pdfColor{float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255}
}

type pdfCanvas struct {
	w, h, y float64
	pages   []*bytes.Buffer
}

func (p *pdfCanvas) page() *bytes.Buffer { return p.pages[len(p.pages)-1] }
func (p *pdfCanvas) newPage() {
	p.pages = append(p.pages, &bytes.Buffer{})
	p.y = p.h - 48
}

// room starts a new page when the next h points would not fit; it reports whether it did.
func (p *pdfCanvas) room(h float64) bool {
	if p.y-h >= 48 {
		return false
	}
	p.newPage()
	return true
}

func (p *pdfCanvas) rect(c pdfColor, x, y, w, h float64) {
	fmt.Fprintf(p.page(), "%.3f %.3f %.3f rg %.2f %.2f %.2f %.2f re f\n", c[0], c[1], c[2], x, y, w, h)
}

// box fills a rectangle and draws a thin border round it.
func (p *pdfCanvas) box(fill, stroke pdfColor, x, y, w, h float64) {
	fmt.Fprintf(p.page(), "%.3f %.3f %.3f rg %.3f %.3f %.3f RG .75 w %.2f %.2f %.2f %.2f re B\n", fill[0], fill[1], fill[2], stroke[0], stroke[1], stroke[2], x, y, w, h)
}

func (p *pdfCanvas) text(font string, size, x, y float64, c pdfColor, s string) {
	fmt.Fprintf(p.page(), "BT %.3f %.3f %.3f rg /%s %.1f Tf %.2f %.2f Td (%s) Tj ET\n", c[0], c[1], c[2], font, size, x, y, pdfString(s))
}

func (p *pdfCanvas) right(font string, size, x, y float64, c pdfColor, s string) {
	p.text(font, size, x-pdfWidth(font, size, s), y, c, s)
}

func pdfWidth(font string, size float64, s string) float64 {
	widths := &helveticaWidths
	if font == pdfBold {
		widths = &helveticaBoldWidths
	}
	total := 0
	for _, r := range s {
		if r >= 32 && r <= 126 {
			total += widths[r-32]
		} else {
			total += 556
		}
	}
	return float64(total) * size / 1000
}

// pdfWrap breaks text into lines no wider than width, keeping the line breaks the owner typed. A word wider than a line is cut.
func pdfWrap(font string, size, width float64, s string) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			for pdfWidth(font, size, word) > width {
				runes, n := []rune(word), 1
				for n < len(runes)-1 && pdfWidth(font, size, string(runes[:n+1])) <= width {
					n++
				}
				if line != "" {
					out, line = append(out, line), ""
				}
				out, word = append(out, string(runes[:n])), string(runes[n:])
			}
			if try := strings.TrimSpace(line + " " + word); pdfWidth(font, size, try) <= width {
				line = try
			} else {
				out, line = append(out, line), word
			}
		}
		out = append(out, line)
	}
	return out
}

// block writes a small heading (optional) and wrapped text at the full width, moving to a new page between lines when needed.
func (p *pdfCanvas) block(x, width float64, heading string, size float64, c pdfColor, s string) {
	p.room(30)
	if heading != "" {
		p.text(pdfBold, 8.5, x, p.y, pdfMute, strings.ToUpper(heading))
		p.y -= 14
	}
	for _, line := range pdfWrap(pdfRegular, size, width, s) {
		p.room(size + 3)
		p.text(pdfRegular, size, x, p.y, c, line)
		p.y -= size + 3.5
	}
}

// pdfLogoImage turns a PNG, JPEG or WebP logo into white-backed RGB samples, at most about 256 pixels across. ok is false for anything unusable.
func pdfLogoImage(data []byte) (w, h int, rgb []byte, ok bool) {
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width == 0 || cfg.Height == 0 {
		return 0, 0, nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, false
	}
	b := img.Bounds()
	step := max(b.Dx(), b.Dy())/256 + 1
	w, h = b.Dx()/step, b.Dy()/step
	if w == 0 || h == 0 {
		return 0, 0, nil, false
	}
	rgb = make([]byte, 0, w*h*3)
	for y := range h {
		for x := range w {
			var r, g, bl uint64
			for dy := range step {
				for dx := range step {
					cr, cg, cb, ca := img.At(b.Min.X+x*step+dx, b.Min.Y+y*step+dy).RGBA()
					back := uint64(0xffff - ca) // the colour values are premultiplied, so adding this lays them over white
					r, g, bl = r+uint64(cr)+back, g+uint64(cg)+back, bl+uint64(cb)+back
				}
			}
			n := uint64(step*step) * 257
			rgb = append(rgb, byte(r/n), byte(g/n), byte(bl/n))
		}
	}
	return w, h, rgb, true
}

func invoicePDF(d invoiceDocument) []byte {
	if d.b2b {
		return dealerInvoicePDF(d)
	}
	t := d.template
	p := &pdfCanvas{w: 612, h: 792}
	if t.PaperSize == "a4" {
		p.w, p.h = 595, 842
	}
	p.newPage()
	const m = 40.0
	width := p.w - 2*m
	accent := pdfAccent(t.AccentColor)

	// The heading: logo, business name and details at the left; the title, number and date at the right.
	top, right := p.h, p.w-m
	textX, logoBottom := m, top
	logoW, logoH, logo, hasLogo := 0, 0, []byte(nil), false
	if d.logo != nil {
		logoW, logoH, logo, hasLogo = pdfLogoImage(d.logo)
	}
	if hasLogo {
		scale := 72 / float64(max(logoW, logoH))
		drawW, drawH := float64(logoW)*scale, float64(logoH)*scale
		fmt.Fprintf(p.page(), "q %.2f 0 0 %.2f %.2f %.2f cm /Im1 Do Q\n", drawW, drawH, m, top-36-drawH)
		textX, logoBottom = m+drawW+16, top-36-drawH
	}
	y := top - 50
	for _, line := range pdfWrap(pdfBold, 15, 270, d.issuer.LegalName) {
		p.text(pdfBold, 15, textX, y, pdfInk, line)
		y -= 18
	}
	y -= 2
	details := d.issuer.Address + "\n" + t.BusinessDetails
	if t.ShowTaxRegistration && d.issuer.TaxRegistration != "" {
		details += "\nHST: " + d.issuer.TaxRegistration
	}
	for _, line := range pdfWrap(pdfRegular, 9.5, 270, details) {
		if line != "" {
			p.text(pdfRegular, 9.5, textX, y, pdfInk, line)
			y -= 14
		}
	}
	p.right(pdfBold, 30, right, top-62, accent, strings.ToUpper(t.Title))
	metaY := top - 84
	meta := [][2]string{{"INVOICE #", d.number}, {"DATE", d.date}}
	if d.quote {
		meta[0][0] = "QUOTE #"
	}
	if d.validUntil != "" {
		meta = append(meta, [2]string{"VALID UNTIL", d.validUntil})
	}
	if d.dueDate != "" {
		meta = append(meta, [2]string{"DUE DATE", d.dueDate})
	}
	if d.po != "" {
		meta = append(meta, [2]string{"PO #", d.po})
	}
	if !d.quote && d.status == "draft" {
		meta = append(meta, [2]string{"STATUS", "Draft"})
	} else if d.overdue {
		meta = append(meta, [2]string{"STATUS", "Overdue"})
	}
	for _, row := range meta {
		p.right(pdfRegular, 9.5, right, metaY, pdfInk, row[1])
		p.right(pdfRegular, 9, right-pdfWidth(pdfRegular, 9.5, row[1])-8, metaY, pdfLabel, row[0])
		metaY -= 15
	}
	p.y = min(y, logoBottom, metaY) - 10
	p.rect(pdfLine, 0, p.y, p.w, .75)

	// Who it is for, and when the work was done.
	var client []string
	if d.customer.Name != "" {
		client = append(client, "Name: "+d.customer.Name)
	}
	if t.ShowCustomerContact && d.customer.Email != "" {
		client = append(client, "Email: "+d.customer.Email)
	}
	if t.ShowCustomerContact && d.customer.Phone != "" {
		client = append(client, "Phone Number: "+d.customer.Phone)
	}
	var clientLines []string
	for _, c := range client {
		clientLines = append(clientLines, pdfWrap(pdfRegular, 9.5, width/2-12, c)...)
	}
	var facts [][2]string
	for _, f := range [][2]string{{"Service Date:", d.serviceDate}, {"Time:", d.serviceTime}, {"Payment:", d.payment}, {"Coupon:", d.coupon}} {
		if f[1] != "" {
			facts = append(facts, f)
		}
	}
	bandH := float64(max(len(clientLines)*14+16, len(facts)*16)) + 44
	p.rect(pdfPanel, 0, p.y-bandH, p.w, bandH)
	cy := p.y - 28
	p.text(pdfRegular, 9, m, cy, pdfLabel, strings.ToUpper(t.BillToLabel))
	cy -= 18
	for _, line := range clientLines {
		p.text(pdfRegular, 9.5, m, cy, pdfInk, line)
		cy -= 14
	}
	fy := p.y - 28
	for _, f := range facts {
		p.right(pdfRegular, 9.5, right, fy, pdfInk, f[1])
		p.right(pdfRegular, 9.5, right-max(pdfWidth(pdfRegular, 9.5, f[1]), 96)-14, fy, pdfLabel, f[0])
		fy -= 16
	}
	p.y -= bandH + 28

	if t.HeaderNote != "" {
		p.block(m, width, "", 10, pdfInk, t.HeaderNote)
		p.y -= 10
	}

	serviceBreakdown(p, d, m, width, right)

	// The totals, kept together at the right.
	const totalsW = 250.0
	boxLeft, boxH := right-totalsW, 120.0
	if d.b2b && d.discount == 0 {
		boxH = 98
	}
	p.room(boxH)
	p.box(pdfPanel, pdfLine, boxLeft, p.y-boxH, totalsW, boxH)
	ry := p.y - 26
	row := func(name, value string) {
		p.text(pdfRegular, 10, boxLeft+18, ry, pdfMute, name)
		p.right(pdfRegular, 10, right-18, ry, pdfInk, value)
		ry -= 22
	}
	row("Subtotal", invoiceMoney(d.subtotal))
	if !d.b2b || d.discount > 0 {
		label := "Discount"
		if d.discountLabel != "" {
			label = d.discountLabel
		}
		row(label, "- "+invoiceMoney(d.discount))
	}
	row(d.taxLabel, invoiceMoney(d.tax))
	p.rect(pdfLine, boxLeft+18, ry+13, totalsW-36, .75)
	ry -= 8
	totalLabel := "TOTAL DUE"
	if d.quote {
		totalLabel = "QUOTED TOTAL"
	}
	p.text(pdfBold, 13, boxLeft+18, ry, pdfInk, totalLabel)
	p.right(pdfBold, 13, right-18, ry, pdfInk, invoiceMoney(d.total))
	p.y -= boxH + 20
	return finishDocument(p, d, m, width, logoW, logoH, logo)
}

// serviceBreakdown is the customer invoice's and quotation's list: one box per service with its price.
func serviceBreakdown(p *pdfCanvas, d invoiceDocument, m, width, right float64) {
	p.room(70)
	p.text(pdfBold, 9, m, p.y, pdfLabel, "SERVICE BREAKDOWN")
	p.y -= 12
	for _, it := range d.items {
		name := it.description
		if it.quantity > 1 {
			name += " × " + strconv.Itoa(it.quantity)
		}
		var car []string
		if it.vin != "" {
			car = append(car, "VIN: "+it.vin)
		}
		if it.stock != "" {
			car = append(car, "Stock: "+it.stock)
		}
		if len(car) > 0 {
			name += "\n" + strings.Join(car, "   ")
		}
		lines := pdfWrap(pdfRegular, 10, width-130, name)
		rowH := float64(len(lines))*13.5 + 22
		p.room(rowH)
		p.box(pdfPanel, pdfLine, m, p.y-rowH, width, rowH)
		ty := p.y - 19
		p.right(pdfRegular, 10, right-14, ty, pdfInk, invoiceMoney(it.lineCents))
		for _, line := range lines {
			p.text(pdfRegular, 10, m+14, ty, pdfInk, line)
			ty -= 13.5
		}
		p.y -= rowH
	}
	p.y -= 20
}

// finishDocument draws what follows the totals: paid and owing, notes, and the footer.
func finishDocument(p *pdfCanvas, d invoiceDocument, m, width float64, logoW, logoH int, logo []byte) []byte {
	t := d.template
	// What has been paid and what is still owed. A quotation has neither; a voided invoice says so instead.
	lightRed, redEdge := pdfColor{.996, .949, .949}, pdfColor{.988, .647, .647}
	halfW := width/2 - 8
	status := func(x float64, fill, stroke, labelC, amountC pdfColor, label string, cents int64) {
		p.box(fill, stroke, x, p.y-58, halfW, 58)
		p.text(pdfBold, 8, x+halfW/2-pdfWidth(pdfBold, 8, label)/2, p.y-22, labelC, label)
		amount := invoiceMoney(cents)
		p.text(pdfBold, 17, x+halfW/2-pdfWidth(pdfBold, 17, amount)/2, p.y-44, amountC, amount)
	}
	switch {
	case d.quote:
	case d.status == "void":
		const voided = "This invoice was voided, so nothing is owed on it."
		p.room(44)
		p.box(lightRed, redEdge, m, p.y-40, width, 40)
		p.text(pdfBold, 10, p.w/2-pdfWidth(pdfBold, 10, voided)/2, p.y-24, pdfInk, voided)
		p.y -= 40 + 20
	default:
		p.room(62)
		status(m, pdfColor{.863, .988, .906}, pdfColor{.525, .937, .675}, pdfColor{.086, .639, .29}, pdfColor{.086, .396, .204}, "PAID AMOUNT", d.paid)
		status(m+width/2+8, lightRed, redEdge, pdfColor{.863, .149, .149}, pdfColor{.6, .106, .106}, "OUTSTANDING BALANCE", d.due)
		p.y -= 58 + 20
	}

	if d.quote && d.note != "" {
		p.block(m, width, "Note", 10, pdfInk, d.note)
		p.y -= 10
	}
	if d.quote && t.QuotationNote != "" {
		p.block(m, width, "Terms", 10, pdfInk, t.QuotationNote)
		p.y -= 10
	}
	if !d.quote && t.PaymentInstructions != "" {
		p.block(m, width, "Payment", 10, pdfInk, t.PaymentInstructions)
		p.y -= 10
	}
	centered := func(font string, size float64, c pdfColor, s string) {
		for _, line := range pdfWrap(font, size, width, s) {
			p.room(size + 4)
			p.text(font, size, p.w/2-pdfWidth(font, size, line)/2, p.y, c, line)
			p.y -= size + 5
		}
	}
	p.room(60)
	p.rect(pdfLine, 0, p.y, p.w, .75)
	p.y -= 22
	if t.FooterNote != "" {
		centered(pdfBold, 11, pdfInk, t.FooterNote)
		p.y -= 3
	}
	kindWord := "invoice"
	if d.quote {
		kindWord = "quotation"
	}
	centered(pdfRegular, 8.5, pdfLabel, "This is a computer generated "+kindWord+". Signature is not required.")
	if d.franchise && d.issuer.LegalName != "" {
		centered(pdfRegular, 8.5, pdfLabel, "Independently owned and operated by "+strings.TrimRight(d.issuer.LegalName, ".")+".")
	}
	return p.bytes(logoW, logoH, logo)
}

// bytes assembles the file: catalog, page list, the two built-in fonts, the logo when there is one, then each page and its content.
func (p *pdfCanvas) bytes(logoW, logoH int, logo []byte) []byte {
	objs := []string{"", "",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
	}
	resources := "<< /Font << /F1 3 0 R /F2 4 0 R >>"
	if logo != nil {
		var packed bytes.Buffer
		z := zlib.NewWriter(&packed)
		_, _ = z.Write(logo)
		_ = z.Close()
		objs = append(objs, fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", logoW, logoH, packed.Len(), packed.String()))
		resources += " /XObject << /Im1 5 0 R >>"
	}
	resources += " >>"
	kids := make([]string, 0, len(p.pages))
	for _, c := range p.pages {
		page := len(objs) + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", page))
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources %s /Contents %d 0 R >>", p.w, p.h, resources, page+1),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", c.Len(), c.String()))
	}
	objs[0] = "<< /Type /Catalog /Pages 2 0 R >>"
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(kids))

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
