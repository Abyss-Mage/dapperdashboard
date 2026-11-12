package postgres

import (
	"bytes"
	"os"
	"testing"
)

// The dealership layout draws the owner's template: header row, client box, the four-column table and the totals.
func TestDealerInvoicePDFLayout(t *testing.T) {
	tpl := DefaultInvoicePrintTemplate()
	tpl.BusinessDetails = "Email: info@example.test | Phone: +1 (705) 555-0100\nBIN: 1000000000 | HST: 12345 6789 RT0001\nexample-detailing.ca"
	d := invoiceDocument{b2b: true, number: "DDP00079", status: "issued", date: "21/Nov/2025", dueDate: "06/Dec/2025", po: "PO-1",
		issuer:   QuoteIssuer{LegalName: "Synthetic Detailing Pros", Address: "1 Example Road\nPeterborough, Ontario"},
		customer: InvoiceCustomerSnapshot{Name: "Synthetic Subaru", Phone: "+1 (705) 555-0101", Email: "buyer@example.test"},
		items: []invoiceDocumentItem{{description: "Interior Detail - Subaru", detail: "1212SPU - Crosstrek 2024", quantity: 1, unitPriceCents: 11500, lineCents: 11500},
			{description: "Surcharge", detail: "Online Payment Surcharge", quantity: 1, unitPriceCents: 749, lineCents: 749}},
		subtotal: 12249, tax: 1592, total: 13841, due: 13841, taxLabel: "HST 13%", template: tpl, logo: defaultInvoiceLogo}
	pdf := invoicePDF(d)
	for _, want := range []string{"Item No.", "Line Total", "Client Details", "Grand Total", "Invoice #:", "Due:", "PO:", "info@example.test | https://example-detailing.ca", "computer-generated invoice"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Fatalf("dealership PDF is missing %q", want)
		}
	}
	if out := os.Getenv("DAPPER_PDF_SAMPLE"); out != "" {
		_ = os.WriteFile(out, pdf, 0o600)
	}
}
