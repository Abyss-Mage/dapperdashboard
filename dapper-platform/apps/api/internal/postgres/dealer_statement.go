package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The dealership statement (error404 #31): the dealership's invoices in a date range with what is still owed, optionally the payments in
// that range and an aging summary of what is unpaid, as a page and as a PDF in the dealership invoice style.

type StatementRow struct {
	InvoiceID     string `json:"invoiceId"`
	InvoiceNumber string `json:"invoiceNumber"`
	InvoiceDate   string `json:"invoiceDate"`
	DueDate       string `json:"dueDate"`
	Status        string `json:"status"`
	Overdue       bool   `json:"overdue"`
	TotalCents    int64  `json:"totalCents"`
	BalanceCents  int64  `json:"balanceCents"`
}

type StatementAging struct {
	Current int64 `json:"current"`
	Days30  int64 `json:"days1to30"`
	Days60  int64 `json:"days31to60"`
	Days90  int64 `json:"days61to90"`
	Over90  int64 `json:"over90"`
}

type Statement struct {
	From         string          `json:"from"`
	To           string          `json:"to"`
	Rows         []StatementRow  `json:"rows"`
	Payments     []PortalPayment `json:"payments"`
	BalanceCents int64           `json:"balanceCents"`
	BilledCents  int64           `json:"billedCents"`
	Aging        StatementAging  `json:"aging"`
}

func statementDay(s DealerInvoiceSummary) string {
	if s.InvoiceDate != "" {
		return s.InvoiceDate
	}
	t := s.CreatedAt
	if s.IssuedAt != nil {
		t = *s.IssuedAt
	}
	return t.In(documentZone()).Format("2006-01-02")
}

// PortalStatement builds the statement for the dates (YYYY-MM-DD, inclusive) and status (all, paid, unpaid, overdue).
func (s *Store) PortalStatement(ctx context.Context, w PortalWho, from, to, status string) (Statement, error) {
	for _, d := range []string{from, to} {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return Statement{}, ErrPortalInput
		}
	}
	if from > to {
		return Statement{}, ErrPortalInput
	}
	switch status {
	case "", "all", "paid", "unpaid", "overdue":
	default:
		return Statement{}, ErrPortalInput
	}
	list, err := s.PortalInvoices(ctx, w)
	if err != nil {
		return Statement{}, err
	}
	out := Statement{From: from, To: to, Rows: []StatementRow{}, Payments: []PortalPayment{}}
	today := dealerToday()
	for i := len(list) - 1; i >= 0; i-- { // oldest first, like a ledger
		x := list[i]
		day := statementDay(x)
		if x.Status == "void" || day < from || day > to {
			continue
		}
		open := x.Status == "issued" || x.Status == "partially_paid"
		if (status == "paid" && x.Status != "paid") || (status == "unpaid" && !open) || (status == "overdue" && !x.Overdue) {
			continue
		}
		due := x.DueDate
		if due == "" && x.DueAt != nil {
			due = x.DueAt.In(documentZone()).Format("2006-01-02")
		}
		out.Rows = append(out.Rows, StatementRow{InvoiceID: x.ID, InvoiceNumber: x.InvoiceNumber, InvoiceDate: day, DueDate: due, Status: x.Status, Overdue: x.Overdue, TotalCents: x.TotalCents, BalanceCents: x.DueCents})
		out.BilledCents += x.TotalCents
		if !open {
			continue
		}
		out.BalanceCents += x.DueCents
		late := 0
		if t, e := time.Parse("2006-01-02", due); e == nil {
			late = int(today.Sub(t).Hours() / 24)
		}
		switch {
		case late <= 0:
			out.Aging.Current += x.DueCents
		case late <= 30:
			out.Aging.Days30 += x.DueCents
		case late <= 60:
			out.Aging.Days60 += x.DueCents
		case late <= 90:
			out.Aging.Days90 += x.DueCents
		default:
			out.Aging.Over90 += x.DueCents
		}
	}
	pays, err := s.PortalPayments(ctx, w)
	if err != nil {
		return Statement{}, err
	}
	for _, p := range pays {
		if d := p.RecordedAt.In(documentZone()).Format("2006-01-02"); d >= from && d <= to {
			out.Payments = append(out.Payments, p)
		}
	}
	return out, nil
}

func statementShort(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02/Jan/2006")
}

// PortalStatementPDF draws the statement with the franchise's dealership template look (logo, business details, colour).
func (s *Store) PortalStatementPDF(ctx context.Context, w PortalWho, st Statement, withPayments, withAging bool) (string, []byte) {
	var d invoiceDocument
	s.dressDocument(ctx, &d, w.Scope.OrganizationID, w.Scope.LocationID, dealerPrint)
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
		accent = pdfColor{.894, .102, .169}
	}
	grid := pdfColor{.85, .85, .85}
	top := p.h - 36
	logoW, logoH, logo, hasLogo := 0, 0, []byte(nil), false
	if d.logo != nil {
		logoW, logoH, logo, hasLogo = pdfLogoImage(d.logo)
	}
	logoBottom := top
	if hasLogo {
		scale := 90 / float64(max(logoW, logoH))
		lw, lh := float64(logoW)*scale, float64(logoH)*scale
		fmt.Fprintf(p.page(), "q %.2f 0 0 %.2f %.2f %.2f cm /Im1 Do Q\n", lw, lh, right-lw, top-lh)
		logoBottom = top - lh
	}
	y := top - 16
	name := d.issuer.LegalName
	if name == "" {
		name = w.BusinessName
	}
	p.text(pdfBold, 15, m, y, pdfInk, name)
	y -= 16
	details := strings.TrimSpace(t.BusinessDetails + "\n" + d.issuer.Address)
	for _, line := range pdfWrap(pdfRegular, 9.5, width-130, details) {
		if line != "" {
			p.text(pdfRegular, 9.5, m, y, pdfInk, line)
			y -= 12.5
		}
	}
	p.y = min(y, logoBottom) - 24
	p.text(pdfBold, 11, m, p.y, pdfInk, w.DealershipName)
	for _, line := range pdfWrap(pdfRegular, 9.5, width/2, strings.TrimSpace(w.ContactEmail+"\n"+w.Address)) {
		if line != "" {
			p.y -= 13
			p.text(pdfRegular, 9.5, m, p.y, pdfInk, line)
		}
	}
	p.right(pdfBold, 18, right, p.y+14, pdfInk, "Statement")
	p.right(pdfRegular, 9.5, right, p.y-4, pdfInk, "Statement Date  "+statementShort(st.From)+" - "+statementShort(st.To))
	p.right(pdfBold, 9.5, right, p.y-18, pdfInk, "Balance Due  "+invoiceMoney(st.BalanceCents))
	p.y -= 40

	table := func(heads []string, widths []float64, alignRight []bool, rows [][]string) {
		header := func() {
			p.room(50)
			p.rect(accent, m, p.y-24, width, 24)
			x := m
			for i, h := range heads {
				if alignRight[i] {
					p.right(pdfBold, 9.5, x+widths[i]-8, p.y-16, pdfColor{1, 1, 1}, h)
				} else {
					p.text(pdfBold, 9.5, x+8, p.y-16, pdfColor{1, 1, 1}, h)
				}
				x += widths[i]
			}
			p.y -= 24
		}
		header()
		for _, r := range rows {
			if p.room(22) {
				header()
			}
			x := m
			for i, v := range r {
				if alignRight[i] {
					p.right(pdfRegular, 9.5, x+widths[i]-8, p.y-15, pdfInk, v)
				} else {
					p.text(pdfRegular, 9.5, x+8, p.y-15, pdfInk, v)
				}
				x += widths[i]
			}
			p.y -= 22
			p.rect(grid, m, p.y, width, .75)
		}
		p.y -= 16
	}
	var rows [][]string
	for _, r := range st.Rows {
		state := map[string]string{"issued": "Unpaid", "partially_paid": "Part paid", "paid": "Paid"}[r.Status]
		if r.Overdue {
			state = "Past due"
		}
		rows = append(rows, []string{r.InvoiceNumber, statementShort(r.InvoiceDate), statementShort(r.DueDate), state, invoiceMoney(r.TotalCents), invoiceMoney(r.BalanceCents)})
	}
	if len(rows) == 0 {
		p.text(pdfRegular, 10, m, p.y, pdfMute, "No invoices in these dates.")
		p.y -= 24
	} else {
		table([]string{"Invoice Number", "Invoice Date", "Due Date", "Status", "Total", "Balance"}, []float64{width * .2, width * .16, width * .16, width * .14, width * .17, width * .17}, []bool{false, false, false, false, true, true}, rows)
	}
	if withPayments {
		p.room(40)
		p.text(pdfBold, 11, m, p.y, pdfInk, "Payments")
		p.y -= 10
		var prow [][]string
		for _, pay := range st.Payments {
			prow = append(prow, []string{pay.RecordedAt.In(documentZone()).Format("02/Jan/2006"), pay.InvoiceNumber, invoicePaymentWords(pay.Method), invoiceMoney(pay.AmountCents)})
		}
		if len(prow) == 0 {
			p.y -= 14
			p.text(pdfRegular, 10, m, p.y, pdfMute, "No payments in these dates.")
			p.y -= 24
		} else {
			table([]string{"Date", "Invoice", "Method", "Amount"}, []float64{width * .25, width * .25, width * .25, width * .25}, []bool{false, false, false, true}, prow)
		}
	}
	if withAging {
		p.room(70)
		p.text(pdfBold, 11, m, p.y, pdfInk, "Aging")
		p.y -= 10
		a := st.Aging
		table([]string{"Current", "1-30 days", "31-60 days", "61-90 days", "Over 90 days"}, []float64{width / 5, width / 5, width / 5, width / 5, width / 5}, []bool{true, true, true, true, true},
			[][]string{{invoiceMoney(a.Current), invoiceMoney(a.Days30), invoiceMoney(a.Days60), invoiceMoney(a.Days90), invoiceMoney(a.Over90)}})
	}
	p.room(40)
	p.rect(accent, m, p.y, width, 2.5)
	p.y -= 18
	for _, line := range []string{name, footerContact(details)} {
		if line != "" {
			p.text(pdfRegular, 9.5, p.w/2-pdfWidth(pdfRegular, 9.5, line)/2, p.y, pdfInk, line)
			p.y -= 13
		}
	}
	return "Statement-" + unsafeFileName.ReplaceAllString(w.DealershipName, "-") + "-" + st.From + "-to-" + st.To + ".pdf", p.bytes(logoW, logoH, logo)
}
