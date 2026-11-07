package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrSOPDocumentInput       = errors.New("valid document title, category, file name, hash and page count required")
	ErrSOPDocumentNotFound    = errors.New("SOP document not found")
	ErrSOPAcknowledgmentInput = errors.New("valid document, employee, training track and matching hash required")
	ErrSOPHashMismatch        = errors.New("document hash mismatch with current revision")
)

type SOPDocument struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Title          string    `json:"title"`
	Category       string    `json:"category"`
	FileName       string    `json:"fileName"`
	Sha256Hash     string    `json:"sha256Hash"`
	Version        int       `json:"version"`
	PageCount      int       `json:"pageCount"`
	EffectiveDate  string    `json:"effectiveDate"`
	Archived       bool      `json:"archived"`
	CreatedAt      time.Time `json:"createdAt"`
}

type SOPAcknowledgment struct {
	ID                 string    `json:"id"`
	OrganizationID     string    `json:"organizationId"`
	LocationID         string    `json:"locationId"`
	SOPDocumentID      string    `json:"sopDocumentId"`
	DocumentTitle      string    `json:"documentTitle,omitempty"`
	EmployeeID         string    `json:"employeeId"`
	ActorID            string    `json:"actorId"`
	AcknowledgedSha256 string    `json:"acknowledgedSha256"`
	TrainingTrack      string    `json:"trainingTrack"`
	Notes              string    `json:"notes"`
	AcknowledgedAt     time.Time `json:"acknowledgedAt"`
	CreatedAt          time.Time `json:"createdAt"`
}

func (s *Store) RegisterSOPDocument(ctx context.Context, actorID, orgID string, doc SOPDocument) (SOPDocument, error) {
	if actorID == "" || orgID == "" {
		return SOPDocument{}, ErrScope
	}
	title := strings.TrimSpace(doc.Title)
	if utf8.RuneCountInString(title) < 3 || utf8.RuneCountInString(title) > 150 {
		return SOPDocument{}, ErrSOPDocumentInput
	}
	category := strings.TrimSpace(doc.Category)
	switch category {
	case "safety", "chemical", "procedure", "onboarding":
	default:
		return SOPDocument{}, ErrSOPDocumentInput
	}
	fileName := strings.TrimSpace(doc.FileName)
	if utf8.RuneCountInString(fileName) < 3 || utf8.RuneCountInString(fileName) > 150 {
		return SOPDocument{}, ErrSOPDocumentInput
	}
	hash := strings.ToLower(strings.TrimSpace(doc.Sha256Hash))
	if len(hash) != 64 {
		return SOPDocument{}, ErrSOPDocumentInput
	}
	if doc.PageCount <= 0 {
		return SOPDocument{}, ErrSOPDocumentInput
	}
	version := doc.Version
	if version <= 0 {
		version = 1
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO sop_documents(
			organization_id, title, category, file_name, sha256_hash, version, page_count
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (organization_id, file_name, version) DO UPDATE
		SET title=EXCLUDED.title, category=EXCLUDED.category, sha256_hash=EXCLUDED.sha256_hash, page_count=EXCLUDED.page_count, archived=false
		RETURNING id::text, organization_id::text, title, category, file_name, sha256_hash, version, page_count,
			effective_date::text, archived, created_at
	`, orgID, title, category, fileName, hash, version, doc.PageCount)

	var saved SOPDocument
	err := row.Scan(
		&saved.ID, &saved.OrganizationID, &saved.Title, &saved.Category, &saved.FileName,
		&saved.Sha256Hash, &saved.Version, &saved.PageCount, &saved.EffectiveDate,
		&saved.Archived, &saved.CreatedAt,
	)
	return saved, err
}

func (s *Store) ListSOPDocuments(ctx context.Context, scope Scope) ([]SOPDocument, error) {
	if err := s.authorize(ctx, scope, "sop.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, organization_id::text, title, category, file_name, sha256_hash, version, page_count,
			effective_date::text, archived, created_at
		FROM sop_documents
		WHERE organization_id=$1 AND NOT archived
		ORDER BY category ASC, title ASC
	`, scope.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SOPDocument
	for rows.Next() {
		var doc SOPDocument
		if err := rows.Scan(
			&doc.ID, &doc.OrganizationID, &doc.Title, &doc.Category, &doc.FileName,
			&doc.Sha256Hash, &doc.Version, &doc.PageCount, &doc.EffectiveDate,
			&doc.Archived, &doc.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, doc)
	}
	if list == nil {
		list = []SOPDocument{}
	}
	return list, rows.Err()
}

func (s *Store) AcknowledgeSOP(ctx context.Context, scope Scope, employeeID, docID, track, hash, notes string) (SOPAcknowledgment, error) {
	if err := s.authorize(ctx, scope, "sop.acknowledge"); err != nil {
		return SOPAcknowledgment{}, err
	}
	employeeID = strings.TrimSpace(employeeID)
	docID = strings.TrimSpace(docID)
	track = strings.TrimSpace(track)
	hash = strings.ToLower(strings.TrimSpace(hash))
	if employeeID == "" || docID == "" || len(hash) != 64 {
		return SOPAcknowledgment{}, ErrSOPAcknowledgmentInput
	}
	switch track {
	case "orientation", "assisted", "independent", "certified":
	default:
		return SOPAcknowledgment{}, ErrSOPAcknowledgmentInput
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SOPAcknowledgment{}, err
	}
	defer tx.Rollback(ctx)

	// Verify employee principal exists
	var empActive bool
	err = tx.QueryRow(ctx, `SELECT active FROM principals WHERE id=$1`, employeeID).Scan(&empActive)
	if errors.Is(err, pgx.ErrNoRows) || !empActive {
		return SOPAcknowledgment{}, ErrSOPAcknowledgmentInput
	}
	if err != nil {
		return SOPAcknowledgment{}, err
	}

	// Verify doc and hash
	var expectedHash string
	var docTitle string
	err = tx.QueryRow(ctx, `
		SELECT sha256_hash, title FROM sop_documents
		WHERE id=$1 AND organization_id=$2 AND NOT archived
	`, docID, scope.OrganizationID).Scan(&expectedHash, &docTitle)
	if errors.Is(err, pgx.ErrNoRows) {
		return SOPAcknowledgment{}, ErrSOPDocumentNotFound
	}
	if err != nil {
		return SOPAcknowledgment{}, err
	}
	if !strings.EqualFold(expectedHash, hash) {
		return SOPAcknowledgment{}, ErrSOPHashMismatch
	}

	var ack SOPAcknowledgment
	row := tx.QueryRow(ctx, `
		INSERT INTO sop_acknowledgments(
			organization_id, location_id, sop_document_id, employee_id, actor_id,
			acknowledged_sha256, training_track, notes
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8
		)
		ON CONFLICT (organization_id, location_id, sop_document_id, employee_id, training_track)
		DO UPDATE SET actor_id=EXCLUDED.actor_id, acknowledged_sha256=EXCLUDED.acknowledged_sha256,
			notes=EXCLUDED.notes, acknowledged_at=now()
		RETURNING id::text, organization_id::text, location_id::text, sop_document_id::text,
			employee_id::text, actor_id::text, acknowledged_sha256, training_track, notes,
			acknowledged_at, created_at
	`, scope.OrganizationID, scope.LocationID, docID, employeeID, scope.ActorID,
		hash, track, strings.TrimSpace(notes))

	if err := row.Scan(
		&ack.ID, &ack.OrganizationID, &ack.LocationID, &ack.SOPDocumentID,
		&ack.EmployeeID, &ack.ActorID, &ack.AcknowledgedSha256, &ack.TrainingTrack, &ack.Notes,
		&ack.AcknowledgedAt, &ack.CreatedAt,
	); err != nil {
		return SOPAcknowledgment{}, err
	}
	ack.DocumentTitle = docTitle

	if err := tx.Commit(ctx); err != nil {
		return SOPAcknowledgment{}, err
	}
	return ack, nil
}

func (s *Store) ListSOPAcknowledgments(ctx context.Context, scope Scope, employeeID string) ([]SOPAcknowledgment, error) {
	if err := s.authorize(ctx, scope, "sop.read"); err != nil {
		return nil, err
	}

	var rows pgx.Rows
	var err error
	if employeeID != "" {
		rows, err = s.pool.Query(ctx, `
			SELECT a.id::text, a.organization_id::text, a.location_id::text, a.sop_document_id::text,
				d.title, a.employee_id::text, a.actor_id::text, a.acknowledged_sha256, a.training_track,
				a.notes, a.acknowledged_at, a.created_at
			FROM sop_acknowledgments a
			JOIN sop_documents d ON d.id=a.sop_document_id
			WHERE a.organization_id=$1 AND a.location_id=$2 AND a.employee_id=$3
			ORDER BY a.acknowledged_at DESC
		`, scope.OrganizationID, scope.LocationID, employeeID)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT a.id::text, a.organization_id::text, a.location_id::text, a.sop_document_id::text,
				d.title, a.employee_id::text, a.actor_id::text, a.acknowledged_sha256, a.training_track,
				a.notes, a.acknowledged_at, a.created_at
			FROM sop_acknowledgments a
			JOIN sop_documents d ON d.id=a.sop_document_id
			WHERE a.organization_id=$1 AND a.location_id=$2
			ORDER BY a.acknowledged_at DESC
			LIMIT 100
		`, scope.OrganizationID, scope.LocationID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SOPAcknowledgment
	for rows.Next() {
		var a SOPAcknowledgment
		if err := rows.Scan(
			&a.ID, &a.OrganizationID, &a.LocationID, &a.SOPDocumentID,
			&a.DocumentTitle, &a.EmployeeID, &a.ActorID, &a.AcknowledgedSha256,
			&a.TrainingTrack, &a.Notes, &a.AcknowledgedAt, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	if list == nil {
		list = []SOPAcknowledgment{}
	}
	return list, rows.Err()
}
