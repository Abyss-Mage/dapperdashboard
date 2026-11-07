package postgres

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"testing"
)

func TestInvoicePDF(t *testing.T) {
	pdf := invoicePDF("INV-1001", []invoiceLine{{"Full detail (sedan)", "$250.00"}, {"Add-on: Pet hair × 2", "$40.00"}, {"Total", "$290.00"}, {"Paid", "$0.00"}, {"Balance due", "$290.00"}})
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
	if !bytes.Contains(pdf, []byte("(Add-on: Pet hair \xd7 2)")) || !bytes.Contains(pdf, []byte("(Invoice INV-1001)")) {
		t.Fatal("invoice text missing or not WinAnsi")
	}
	if pdfString(`a(b)\c`) != `a\(b\)\\c` {
		t.Fatal("PDF string escaping")
	}
	if out := os.Getenv("INVOICE_PDF_OUT"); out != "" {
		_ = os.WriteFile(out, pdf, 0o644)
	}
}
