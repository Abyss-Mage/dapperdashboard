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
}

type invoiceDocument struct {
	number, status, date, coupon, taxLabel    string
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
	var issuerRaw, customerRaw []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text,organization_id::text,location_id::text,invoice_number,status,coalesce(issued_at,created_at),subtotal_cents,discount_cents,coalesce(coupon_code,''),tax_cents,total_cents,paid_cents,due_cents,coalesce(issuer_snapshot,'{}'::jsonb),coalesce(customer_snapshot,'{}'::jsonb) FROM invoices
 WHERE booking_id::text=$1 AND status IN ('issued','partially_paid','paid') ORDER BY created_at DESC LIMIT 1`, bookingID).Scan(&id, &org, &loc, &d.number, &d.status, &when, &d.subtotal, &d.discount, &d.coupon, &d.tax, &d.total, &d.paid, &d.due, &issuerRaw, &customerRaw)
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
	state, err := loadEffectiveInvoiceTemplate(ctx, s.pool, org, loc)
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
	zone, err := time.LoadLocation("America/Toronto") // the zone the rest of the booking email uses
	if err != nil {
		zone = time.UTC
	}
	d.date = when.In(zone).Format("January 2, 2006")
	if d.template.ShowLogo {
		d.logo = defaultInvoiceLogo
		if _, own, _, found, e := s.readLocationLogo(ctx, org, loc); e == nil && found {
			d.logo = own
		}
	}
	return d, true
}

func invoiceTaxLabel(tax, base int64, name string) string {
	if tax <= 0 || base <= 0 {
		return "Tax"
	}
	pct := strconv.FormatFloat(math.Round(float64(tax)/float64(base)*10000)/100, 'f', -1, 64) + "%"
	if name != "" {
		return "Tax (" + name + " " + pct + ")"
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

func invoiceStatusWord(status string) string {
	switch status {
	case "paid":
		return "Paid"
	case "partially_paid":
		return "Partially paid"
	}
	return "Issued"
}

// Advance widths of the built-in fonts for characters 32 to 126, in 1/1000 of the font size; needed to right-align amounts and wrap text.
var helveticaWidths = [95]int{278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, 556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556, 1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556, 333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584}
var helveticaBoldWidths = [95]int{278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278, 556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611, 975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556, 333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611, 611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584}

const pdfRegular, pdfBold = "F1", "F2"

type pdfColor [3]float64

var (
	pdfInk   = pdfColor{.043, .043, .047}
	pdfMute  = pdfColor{.36, .36, .4}
	pdfWhite = pdfColor{1, 1, 1}
	pdfLine  = pdfColor{.89, .89, .91}
	pdfZebra = pdfColor{.969, .969, .973}
	pdfSoft  = pdfColor{.72, .71, .69}
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

func (p *pdfCanvas) outline(c pdfColor, x, y, w, h float64) {
	fmt.Fprintf(p.page(), "%.3f %.3f %.3f RG 1.5 w %.2f %.2f %.2f %.2f re S\n", c[0], c[1], c[2], x, y, w, h)
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
	t := d.template
	p := &pdfCanvas{w: 612, h: 792}
	if t.PaperSize == "a4" {
		p.w, p.h = 595, 842
	}
	p.newPage()
	const m = 40.0
	width := p.w - 2*m
	accent := pdfAccent(t.AccentColor)

	// The black band across the top: logo and business name at the left, the title and its details at the right.
	top := p.h
	p.rect(pdfInk, 0, top-104, p.w, 104)
	p.rect(accent, 0, top-110, p.w, 6)
	nameX := m
	logoW, logoH, logo, hasLogo := 0, 0, []byte(nil), false
	if d.logo != nil {
		logoW, logoH, logo, hasLogo = pdfLogoImage(d.logo)
	}
	if hasLogo {
		p.rect(pdfWhite, m, top-88, 72, 72)
		scale := 62 / float64(max(logoW, logoH))
		drawW, drawH := float64(logoW)*scale, float64(logoH)*scale
		fmt.Fprintf(p.page(), "q %.2f 0 0 %.2f %.2f %.2f cm /Im1 Do Q\n", drawW, drawH, m+36-drawW/2, top-52-drawH/2)
		nameX = m + 86
	}
	nameLines := pdfWrap(pdfBold, 14, 230, d.issuer.LegalName)
	if len(nameLines) > 2 {
		nameLines = nameLines[:2]
	}
	nameY := top - 52 - 5 + float64(len(nameLines)-1)*9
	for _, line := range nameLines {
		p.text(pdfBold, 14, nameX, nameY, pdfWhite, line)
		nameY -= 18
	}
	right := p.w - m
	p.right(pdfBold, 25, right, top-44, pdfWhite, strings.ToUpper(t.Title))
	p.right(pdfBold, 10, right, top-63, pdfWhite, "No. "+d.number)
	p.right(pdfRegular, 9.5, right, top-77, pdfSoft, "Issued "+d.date)
	p.right(pdfRegular, 9.5, right, top-91, pdfSoft, invoiceStatusWord(d.status))
	p.y = top - 136

	// Who it is for and who it is from.
	columnW := width/2 - 12
	party := func(x float64, heading, name string, rest []string) float64 {
		y := p.y
		p.text(pdfBold, 8.5, x, y, pdfMute, strings.ToUpper(heading))
		y -= 15
		for _, line := range pdfWrap(pdfBold, 11, columnW, name) {
			p.text(pdfBold, 11, x, y, pdfInk, line)
			y -= 14
		}
		for _, r := range rest {
			for _, line := range pdfWrap(pdfRegular, 10, columnW, r) {
				p.text(pdfRegular, 10, x, y, pdfInk, line)
				y -= 13
			}
		}
		return y
	}
	var contact []string
	if t.ShowCustomerContact {
		for _, v := range []string{d.customer.Email, d.customer.Phone} {
			if v != "" {
				contact = append(contact, v)
			}
		}
	}
	var from []string
	if d.issuer.Address != "" {
		from = append(from, d.issuer.Address)
	}
	if t.ShowTaxRegistration && d.issuer.TaxRegistration != "" {
		from = append(from, "GST/HST: "+d.issuer.TaxRegistration)
	}
	leftEnd, rightEnd := party(m, t.BillToLabel, d.customer.Name, contact), party(m+width/2+12, "From", d.issuer.LegalName, from)
	p.y = min(leftEnd, rightEnd) - 6
	p.rect(pdfLine, m, p.y, width, .75)
	p.y -= 14

	// What is owed, large enough to see first.
	paidInFull := d.total > 0 && d.due <= 0
	label, note, amount := "BALANCE DUE", "Nothing has been paid yet.", d.due
	if paidInFull {
		label, note, amount = "PAID IN FULL", "Thank you. Nothing further is owed.", d.paid
	} else if d.paid > 0 {
		note = invoiceMoney(d.paid) + " received so far."
	}
	boxBottom := p.y - 50
	if paidInFull {
		p.rect(pdfZebra, m, boxBottom, width, 50)
		p.outline(pdfInk, m, boxBottom, width, 50)
	} else {
		p.outline(accent, m, boxBottom, width, 50)
	}
	p.text(pdfBold, 10, m+14, boxBottom+29, pdfInk, label)
	p.text(pdfRegular, 9, m+14, boxBottom+14, pdfMute, note)
	p.right(pdfBold, 21, right-14, boxBottom+17, pdfInk, invoiceMoney(amount))
	p.y = boxBottom - 20

	if t.HeaderNote != "" {
		p.block(m, width, "", 10, pdfInk, t.HeaderNote)
		p.y -= 8
	}

	// The items.
	qtyRight, unitRight, amountRight := right-190, right-100, right-8
	tableHead := func() {
		p.rect(pdfInk, m, p.y-22, width, 22)
		y := p.y - 15
		p.text(pdfBold, 8.5, m+8, y, pdfWhite, strings.ToUpper(t.ItemLabel))
		p.right(pdfBold, 8.5, qtyRight, y, pdfWhite, strings.ToUpper(t.QuantityLabel))
		p.right(pdfBold, 8.5, unitRight, y, pdfWhite, strings.ToUpper(t.UnitPriceLabel))
		p.right(pdfBold, 8.5, amountRight, y, pdfWhite, strings.ToUpper(t.AmountLabel))
		p.y -= 22
	}
	p.room(70)
	tableHead()
	for i, it := range d.items {
		lines := pdfWrap(pdfRegular, 10.5, width-250, it.description)
		rowH := float64(len(lines))*13.5 + 12
		if p.room(rowH) {
			tableHead()
		}
		if i%2 == 1 {
			p.rect(pdfZebra, m, p.y-rowH, width, rowH)
		}
		y := p.y - 17
		p.right(pdfRegular, 10.5, qtyRight, y, pdfInk, strconv.Itoa(it.quantity))
		p.right(pdfRegular, 10.5, unitRight, y, pdfInk, invoiceMoney(it.unitPriceCents))
		p.right(pdfRegular, 10.5, amountRight, y, pdfInk, invoiceMoney(it.lineCents))
		for _, line := range lines {
			p.text(pdfRegular, 10.5, m+8, y, pdfInk, line)
			y -= 13.5
		}
		p.y -= rowH
		p.rect(pdfLine, m, p.y, width, .75)
	}
	p.y -= 12

	// The totals, kept together at the right.
	boxLeft := right - 250
	p.room(130)
	row := func(font, name, value string) {
		p.text(font, 10.5, boxLeft+8, p.y-12, pdfInk, name)
		p.right(font, 10.5, amountRight, p.y-12, pdfInk, value)
		p.y -= 18
	}
	row(pdfRegular, "Subtotal", invoiceMoney(d.subtotal))
	if d.discount > 0 {
		name := "Discount"
		if d.coupon != "" {
			name += " (" + d.coupon + ")"
		}
		row(pdfRegular, name, "-"+invoiceMoney(d.discount))
	}
	row(pdfRegular, d.taxLabel, invoiceMoney(d.tax))
	p.rect(pdfInk, boxLeft, p.y-2, 250, .9)
	p.y -= 4
	row(pdfBold, "Total", invoiceMoney(d.total))
	if d.paid > 0 {
		row(pdfRegular, "Paid", "-"+invoiceMoney(d.paid))
	}
	if !paidInFull {
		p.y -= 4
		p.rect(pdfInk, boxLeft, p.y-28, 250, 28)
		p.text(pdfBold, 12, boxLeft+8, p.y-19, pdfWhite, "Balance due")
		p.right(pdfBold, 12, amountRight, p.y-19, pdfWhite, invoiceMoney(d.due))
		p.y -= 28
	}
	p.y -= 24

	if t.PaymentInstructions != "" {
		p.block(m, width, "Payment", 10, pdfInk, t.PaymentInstructions)
		p.y -= 10
	}
	owner := ""
	if d.franchise && d.issuer.LegalName != "" {
		owner = "Independently owned and operated by " + strings.TrimRight(d.issuer.LegalName, ".") + "."
	}
	if t.FooterNote != "" || owner != "" {
		p.room(30)
		p.rect(pdfLine, m, p.y+6, width, .75)
		p.y -= 8
		if t.FooterNote != "" {
			p.block(m, width, "", 9.5, pdfMute, t.FooterNote)
		}
		if owner != "" {
			p.block(m, width, "", 9.5, pdfMute, owner)
		}
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
