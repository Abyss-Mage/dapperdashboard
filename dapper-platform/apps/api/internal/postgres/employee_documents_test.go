package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func verifyEmployeeDocuments(t *testing.T, ctx context.Context, s *Store, employee, owner, org string) {
	t.Helper()
	body := []byte("%PDF-1.4\n% synthetic policy document\n")
	sum := sha256.Sum256(body)
	register := func(title, file string, hash string) SOPDocument {
		d, err := s.RegisterSOPDocument(ctx, owner, org, SOPDocument{Title: title, Category: "onboarding", FileName: file, Sha256Hash: hash, PageCount: 1, Version: 1})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	good := register("Synthetic Onboarding Policy", "synthetic-policy.pdf", hex.EncodeToString(sum[:]))
	missing := register("Synthetic Missing File", "synthetic-missing.pdf", hex.EncodeToString(sum[:]))
	traversal := register("Synthetic Traversal", "../escape.pdf", hex.EncodeToString(sum[:]))

	if docs, err := s.EmployeeDocuments(ctx, employee); err != nil || len(docs) != 3 || docs[0].FileAvailable {
		t.Fatalf("documents without a configured directory %+v %v", docs, err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "synthetic-policy.pdf"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	s.SetSOPDirectory(dir)
	defer s.SetSOPDirectory("")

	docs, err := s.EmployeeDocuments(ctx, employee)
	if err != nil || len(docs) != 3 {
		t.Fatalf("documents %+v %v", docs, err)
	}
	available := map[string]bool{}
	for _, d := range docs {
		available[d.ID] = d.FileAvailable
		if d.AcknowledgedAt != nil {
			t.Fatal("acknowledged before the employee acted")
		}
	}
	if !available[good.ID] || available[missing.ID] || available[traversal.ID] {
		t.Fatalf("availability %+v", available)
	}
	if data, name, err := s.EmployeeDocumentFile(ctx, employee, good.ID); err != nil || !bytes.Equal(data, body) || name != "synthetic-policy.pdf" {
		t.Fatal("document did not round trip", err)
	}
	if _, _, err = s.EmployeeDocumentFile(ctx, employee, missing.ID); !errors.Is(err, ErrDocumentUnavailable) {
		t.Fatal("missing file served", err)
	}
	if _, _, err = s.EmployeeDocumentFile(ctx, employee, traversal.ID); !errors.Is(err, ErrDocumentUnavailable) {
		t.Fatal("path traversal served", err)
	}
	if _, _, err = s.EmployeeDocumentFile(ctx, employee, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrSOPDocumentNotFound) {
		t.Fatal("unknown document", err)
	}
	if _, _, err = s.EmployeeDocumentReceipt(ctx, employee, good.ID); !errors.Is(err, ErrDocumentNotAck) {
		t.Fatal("receipt before acknowledgment", err)
	}

	if _, err = s.EmployeeAcknowledgeDocument(ctx, employee, good.ID, " x "); !errors.Is(err, ErrDocumentSignature) {
		t.Fatal("one-letter signature accepted", err)
	}
	if _, err = s.EmployeeAcknowledgeDocument(ctx, employee, missing.ID, "Synthetic Employee"); !errors.Is(err, ErrDocumentUnavailable) {
		t.Fatal("acknowledged a document that could not be read", err)
	}
	ack, err := s.EmployeeAcknowledgeDocument(ctx, employee, good.ID, "  Synthetic   Employee ")
	if err != nil || ack.AcknowledgedAt == nil {
		t.Fatalf("acknowledge %+v %v", ack, err)
	}
	again, err := s.EmployeeAcknowledgeDocument(ctx, employee, good.ID, "Someone Else")
	if err != nil || again.AcknowledgedAt == nil || !again.AcknowledgedAt.Equal(*ack.AcknowledgedAt) {
		t.Fatal("repeat acknowledgment changed the first record", err)
	}
	pdf, name, err := s.EmployeeDocumentReceipt(ctx, employee, good.ID)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) || !strings.HasSuffix(name, ".pdf") ||
		!bytes.Contains(pdf, []byte("Policy acknowledgment receipt")) || !bytes.Contains(pdf, []byte("Synthetic Employee")) || bytes.Contains(pdf, []byte("Someone Else")) || !bytes.Contains(pdf, []byte(hex.EncodeToString(sum[:]))) {
		t.Fatalf("receipt content (%d bytes) %v", len(pdf), err)
	}
	// A changed file no longer verifies, so it can neither be served nor newly acknowledged.
	if err = os.WriteFile(filepath.Join(dir, "synthetic-policy.pdf"), append(body, 'x'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.EmployeeDocumentFile(ctx, employee, good.ID); !errors.Is(err, ErrDocumentUnavailable) {
		t.Fatal("tampered file served", err)
	}
	var stranger string
	if err = s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('synthetic-docs-stranger@example.invalid') RETURNING id::text`).Scan(&stranger); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EmployeeDocuments(ctx, stranger); !errors.Is(err, ErrEmployeeAssignment) {
		t.Fatal("unassigned principal listed documents", err)
	}
}

func TestReceiptPDFEscapesText(t *testing.T) {
	pdf := buildReceiptPDF([]string{"Title", "Sub", "", "Name: Zoë (test) \\ end"})
	if !bytes.Contains(pdf, []byte(`Name: Zo? \(test\) \\ end`)) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("unsafe PDF text: %q", pdf)
	}
}
