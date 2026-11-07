package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrPortalAddress = errors.New("give the address a name (1 to 40 characters) and an address of 5 to 300 characters")
var ErrPortalAddressLimit = errors.New("a customer can save up to 10 addresses")
var ErrPortalAvatar = errors.New("upload a JPEG or PNG picture of 2 MB or less")

// PortalAddress is one of a customer's saved addresses.
type PortalAddress struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Address   string `json:"address"`
	IsDefault bool   `json:"isDefault"`
}

// PortalAddresses lists the customer's saved addresses, the default first.
func (s *Store) PortalAddresses(ctx context.Context, actor, customer string) ([]PortalAddress, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return nil, err
	}
	out, err := readPortalAddresses(ctx, tx, org, loc, customer)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func readPortalAddresses(ctx context.Context, tx pgx.Tx, org, loc, customer string) ([]PortalAddress, error) {
	rows, err := tx.Query(ctx, `SELECT id::text,label,address,is_default FROM customer_addresses WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY is_default DESC,created_at,id LIMIT 10`, org, loc, customer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PortalAddress{}
	for rows.Next() {
		var a PortalAddress
		if err = rows.Scan(&a.ID, &a.Label, &a.Address, &a.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SavePortalAddress adds or changes one saved address. The customer's first address is the default; choosing a new default also becomes the
// address staff see on the customer record (used for mobile jobs).
func (s *Store) SavePortalAddress(ctx context.Context, actor, customer string, in PortalAddress) (PortalAddress, error) {
	in.Label, in.Address = strings.TrimSpace(in.Label), strings.TrimSpace(in.Address)
	if l, a := utf8.RuneCountInString(in.Label), utf8.RuneCountInString(in.Address); l < 1 || l > 40 || a < 5 || a > 300 {
		return in, ErrPortalAddress
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return in, err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return in, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM customer_addresses WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3`, org, loc, customer).Scan(&count); err != nil {
		return in, err
	}
	if in.ID == "" {
		if count >= 10 {
			return in, ErrPortalAddressLimit
		}
		if count == 0 {
			in.IsDefault = true
		}
		if in.IsDefault {
			if _, err = tx.Exec(ctx, `UPDATE customer_addresses SET is_default=false WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND is_default`, org, loc, customer); err != nil {
				return in, err
			}
		}
		err = tx.QueryRow(ctx, `INSERT INTO customer_addresses(organization_id,location_id,customer_id,label,address,is_default) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, org, loc, customer, in.Label, in.Address, in.IsDefault).Scan(&in.ID)
	} else {
		if in.IsDefault {
			if _, err = tx.Exec(ctx, `UPDATE customer_addresses SET is_default=false WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND is_default AND id<>$4`, org, loc, customer, in.ID); err != nil {
				return in, err
			}
		}
		// A default cannot be switched off here: pick another address as the default instead.
		err = tx.QueryRow(ctx, `UPDATE customer_addresses SET label=$5,address=$6,is_default=is_default OR $7 WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND id=$4 RETURNING is_default`, org, loc, customer, in.ID, in.Label, in.Address, in.IsDefault).Scan(&in.IsDefault)
		if errors.Is(err, pgx.ErrNoRows) {
			return in, ErrCustomerNotFound
		}
	}
	if err != nil {
		return in, err
	}
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE customers SET address=$4 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL`, org, loc, customer, in.Address); err != nil {
			return in, err
		}
	}
	return in, tx.Commit(ctx)
}

// DeletePortalAddress removes a saved address; if it was the default, the oldest remaining one becomes the default.
func (s *Store) DeletePortalAddress(ctx context.Context, actor, customer, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return err
	}
	var wasDefault bool
	err = tx.QueryRow(ctx, `DELETE FROM customer_addresses WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND id=$4 RETURNING is_default`, org, loc, customer, id).Scan(&wasDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCustomerNotFound
	}
	if err != nil {
		return err
	}
	if wasDefault {
		var addr string
		err = tx.QueryRow(ctx, `UPDATE customer_addresses SET is_default=true WHERE id=(SELECT id FROM customer_addresses WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 ORDER BY created_at,id LIMIT 1) RETURNING address`, org, loc, customer).Scan(&addr)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE customers SET address=$4 WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND removed_at IS NULL`, org, loc, customer, addr); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SetPortalAvatar stores the customer's profile picture (re-encoded, no metadata, at most 2 MB).
func (s *Store) SetPortalAvatar(ctx context.Context, actor, customer string, data []byte) error {
	normalized, mime, err := NormalizeContentImage(data)
	if err != nil || len(normalized) > 2*1024*1024 {
		return ErrPortalAvatar
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO customer_avatars(organization_id,location_id,customer_id,content_type,data) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(organization_id,location_id,customer_id) DO UPDATE SET content_type=excluded.content_type,data=excluded.data,updated_at=now()`, org, loc, customer, mime, normalized); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PortalAvatar returns the customer's own profile picture.
func (s *Store) PortalAvatar(ctx context.Context, actor, customer string) ([]byte, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return nil, "", err
	}
	var data []byte
	var mime string
	err = tx.QueryRow(ctx, `SELECT data,content_type FROM customer_avatars WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3`, org, loc, customer).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrCustomerNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return data, mime, tx.Commit(ctx)
}

// DeletePortalAvatar removes the picture.
func (s *Store) DeletePortalAvatar(ctx context.Context, actor, customer string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	org, loc, err := portalLink(ctx, tx, actor, customer)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM customer_avatars WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3`, org, loc, customer); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
