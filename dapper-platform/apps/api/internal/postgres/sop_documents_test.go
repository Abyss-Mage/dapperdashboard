package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifySOPDocuments(t *testing.T, ctx context.Context, s *Store, a, b Scope) {
	t.Helper()

	// Grant required actions to a.ActorID
	if _, err := s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES
($1,$2,$3,'sop.read'),
($1,$2,$3,'sop.acknowledge'),
($1,$2,$3,'sop.manage') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}

	testHash := "db5732a72dba4bfcbf455a846c30467a2c06910abf4e601a7d90efc9d377d587" // Interior detail SOP hash

	// 1. Register SOP Document
	doc, err := s.RegisterSOPDocument(ctx, a.ActorID, a.OrganizationID, SOPDocument{
		Title:      "Interior Detail SOP",
		Category:   "procedure",
		FileName:   "interior_detail_sop.pdf",
		Sha256Hash: testHash,
		Version:    1,
		PageCount:  7,
	})
	if err != nil {
		t.Fatalf("failed to register SOP document: %v", err)
	}
	if doc.ID == "" || doc.Title != "Interior Detail SOP" || doc.PageCount != 7 {
		t.Fatalf("unexpected document state: %+v", doc)
	}

	// 2. List SOP Documents
	docs, err := s.ListSOPDocuments(ctx, a)
	if err != nil {
		t.Fatalf("failed to list SOP documents: %v", err)
	}
	if len(docs) < 1 {
		t.Fatalf("expected at least 1 document, got %d", len(docs))
	}

	// 3. Employee Acknowledgment with matching hash
	ack, err := s.AcknowledgeSOP(ctx, a, a.ActorID, doc.ID, "orientation", testHash, "Completed orientation walkthrough")
	if err != nil {
		t.Fatalf("failed to acknowledge SOP: %v", err)
	}
	if ack.ID == "" || ack.TrainingTrack != "orientation" || ack.DocumentTitle != doc.Title {
		t.Fatalf("unexpected acknowledgment state: %+v", ack)
	}

	// 4. Mismatched hash rejection
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = s.AcknowledgeSOP(ctx, a, a.ActorID, doc.ID, "assisted", wrongHash, "Invalid hash attempt")
	if !errors.Is(err, ErrSOPHashMismatch) {
		t.Fatalf("expected ErrSOPHashMismatch, got: %v", err)
	}

	// 5. Invalid training track rejection
	_, err = s.AcknowledgeSOP(ctx, a, a.ActorID, doc.ID, "master_trainer", testHash, "Invalid track")
	if !errors.Is(err, ErrSOPAcknowledgmentInput) {
		t.Fatalf("expected ErrSOPAcknowledgmentInput, got: %v", err)
	}

	// 6. Query employee acknowledgments
	acks, err := s.ListSOPAcknowledgments(ctx, a, a.ActorID)
	if err != nil {
		t.Fatalf("failed to list acknowledgments: %v", err)
	}
	if len(acks) != 1 || acks[0].TrainingTrack != "orientation" {
		t.Fatalf("expected 1 acknowledgment, got: %+v", acks)
	}

	// 7. Cross-organization isolation: scopeB cannot list scopeA's documents
	bDocs, err := s.ListSOPDocuments(ctx, b)
	if err != ErrForbidden && len(bDocs) != 0 {
		t.Fatalf("cross-scope document read leaked: %+v", bDocs)
	}
}
