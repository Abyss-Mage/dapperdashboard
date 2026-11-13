package postgres

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrEnquiryInput    = errors.New("enter your name and an email address or phone number")
	ErrEnquiryNotFound = errors.New("enquiry not found")
)

type WebsiteEnquiryInput struct {
	Name, Email, Phone, Vehicle, Service, Message string
}

type WebsiteEnquiry struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Phone     string     `json:"phone"`
	Vehicle   string     `json:"vehicle"`
	Service   string     `json:"service"`
	Message   string     `json:"message"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"createdAt"`
	HandledAt *time.Time `json:"handledAt,omitempty"`
}

// CreateWebsiteEnquiry saves a contact-form message for the organization that runs the public website (this installation
// serves one: the organization with the most franchises). It returns the new enquiry's id.
func (s *Store) CreateWebsiteEnquiry(ctx context.Context, in WebsiteEnquiryInput) (string, error) {
	in.Name, in.Email, in.Phone = strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone)
	in.Vehicle, in.Service, in.Message = strings.TrimSpace(in.Vehicle), strings.TrimSpace(in.Service), strings.TrimSpace(in.Message)
	if n := utf8.RuneCountInString(in.Name); n < 2 || n > 100 || (in.Email == "" && in.Phone == "") {
		return "", ErrEnquiryInput
	}
	if in.Email != "" {
		if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email || len(in.Email) > 254 {
			return "", ErrEnquiryInput
		}
	}
	if utf8.RuneCountInString(in.Phone) > 40 || utf8.RuneCountInString(in.Vehicle) > 120 || utf8.RuneCountInString(in.Service) > 80 || utf8.RuneCountInString(in.Message) > 3000 {
		return "", ErrEnquiryInput
	}
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO website_enquiries(organization_id,name,email,phone,vehicle,service,message)
 SELECT o.id,$1,$2,$3,$4,$5,$6 FROM organizations o ORDER BY (SELECT count(*) FROM locations l WHERE l.organization_id=o.id) DESC, o.id LIMIT 1 RETURNING id::text`,
		in.Name, in.Email, in.Phone, in.Vehicle, in.Service, in.Message).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrEnquiryInput
	}
	return id, err
}

func (s *Store) enquiryAccess(ctx context.Context, tx pgx.Tx, actor, org, action string) error {
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, action)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// WebsiteEnquiries lists enquiries (newest first) for organization administrators with marketing.read.
// status is "new", "read", "archived" or "" for every one that is not archived.
func (s *Store) WebsiteEnquiries(ctx context.Context, actor, org, status string) ([]WebsiteEnquiry, int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	if err = s.enquiryAccess(ctx, tx, actor, org, "marketing.read"); err != nil {
		return nil, 0, err
	}
	var fresh int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM website_enquiries WHERE organization_id=$1 AND status='new'`, org).Scan(&fresh); err != nil {
		return nil, 0, err
	}
	filter, args := `status<>'archived'`, []any{org}
	if status == "new" || status == "read" || status == "archived" {
		filter, args = `status=$2`, append(args, status)
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,email,phone,vehicle,service,message,status,created_at,handled_at FROM website_enquiries WHERE organization_id=$1 AND `+filter+` ORDER BY created_at DESC LIMIT 300`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []WebsiteEnquiry{}
	for rows.Next() {
		var e WebsiteEnquiry
		if err = rows.Scan(&e.ID, &e.Name, &e.Email, &e.Phone, &e.Vehicle, &e.Service, &e.Message, &e.Status, &e.CreatedAt, &e.HandledAt); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, fresh, rows.Err()
}

// SetWebsiteEnquiryStatus marks an enquiry new, read or archived. It needs marketing.manage.
func (s *Store) SetWebsiteEnquiryStatus(ctx context.Context, actor, org, id, status string) error {
	if status != "new" && status != "read" && status != "archived" {
		return ErrEnquiryInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.enquiryAccess(ctx, tx, actor, org, "marketing.manage"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE website_enquiries SET status=$3,handled_by=$4,handled_at=now() WHERE organization_id=$1 AND id=$2`, org, id, status, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrEnquiryNotFound
	}
	return tx.Commit(ctx)
}

// DeleteWebsiteEnquiry erases an archived enquiry for good (error404 #108); only archived ones, so a message is never lost by one click.
func (s *Store) DeleteWebsiteEnquiry(ctx context.Context, actor, org, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.enquiryAccess(ctx, tx, actor, org, "marketing.manage"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM website_enquiries WHERE organization_id=$1 AND id=$2 AND status='archived'`, org, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrEnquiryNotFound
	}
	return tx.Commit(ctx)
}
