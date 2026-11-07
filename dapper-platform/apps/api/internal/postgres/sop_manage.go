package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// MaxSOPFileBytes is the largest policy PDF an administrator can add.
const MaxSOPFileBytes = 10 << 20

var (
	ErrSOPFileInput   = errors.New("the document must be a PDF under 10 MB with a title of 3 to 150 characters and a valid category")
	ErrSOPStorageOff  = errors.New("document storage is not set up on this server")
	sopPageMarker     = regexp.MustCompile(`/Type\s*/Page[^s]`)
	sopCategoryChoice = map[string]bool{"safety": true, "chemical": true, "procedure": true, "onboarding": true}
)

// AddSOPDocument stores an uploaded PDF and publishes it to employees. The file is named after its checksum so two
// different uploads can never overwrite each other, and the original bytes are never modified. Adding a PDF that was
// removed earlier brings it back.
func (s *Store) AddSOPDocument(ctx context.Context, scope Scope, title, category string, data []byte) (SOPDocument, error) {
	if err := s.authorize(ctx, scope, "sop.manage"); err != nil {
		return SOPDocument{}, err
	}
	title = strings.Join(strings.Fields(title), " ")
	if n := utf8.RuneCountInString(title); n < 3 || n > 150 || !sopCategoryChoice[category] ||
		len(data) < 8 || len(data) > MaxSOPFileBytes || !bytes.HasPrefix(data, []byte("%PDF-")) {
		return SOPDocument{}, ErrSOPFileInput
	}
	if s.sopDir == "" {
		return SOPDocument{}, ErrSOPStorageOff
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	fileName := hash[:24] + ".pdf"
	if err := os.MkdirAll(s.sopDir, 0o750); err != nil {
		return SOPDocument{}, ErrSOPStorageOff
	}
	path := filepath.Join(s.sopDir, fileName)
	if existing, err := os.ReadFile(path); err != nil || !bytes.Equal(existing, data) {
		tmp := path + ".part"
		if err = os.WriteFile(tmp, data, 0o640); err != nil {
			return SOPDocument{}, ErrSOPStorageOff
		}
		if err = os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return SOPDocument{}, ErrSOPStorageOff
		}
	}
	pages := len(sopPageMarker.FindAll(data, -1))
	if pages < 1 {
		pages = 1
	}
	var doc SOPDocument
	err := s.pool.QueryRow(ctx, `
		INSERT INTO sop_documents(organization_id, title, category, file_name, sha256_hash, version, page_count)
		VALUES ($1,$2,$3,$4,$5,1,$6)
		ON CONFLICT (organization_id, file_name, version) DO UPDATE
		SET title=EXCLUDED.title, category=EXCLUDED.category, archived=false
		RETURNING id::text, organization_id::text, title, category, file_name, sha256_hash, version, page_count,
			effective_date::text, archived, created_at
	`, scope.OrganizationID, title, category, fileName, hash, pages).Scan(
		&doc.ID, &doc.OrganizationID, &doc.Title, &doc.Category, &doc.FileName, &doc.Sha256Hash,
		&doc.Version, &doc.PageCount, &doc.EffectiveDate, &doc.Archived, &doc.CreatedAt)
	return doc, err
}

// RemoveSOPDocument takes a document off the employee list. The record and file stay so earlier acknowledgments keep
// their evidence.
func (s *Store) RemoveSOPDocument(ctx context.Context, scope Scope, id string) error {
	if err := s.authorize(ctx, scope, "sop.manage"); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE sop_documents SET archived=true WHERE id=$1 AND organization_id=$2 AND NOT archived`, id, scope.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSOPDocumentNotFound
	}
	return nil
}

// SOPDocumentFile returns a listed document's PDF for administrators who can read SOPs, after checking its checksum.
func (s *Store) SOPDocumentFile(ctx context.Context, scope Scope, id string) ([]byte, string, error) {
	if err := s.authorize(ctx, scope, "sop.read"); err != nil {
		return nil, "", err
	}
	var title, fileName, hash string
	err := s.pool.QueryRow(ctx, `SELECT title, file_name, sha256_hash FROM sop_documents WHERE id=$1 AND organization_id=$2 AND NOT archived`, id, scope.OrganizationID).Scan(&title, &fileName, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrSOPDocumentNotFound
	}
	if err != nil {
		return nil, "", err
	}
	data, err := s.verifiedDocumentFile(fileName, hash)
	return data, title, err
}
