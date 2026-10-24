package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrDocumentUnavailable = errors.New("the document file is not available or does not match its registered checksum")
	ErrDocumentSignature   = errors.New("type your full name to acknowledge")
	ErrDocumentNotAck      = errors.New("this document has not been acknowledged")
)

// employeeAckTrack is the training track used when an employee acknowledges a document themselves.
const employeeAckTrack = "orientation"

// SetSOPDirectory installs the directory holding the registered document files (originals, never modified).
func (s *Store) SetSOPDirectory(dir string) { s.sopDir = dir }

type EmployeeDocument struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Category       string     `json:"category"`
	Version        int        `json:"version"`
	PageCount      int        `json:"pageCount"`
	EffectiveDate  string     `json:"effectiveDate"`
	Sha256         string     `json:"sha256"`
	FileAvailable  bool       `json:"fileAvailable"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

// verifiedDocumentFile reads the registered file and proves it still matches the registered checksum.
func (s *Store) verifiedDocumentFile(fileName, wantHash string) ([]byte, error) {
	if s.sopDir == "" || fileName != filepath.Base(fileName) || strings.ContainsAny(fileName, `/\`) {
		return nil, ErrDocumentUnavailable
	}
	data, err := os.ReadFile(filepath.Join(s.sopDir, fileName))
	if err != nil {
		return nil, ErrDocumentUnavailable
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), wantHash) {
		return nil, ErrDocumentUnavailable
	}
	return data, nil
}

type docRow struct {
	EmployeeDocument
	FileName string
}

func (s *Store) employeeDocumentRows(ctx context.Context, tx pgx.Tx, a EmployeeWorksite, only string) ([]docRow, error) {
	rows, err := tx.Query(ctx, `SELECT d.id::text,d.title,d.category,d.file_name,d.sha256_hash,d.version,d.page_count,d.effective_date::text,k.acknowledged_at
FROM sop_documents d LEFT JOIN sop_acknowledgments k ON k.sop_document_id=d.id AND k.employee_id=$2 AND k.location_id=$3 AND k.training_track=$4
WHERE d.organization_id=$1 AND NOT d.archived AND ($5='' OR d.id::text=$5)
ORDER BY (d.category='onboarding') DESC,d.title LIMIT 50`, a.OrganizationID, a.PrincipalID, a.LocationID, employeeAckTrack, only)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []docRow{}
	for rows.Next() {
		var r docRow
		if err = rows.Scan(&r.ID, &r.Title, &r.Category, &r.FileName, &r.Sha256, &r.Version, &r.PageCount, &r.EffectiveDate, &r.AcknowledgedAt); err != nil {
			return nil, err
		}
		_, ferr := s.verifiedDocumentFile(r.FileName, r.Sha256)
		r.FileAvailable = ferr == nil
		out = append(out, r)
	}
	return out, rows.Err()
}

// EmployeeDocuments lists the organisation's published documents with this employee's acknowledgment state.
func (s *Store) EmployeeDocuments(ctx context.Context, actor string) ([]EmployeeDocument, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	rows, err := s.employeeDocumentRows(ctx, tx, a, "")
	if err != nil {
		return nil, err
	}
	out := make([]EmployeeDocument, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.EmployeeDocument)
	}
	return out, nil
}

// EmployeeDocumentFile returns the original PDF bytes, only if they still match the registered checksum.
func (s *Store) EmployeeDocumentFile(ctx context.Context, actor, id string) ([]byte, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, "", err
	}
	rows, err := s.employeeDocumentRows(ctx, tx, a, id)
	if err != nil {
		return nil, "", err
	}
	if len(rows) != 1 {
		return nil, "", ErrSOPDocumentNotFound
	}
	data, err := s.verifiedDocumentFile(rows[0].FileName, rows[0].Sha256)
	return data, rows[0].FileName, err
}

// EmployeeAcknowledgeDocument records the employee's own electronic acknowledgment of the current registered
// revision. The first acknowledgment stands; repeating returns it unchanged. The file must be verifiable.
func (s *Store) EmployeeAcknowledgeDocument(ctx context.Context, actor, id, signedName string) (EmployeeDocument, error) {
	signedName = strings.Join(strings.Fields(signedName), " ")
	if n := utf8.RuneCountInString(signedName); n < 2 || n > 120 {
		return EmployeeDocument{}, ErrDocumentSignature
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EmployeeDocument{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return EmployeeDocument{}, err
	}
	rows, err := s.employeeDocumentRows(ctx, tx, a, id)
	if err != nil {
		return EmployeeDocument{}, err
	}
	if len(rows) != 1 {
		return EmployeeDocument{}, ErrSOPDocumentNotFound
	}
	if !rows[0].FileAvailable {
		return EmployeeDocument{}, ErrDocumentUnavailable
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sop_acknowledgments(organization_id,location_id,sop_document_id,employee_id,actor_id,acknowledged_sha256,training_track,notes) VALUES($1,$2,$3,$4,$4,$5,$6,$7) ON CONFLICT (organization_id,location_id,sop_document_id,employee_id,training_track) DO NOTHING`,
		a.OrganizationID, a.LocationID, id, actor, rows[0].Sha256, employeeAckTrack, "Electronic acknowledgment by employee; typed name: "+signedName); err != nil {
		return EmployeeDocument{}, err
	}
	if err = employeeEvent(ctx, tx, actor, a.OrganizationID, id, "document.acknowledged", map[string]any{"version": rows[0].Version}); err != nil {
		return EmployeeDocument{}, err
	}
	after, err := s.employeeDocumentRows(ctx, tx, a, id)
	if err != nil || len(after) != 1 {
		return EmployeeDocument{}, err
	}
	return after[0].EmployeeDocument, tx.Commit(ctx)
}

// EmployeeDocumentReceipt builds a one-page PDF receipt for an acknowledgment the employee made.
func (s *Store) EmployeeDocumentReceipt(ctx context.Context, actor, id string) ([]byte, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return nil, "", err
	}
	var title, hash, notes, ackID, email string
	var version int
	var at time.Time
	err = tx.QueryRow(ctx, `SELECT d.title,k.acknowledged_sha256,k.notes,k.id::text,d.version,k.acknowledged_at,COALESCE(NULLIF(c_p.email,''),p.external_subject)
FROM sop_acknowledgments k JOIN sop_documents d ON d.id=k.sop_document_id JOIN principals p ON p.id=k.employee_id LEFT JOIN password_credentials c_p ON c_p.principal_id=p.id
WHERE k.sop_document_id=$1 AND k.employee_id=$2 AND k.organization_id=$3 AND k.location_id=$4 AND k.training_track=$5`, id, actor, a.OrganizationID, a.LocationID, employeeAckTrack).Scan(&title, &hash, &notes, &ackID, &version, &at, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrDocumentNotAck
	}
	if err != nil {
		return nil, "", err
	}
	signed := strings.TrimPrefix(notes, "Electronic acknowledgment by employee; typed name: ")
	zone, zerr := time.LoadLocation(a.Timezone)
	if zerr != nil {
		zone = time.UTC
	}
	pdf := buildReceiptPDF([]string{
		"Dapper Detailing Pros",
		"Policy acknowledgment receipt",
		"",
		"Document: " + title + " (version " + fmt.Sprint(version) + ")",
		"Acknowledged by: " + signed,
		"Account: " + email,
		"Acknowledged at: " + at.In(zone).Format("2006-01-02 15:04:05 MST"),
		worksiteLine(a),
		"",
		"Document SHA-256:",
		hash,
		"Record ID: " + ackID,
		"",
		"This receipt records an electronic acknowledgment made while signed in.",
		"It is not a handwritten signature and has not been reviewed as a legal e-signature.",
	})
	return pdf, "policy-receipt-" + ackID[:8] + ".pdf", nil
}

// buildReceiptPDF writes a minimal single-page PDF using the built-in Helvetica font. Characters outside printable
// ASCII are shown as '?' because no font is embedded.
func buildReceiptPDF(lines []string) []byte {
	escape := func(v string) string {
		var b strings.Builder
		for _, r := range v {
			switch {
			case r == '\\' || r == '(' || r == ')':
				b.WriteByte('\\')
				b.WriteRune(r)
			case r >= 32 && r < 127:
				b.WriteRune(r)
			default:
				b.WriteByte('?')
			}
		}
		return b.String()
	}
	var content bytes.Buffer
	content.WriteString("BT /F1 11 Tf 56 770 Td 16 TL\n")
	for i, l := range lines {
		if i == 0 {
			content.WriteString("/F1 18 Tf 22 TL\n")
		} else if i == 1 {
			content.WriteString("/F1 13 Tf 20 TL\n")
		} else if i == 2 {
			content.WriteString("/F1 11 Tf 16 TL\n")
		}
		fmt.Fprintf(&content, "(%s) Tj T*\n", escape(l))
	}
	content.WriteString("ET")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func worksiteLine(a EmployeeWorksite) string {
	if a.DealershipName != "" {
		return "Worksite: " + a.LocationName + " / " + a.DealershipName
	}
	return "Worksite: " + a.LocationName
}
