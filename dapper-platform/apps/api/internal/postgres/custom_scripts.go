package postgres

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const maxCustomScripts = 50

var (
	ErrCustomScriptInput    = errors.New("a name, a placement, code under 20,000 characters and page paths starting with / are required")
	ErrCustomScriptNotFound = errors.New("custom script not found")
	ErrCustomScriptLimit    = errors.New("the limit of 50 custom scripts has been reached")
)

// CustomScript is one snippet. Paths limit it to pages whose address starts with one of them; empty means every page.
type CustomScript struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Placement string    `json:"placement"`
	Code      string    `json:"code"`
	Paths     []string  `json:"paths"`
	Active    bool      `json:"active"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (c *CustomScript) normalize() error {
	c.Name = strings.Join(strings.Fields(c.Name), " ")
	if n := utf8.RuneCountInString(c.Name); n < 1 || n > 80 {
		return ErrCustomScriptInput
	}
	if c.Placement != "head" && c.Placement != "body" {
		return ErrCustomScriptInput
	}
	if strings.TrimSpace(c.Code) == "" || utf8.RuneCountInString(c.Code) > 20000 || strings.ContainsRune(c.Code, 0) {
		return ErrCustomScriptInput
	}
	clean := []string{}
	for _, p := range c.Paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || len(p) > 100 || strings.ContainsAny(p, " \t\r\n\"'<>\\") {
			return ErrCustomScriptInput
		}
		clean = append(clean, p)
	}
	if len(clean) > 20 {
		return ErrCustomScriptInput
	}
	c.Paths = clean
	return nil
}

// customScriptManager: running code on the public website is as powerful as the owner's own access, so it needs the
// access-management capability that only Super Admin and Owner roles carry.
func (s *Store) customScriptManager(ctx context.Context, tx pgx.Tx, actor, org string) error {
	ok, err := actorHasOrgCapability(ctx, tx, actor, org, "access.manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Store) CustomScripts(ctx context.Context, actor, org string) ([]CustomScript, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = s.customScriptManager(ctx, tx, actor, org); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,placement,code,paths,active,updated_at FROM custom_scripts WHERE organization_id=$1 ORDER BY created_at`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CustomScript{}
	for rows.Next() {
		var c CustomScript
		if err = rows.Scan(&c.ID, &c.Name, &c.Placement, &c.Code, &c.Paths, &c.Active, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveCustomScript creates a snippet (empty id) or replaces one.
func (s *Store) SaveCustomScript(ctx context.Context, actor, org string, c CustomScript) (CustomScript, error) {
	if err := c.normalize(); err != nil {
		return CustomScript{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CustomScript{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.customScriptManager(ctx, tx, actor, org); err != nil {
		return CustomScript{}, err
	}
	if c.ID == "" {
		var n int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM custom_scripts WHERE organization_id=$1`, org).Scan(&n); err != nil {
			return CustomScript{}, err
		}
		if n >= maxCustomScripts {
			return CustomScript{}, ErrCustomScriptLimit
		}
		err = tx.QueryRow(ctx, `INSERT INTO custom_scripts(organization_id,name,placement,code,paths,active,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$7) RETURNING id::text,updated_at`,
			org, c.Name, c.Placement, c.Code, c.Paths, c.Active, actor).Scan(&c.ID, &c.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `UPDATE custom_scripts SET name=$3,placement=$4,code=$5,paths=$6,active=$7,updated_by=$8,updated_at=now() WHERE id=$1 AND organization_id=$2 RETURNING updated_at`,
			c.ID, org, c.Name, c.Placement, c.Code, c.Paths, c.Active, actor).Scan(&c.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return CustomScript{}, ErrCustomScriptNotFound
		}
	}
	if err != nil {
		return CustomScript{}, err
	}
	return c, tx.Commit(ctx)
}

func (s *Store) DeleteCustomScript(ctx context.Context, actor, org, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.customScriptManager(ctx, tx, actor, org); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM custom_scripts WHERE id=$1 AND organization_id=$2`, id, org)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCustomScriptNotFound
	}
	return tx.Commit(ctx)
}

// PublicCustomScripts are the active snippets the public site loads. This installation serves one organization's website.
func (s *Store) PublicCustomScripts(ctx context.Context) ([]CustomScript, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,placement,code,paths,active,updated_at FROM custom_scripts WHERE active ORDER BY created_at LIMIT $1`, maxCustomScripts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CustomScript{}
	for rows.Next() {
		var c CustomScript
		if err = rows.Scan(&c.ID, &c.Name, &c.Placement, &c.Code, &c.Paths, &c.Active, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
