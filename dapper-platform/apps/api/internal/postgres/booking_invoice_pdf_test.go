package postgres

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func sampleInvoiceDocument() invoiceDocument {
	t := DefaultInvoicePrintTemplate()
	t.PaymentInstructions = "Pay at the shop or online."
	return invoiceDocument{number: "INV-1001", status: "partially_paid", date: "November 8, 2026", taxLabel: "Tax (HST 13%)",
		issuer:   QuoteIssuer{LegalName: "Dapper Peterborough Inc.", Address: "850 The Kingsway\nPeterborough, ON", TaxRegistration: "123456789RT0001"},
		customer: InvoiceCustomerSnapshot{Name: "Ann Synthetic", Email: "ann@example.invalid", Phone: "+1 705 555 0100"},
		items: []invoiceDocumentItem{
			{description: "Full detail (sedan)", quantity: 1, unitPriceCents: 25000, lineCents: 25000},
			{description: "Pet hair × 2", quantity: 2, unitPriceCents: 2000, lineCents: 4000},
		},
		subtotal: 29000, tax: 3770, total: 32770, paid: 10000, due: 22770, template: t, logo: defaultInvoiceLogo}
}

func checkPDFStructure(t *testing.T, pdf []byte) {
	t.Helper()
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4\n")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatal("not a PDF")
	}
	// startxref must point at the xref table, and each object offset at its "n 0 obj" line, or readers call the file damaged.
	m := regexp.MustCompile(`startxref\n(\d+)\n`).FindSubmatch(pdf)
	xref, _ := strconv.Atoi(string(m[1]))
	if !bytes.HasPrefix(pdf[xref:], []byte("xref\n")) {
		t.Fatal("startxref is off")
	}
	for i, off := range regexp.MustCompile(`(\d{10}) 00000 n`).FindAllSubmatch(pdf, -1) {
		n, _ := strconv.Atoi(string(off[1]))
		if !bytes.HasPrefix(pdf[n:], []byte(strconv.Itoa(i+1)+" 0 obj")) {
			t.Fatalf("object %d offset is off", i+1)
		}
	}
}

func TestInvoicePDF(t *testing.T) {
	d := sampleInvoiceDocument()
	pdf := invoicePDF(d)
	checkPDFStructure(t, pdf)
	for _, want := range []string{"(Pet hair \xd7 2)", "(INVOICE)", "(No. INV-1001)", "(Dapper Peterborough Inc.)", "(Tax \\(HST 13%\\))", "($327.70)", "(Pay at the shop or online.)", "/Im1 Do", "/Subtype /Image"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Fatalf("missing %q", want)
		}
	}
	if bytes.Contains(pdf, []byte("Independently owned")) {
		t.Fatal("brand-owned invoice carries the franchise line")
	}
	if pdfString(`a(b)\c`) != `a\(b\)\\c` {
		t.Fatal("PDF string escaping")
	}

	d.franchise = true
	franchise := invoicePDF(d)
	if !bytes.Contains(franchise, []byte("(Independently owned and operated by Dapper Peterborough Inc.)")) {
		t.Fatal("franchise invoice is missing the ownership line")
	}

	d = sampleInvoiceDocument()
	d.template.ShowLogo, d.logo = false, nil
	d.template.PaperSize, d.template.AccentColor, d.template.Title = "a4", "#0000ff", "Receipt"
	plain := invoicePDF(d)
	checkPDFStructure(t, plain)
	if bytes.Contains(plain, []byte("/Im1")) || !bytes.Contains(plain, []byte("[0 0 595 842]")) || !bytes.Contains(plain, []byte("(RECEIPT)")) || !bytes.Contains(plain, []byte("0.000 0.000 1.000 rg")) {
		t.Fatal("template settings not followed")
	}
	if out := os.Getenv("INVOICE_PDF_OUT"); out != "" {
		_ = os.WriteFile(out, franchise, 0o644)
	}
}

func TestInvoicePDFRunsOntoMorePagesAndWraps(t *testing.T) {
	d := sampleInvoiceDocument()
	d.items = nil
	for range 60 {
		d.items = append(d.items, invoiceDocumentItem{description: strings.Repeat("A long service description ", 6), quantity: 1, unitPriceCents: 100, lineCents: 100})
	}
	pdf := invoicePDF(d)
	checkPDFStructure(t, pdf)
	if n := bytes.Count(pdf, []byte("/Type /Page /Parent")); n < 3 {
		t.Fatalf("expected several pages, got %d", n)
	}
	for _, line := range pdfWrap(pdfRegular, 10, 100, "Averyveryveryveryveryveryveryverylongwordwithnospaces and\nmore") {
		if pdfWidth(pdfRegular, 10, line) > 100 {
			t.Fatalf("line too wide: %q", line)
		}
	}
}

func TestInvoiceMoneyAndTaxLabel(t *testing.T) {
	for cents, want := range map[int64]string{0: "$0.00", 5: "$0.05", 123456789: "$1,234,567.89", 100000: "$1,000.00", -2500: "-$25.00"} {
		if got := invoiceMoney(cents); got != want {
			t.Fatalf("%d: %s", cents, got)
		}
	}
	if got := invoiceTaxLabel(1300, 10000, "HST"); got != "Tax (HST 13%)" {
		t.Fatal(got)
	}
	if got := invoiceTaxLabel(0, 10000, "HST"); got != "Tax" {
		t.Fatal(got)
	}
}

func TestLogoImageFormats(t *testing.T) {
	w, h, rgb, ok := pdfLogoImage(defaultInvoiceLogo)
	if !ok || w == 0 || h == 0 || len(rgb) != w*h*3 || w > 256 || h > 256 {
		t.Fatalf("default logo not usable: %d %d %d %v", w, h, len(rgb), ok)
	}
	if _, _, _, ok = pdfLogoImage([]byte("not an image")); ok {
		t.Fatal("garbage accepted as a logo")
	}
}
