package postgres

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// dealerInvoicePDF is the dealership invoice in the layout the owner sent (error404 #22/#29, file of 2026-11-11): business name with a
// rule under it, the business details, the logo at the right; a row with invoice number, date, due date and PO; a Client Details box; a
// bordered table of Item No., Description, Qty and Line Total under a red header; a totals box (subtotal, discount, tax, grand total);
// and a red rule over the footer. Wording, colour and logo come from the dealership template like every other document.
func dealerInvoicePDF(d invoiceDocument) []byte {
	t := d.template
	p := &pdfCanvas{w: 612, h: 792}
	if t.PaperSize == "a4" {
		p.w, p.h = 595, 842
	}
	p.newPage()
	const m = 36.0
	width, right := p.w-2*m, p.w-m
	accent := pdfAccent(t.AccentColor)
	if accent == pdfInk {
		accent = pdfColor{.894, .102, .169} // the template's red when none is set
	}
	grid := pdfColor{.85, .85, .85}
	top := p.h - 36

	// The logo at the right, about 100 points across.
	logoW, logoH, logo, hasLogo := 0, 0, []byte(nil), false
	if d.logo != nil {
		logoW, logoH, logo, hasLogo = pdfLogoImage(d.logo)
	}
	logoBottom := top
	if hasLogo {
		scale := 100 / float64(max(logoW, logoH))
		w, h := float64(logoW)*scale, float64(logoH)*scale
		fmt.Fprintf(p.page(), "q %.2f 0 0 %.2f %.2f %.2f cm /Im1 Do Q\n", w, h, right-w, top-h)
		logoBottom = top - h
	}

	// The business name with a short rule under it, then its details.
	y := top - 18
	name := d.issuer.LegalName
	if name == "" {
		name = t.Title
	}
	for _, line := range pdfWrap(pdfBold, 18, width-130, name) {
		p.text(pdfBold, 18, m, y, pdfInk, line)
		y -= 21
	}
	p.rect(pdfInk, m, y+8, 130, 2)
	y -= 10
	details := strings.TrimSpace(t.BusinessDetails)
	if t.ShowTaxRegistration && d.issuer.TaxRegistration != "" && !strings.Contains(details, d.issuer.TaxRegistration) {
		details = strings.TrimSpace(details + "\nHST: " + d.issuer.TaxRegistration)
	}
	if d.issuer.Address != "" {
		details = strings.TrimSpace(details + "\n" + d.issuer.Address)
	}
	for _, line := range pdfWrap(pdfRegular, 9.5, width-130, details) {
		if line != "" {
			p.text(pdfRegular, 9.5, m, y, pdfInk, line)
			y -= 13
		}
	}

	// Invoice number, dates and PO across the page.
	p.y = min(y, logoBottom) - 26
	meta := [][2]string{{"Invoice #:", d.number}, {"Date:", d.date}, {"Due:", d.dueDate}, {"PO:", d.po}}
	col := width / 4
	for i, f := range meta {
		x := m + float64(i)*col
		p.text(pdfBold, 10, x, p.y, pdfInk, f[0])
		p.text(pdfRegular, 10, x+pdfWidth(pdfBold, 10, f[0])+4, p.y, pdfInk, f[1])
	}
	switch {
	case d.status == "void":
		p.y -= 16
		p.text(pdfBold, 10, m, p.y, accent, "VOID: nothing is owed on this invoice.")
	case d.status == "draft":
		p.y -= 16
		p.text(pdfBold, 10, m, p.y, accent, "DRAFT")
	case d.overdue:
		p.y -= 16
		p.text(pdfBold, 10, m, p.y, accent, "OVERDUE")
	}
	p.y -= 18

	// Client Details.
	var client []string
	people := []string{d.customer.Name, d.clientAddress}
	if t.ShowCustomerContact {
		people = []string{d.customer.Name, d.customer.Phone, d.customer.Email, d.clientAddress}
	}
	for _, v := range people {
		if strings.TrimSpace(v) != "" {
			client = append(client, pdfWrap(pdfRegular, 10, width-40, v)...)
		}
	}
	boxH := float64(len(client))*13 + 42
	p.box(pdfPanel, grid, m, p.y-boxH, width, boxH)
	heading := t.BillToLabel
	if heading == "" || strings.EqualFold(heading, "bill to") {
		heading = "Client Details"
	}
	p.text(pdfBold, 11, m+16, p.y-22, pdfInk, heading)
	cy := p.y - 38
	for _, line := range client {
		p.text(pdfRegular, 10, m+16, cy, pdfInk, line)
		cy -= 13
	}
	p.y -= boxH + 24
	if t.HeaderNote != "" {
		p.block(m, width, "", 10, pdfInk, t.HeaderNote)
		p.y -= 12
	}

	// The lines: Item No. | Description | Qty | Line Total.
	cols := []float64{width * .33, width * .40, width * .095, width * .175}
	xs := []float64{m, m + cols[0], m + cols[0] + cols[1], m + cols[0] + cols[1] + cols[2], right}
	header := func() {
		p.room(60)
		p.rect(accent, m, p.y-26, width, 26)
		white := pdfColor{1, 1, 1}
		for i, h := range []string{"Item No.", "Description", "Qty", "Line Total"} {
			p.text(pdfBold, 10, xs[i]+cols[i]/2-pdfWidth(pdfBold, 10, h)/2, p.y-17, white, h)
		}
		p.y -= 26
	}
	header()
	for _, it := range d.items {
		detail := it.detail
		var car []string
		if it.vin != "" {
			car = append(car, "VIN: "+it.vin)
		}
		if it.stock != "" {
			car = append(car, "Stock: "+it.stock)
		}
		if len(car) > 0 {
			detail = strings.TrimSpace(detail + "\n" + strings.Join(car, "   "))
		}
		items := pdfWrap(pdfRegular, 10, cols[0]-16, it.description)
		descs := pdfWrap(pdfRegular, 10, cols[1]-16, detail)
		rowH := float64(max(len(items), len(descs), 1))*13 + 14
		if p.room(rowH) {
			header()
		}
		// The cell borders, then the text.
		for i := range cols {
			p.box(pdfColor{1, 1, 1}, grid, xs[i], p.y-rowH, cols[i], rowH)
		}
		ty := p.y - 17
		for i, line := range items {
			p.text(pdfRegular, 10, xs[0]+8, ty-float64(i)*13, pdfInk, line)
		}
		for i, line := range descs {
			p.text(pdfRegular, 10, xs[1]+8, ty-float64(i)*13, pdfInk, line)
		}
		qty := strconv.Itoa(it.quantity)
		p.text(pdfRegular, 10, xs[2]+cols[2]/2-pdfWidth(pdfRegular, 10, qty)/2, ty, pdfInk, qty)
		p.right(pdfRegular, 10, xs[4]-8, ty, pdfInk, invoiceMoney(it.lineCents))
		p.y -= rowH
	}
	p.y -= 22

	// The totals box at the right.
	rows := [][2]string{{"Subtotal", invoiceMoney(d.subtotal)}}
	discount := "Discount"
	if d.discountLabel != "" {
		discount = d.discountLabel
	}
	rows = append(rows, [2]string{discount, invoiceMoney(d.discount)}, [2]string{strings.NewReplacer(" (", " ", ")", "").Replace(d.taxLabel), invoiceMoney(d.tax)})
	if d.paid > 0 {
		rows = append(rows, [2]string{"Paid", "- " + invoiceMoney(d.paid)})
	}
	const totalsW = 220.0
	boxH = float64(len(rows)+1)*20 + 26
	p.room(boxH)
	left := right - totalsW
	p.box(pdfPanel, grid, left, p.y-boxH, totalsW, boxH)
	ry := p.y - 24
	for _, r := range rows {
		p.text(pdfRegular, 10, left+16, ry, pdfInk, r[0])
		p.right(pdfRegular, 10, right-16, ry, pdfInk, r[1])
		ry -= 20
	}
	last, lastValue := "Grand Total", invoiceMoney(d.total)
	if d.paid > 0 {
		last, lastValue = "Balance Due", invoiceMoney(d.due)
	}
	p.text(pdfBold, 10.5, left+16, ry, pdfInk, last)
	p.right(pdfBold, 10.5, right-16, ry, pdfInk, lastValue)
	p.y -= boxH + 30

	if t.PaymentInstructions != "" {
		p.block(m, width, "Payment", 10, pdfInk, t.PaymentInstructions)
		p.y -= 10
	}
	if d.note != "" {
		p.block(m, width, "Notes", 10, pdfInk, d.note)
		p.y -= 10
	}

	// The footer: a red rule, the business name, its email and website, and the computer-generated line.
	p.room(70)
	p.rect(accent, m, p.y, width, 2.5)
	p.y -= 18
	centered := func(font string, size float64, s string) {
		for _, line := range pdfWrap(font, size, width, s) {
			p.text(font, size, p.w/2-pdfWidth(font, size, line)/2, p.y, pdfInk, line)
			p.y -= size + 4
		}
	}
	centered(pdfBold, 10.5, name)
	if contact := footerContact(details); contact != "" {
		centered(pdfRegular, 9.5, contact)
	}
	if t.FooterNote != "" {
		centered(pdfRegular, 9.5, t.FooterNote)
	}
	centered(pdfRegular, 8.5, "This is a computer-generated invoice and no signature is required.")
	if d.franchise && d.issuer.LegalName != "" {
		centered(pdfRegular, 8.5, "Independently owned and operated by "+strings.TrimRight(d.issuer.LegalName, ".")+".")
	}
	return p.bytes(logoW, logoH, logo)
}

var (
	footerEmail = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	footerSite  = regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.)?[a-z0-9-]+(?:\.[a-z0-9-]+)*\.(?:ca|com|net|org|co)\b`)
)

// footerContact is "email | https://website" taken from the business details, as the owner's template shows in its footer.
func footerContact(details string) string {
	email := footerEmail.FindString(details)
	site := footerSite.FindString(footerEmail.ReplaceAllString(details, " "))
	if site != "" && !strings.HasPrefix(strings.ToLower(site), "http") {
		site = "https://" + site
	}
	var parts []string
	for _, v := range []string{email, site} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " | ")
}

// DealerTemplateSample draws a sample dealership invoice with a template as it is being edited (not saved), with the franchise's own
// business details, taxes and logo when a franchise is given, so the editor's preview is exactly what dealerships receive.
func (s *Store) DealerTemplateSample(ctx context.Context, actor, org, loc string, t InvoicePrintTemplate) ([]byte, error) {
	// Sample data only, plus the franchise's printed business details: a member of the organization, or anyone who reads the franchise's invoices.
	if loc != "" {
		if err := s.authorize(ctx, Scope{ActorID: actor, OrganizationID: org, LocationID: loc}, "invoices.read"); err != nil {
			if err = s.memberOf(ctx, actor, org); err != nil {
				return nil, err
			}
		}
	} else if err := s.memberOf(ctx, actor, org); err != nil {
		return nil, err
	}
	if err := t.normalize(); err != nil {
		return nil, err
	}
	now := time.Now().In(documentZone())
	d := invoiceDocument{b2b: true, number: "DDP00079", status: "issued", date: now.Format("02/Jan/2006"), dueDate: now.AddDate(0, 0, 15).Format("02/Jan/2006"), po: "PO-1001",
		issuer:        QuoteIssuer{LegalName: "Your legal business name", Address: "123 Example Street\nCity, Province A1A 1A1", TaxRegistration: "Tax number"},
		customer:      InvoiceCustomerSnapshot{Name: "Sample Dealership (1234567 Ontario Inc)", Phone: "+1 (705) 555-0100", Email: "buyer@example.com"},
		clientAddress: "1 Sample Road, City, Province A1A 1A1",
		items: []invoiceDocumentItem{{description: "Interior Detail", detail: "1212SPU - Crosstrek 2024", quantity: 1, unitPriceCents: 11500, lineCents: 11500},
			{description: "Interior Detail", detail: "6310SPNA - Crosstrek 2021", quantity: 1, unitPriceCents: 11500, lineCents: 11500},
			{description: "Surcharge", detail: "Online Payment Surcharge", quantity: 1, unitPriceCents: 749, lineCents: 749}},
		subtotal: 23749, taxLabel: "Tax", template: t}
	if loc != "" {
		var st InvoicePrintTemplateState
		if err := printParties(ctx, s.pool, org, loc, &st); err == nil {
			if st.Issuer != nil {
				d.issuer = *st.Issuer
			}
			if st.TaxRatePercent > 0 {
				d.tax = int64(math.Round(float64(d.subtotal) * st.TaxRatePercent / 100))
				d.taxLabel = invoiceTaxLabel(d.tax, d.subtotal, st.TaxName)
			}
		}
	}
	d.total, d.due = d.subtotal+d.tax, d.subtotal+d.tax
	if t.ShowLogo {
		d.logo = defaultInvoiceLogo
		if loc != "" {
			if _, own, _, found, e := s.readLocationLogo(ctx, org, loc); e == nil && found {
				d.logo = own
			}
		}
	}
	outlet, _ := outletOf(ctx, s.pool, org, loc)
	d.franchise = outlet == "franchise"
	return dealerInvoicePDF(d), nil
}
