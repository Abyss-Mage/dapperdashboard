package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Invoice Ninja (v5 API) for dealership orders: the owner saves the Invoice Ninja address and an API token, then sends a
// dealership's invoice there. Dapper creates (or reuses) the dealership as an Invoice Ninja client and creates the invoice
// with the same lines. Each invoice is sent once.

var ErrInvoiceNinjaInput = errors.New("an https Invoice Ninja address and an API token are required")
var ErrInvoiceNinjaNotSet = errors.New("Invoice Ninja is not connected")
var ErrInvoiceNinjaRejected = errors.New("Invoice Ninja did not accept the request")
var ErrInvoiceNinjaNotDealership = errors.New("only invoices of dealership customers are sent to Invoice Ninja")
var ErrInvoiceNinjaAlreadySent = errors.New("this invoice was already sent to Invoice Ninja")

type InvoiceNinjaStatus struct {
	Connected bool      `json:"connected"`
	BaseURL   string    `json:"baseUrl"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func ninjaSealContext(org string) string { return "invoice-ninja/" + org }

func (s *Store) InvoiceNinjaSettings(ctx context.Context, actor, org string) (InvoiceNinjaStatus, error) {
	var out InvoiceNinjaStatus
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT base_url,version,updated_at FROM invoice_ninja_settings WHERE organization_id=$1`, org).Scan(&out.BaseURL, &out.Version, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	out.Connected = err == nil
	return out, err
}

// SaveInvoiceNinjaSettings checks the token against Invoice Ninja first, then stores it encrypted.
func (s *Store) SaveInvoiceNinjaSettings(ctx context.Context, actor, org, baseURL, token string, version int) (InvoiceNinjaStatus, error) {
	baseURL, token = strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(token)
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(baseURL) > 200 || token == "" || len(token) > 500 || len(s.mfaKey) != 32 {
		return InvoiceNinjaStatus{}, ErrInvoiceNinjaInput
	}
	if _, err = ninjaCall(ctx, baseURL, token, http.MethodGet, "/api/v1/clients?per_page=1", nil); err != nil {
		return InvoiceNinjaStatus{}, err
	}
	sealed, err := s.sealSecret(ninjaSealContext(org), []byte(token))
	if err != nil {
		return InvoiceNinjaStatus{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InvoiceNinjaStatus{}, err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return InvoiceNinjaStatus{}, err
	}
	out := InvoiceNinjaStatus{Connected: true, BaseURL: baseURL}
	if version == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO invoice_ninja_settings(organization_id,base_url,token_sealed,updated_by) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING version,updated_at`, org, baseURL, sealed, actor).Scan(&out.Version, &out.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `UPDATE invoice_ninja_settings SET base_url=$2,token_sealed=$3,updated_by=$4,updated_at=now(),version=version+1 WHERE organization_id=$1 AND version=$5 RETURNING version,updated_at`, org, baseURL, sealed, actor, version).Scan(&out.Version, &out.UpdatedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoiceNinjaStatus{}, ErrInvoiceTemplateConflict
	}
	if err != nil {
		return InvoiceNinjaStatus{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,entity_id) VALUES($1,$2,'invoice_ninja.connected',$1)`, org, actor); err != nil {
		return InvoiceNinjaStatus{}, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) DisconnectInvoiceNinja(ctx context.Context, actor, org string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = financeConfigurationGrant(ctx, tx, actor, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM invoice_ninja_settings WHERE organization_id=$1`, org); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_audit_events(organization_id,actor_id,action,entity_id) VALUES($1,$2,'invoice_ninja.disconnected',$1)`, org, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type InvoiceNinjaExport struct {
	InvoiceID      string    `json:"invoiceId"`
	NinjaInvoiceID string    `json:"ninjaInvoiceId"`
	NinjaNumber    string    `json:"ninjaNumber"`
	ExportedAt     time.Time `json:"exportedAt"`
}

// SendInvoiceToNinja sends one issued dealership invoice to Invoice Ninja (needs invoices.manage at the franchise).
func (s *Store) SendInvoiceToNinja(ctx context.Context, scope Scope, invoiceID string) (InvoiceNinjaExport, error) {
	var out InvoiceNinjaExport
	if err := s.authorize(ctx, scope, "invoices.manage"); err != nil {
		return out, err
	}
	var baseURL string
	var sealed []byte
	err := s.pool.QueryRow(ctx, `SELECT base_url,token_sealed FROM invoice_ninja_settings WHERE organization_id=$1`, scope.OrganizationID).Scan(&baseURL, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrInvoiceNinjaNotSet
	}
	if err != nil {
		return out, err
	}
	tokenBytes, err := s.openSecret(ninjaSealContext(scope.OrganizationID), sealed)
	if err != nil {
		return out, ErrInvoiceNinjaNotSet
	}
	token := string(tokenBytes)
	var already bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoice_ninja_exports WHERE invoice_id=$1)`, invoiceID).Scan(&already); err != nil {
		return out, err
	}
	if already {
		return out, ErrInvoiceNinjaAlreadySent
	}
	// The invoice, its dealership and its lines.
	var number, dealer, email, phone, currency, notes string
	var subtotal, tax int64
	err = s.pool.QueryRow(ctx, `SELECT i.invoice_number,d.account_name,d.contact_email,COALESCE(c.phone,''),i.currency,COALESCE(i.notes,''),i.subtotal_cents,i.tax_cents
 FROM invoices i JOIN customers c ON (c.organization_id,c.location_id,c.id)=(i.organization_id,i.location_id,i.customer_id)
 JOIN dealership_accounts d ON (d.organization_id,d.location_id,d.customer_id)=(i.organization_id,i.location_id,i.customer_id) AND d.account_type='dealership'
 WHERE i.organization_id=$1 AND i.location_id=$2 AND i.id=$3 AND i.status<>'draft' LIMIT 1`, scope.OrganizationID, scope.LocationID, invoiceID).Scan(&number, &dealer, &email, &phone, &currency, &notes, &subtotal, &tax)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrInvoiceNinjaNotDealership
	}
	if err != nil {
		return out, err
	}
	type line struct {
		ProductKey string  `json:"product_key"`
		Notes      string  `json:"notes"`
		Cost       float64 `json:"cost"`
		Quantity   float64 `json:"quantity"`
		TaxName1   string  `json:"tax_name1,omitempty"`
		TaxRate1   float64 `json:"tax_rate1,omitempty"`
	}
	rate := 0.0
	if subtotal > 0 && tax > 0 {
		rate = math.Round(float64(tax)/float64(subtotal)*100000) / 1000
	}
	rows, err := s.pool.Query(ctx, `SELECT description,quantity,COALESCE(unit_price_cents,total_price_cents/GREATEST(quantity,1)) FROM invoice_items WHERE invoice_id=$1 ORDER BY created_at,id`, invoiceID)
	if err != nil {
		return out, err
	}
	lines := []line{}
	for rows.Next() {
		var d string
		var q, unit int64
		if err = rows.Scan(&d, &q, &unit); err != nil {
			rows.Close()
			return out, err
		}
		l := line{ProductKey: truncate(d, 100), Notes: d, Cost: float64(unit) / 100, Quantity: float64(q)}
		if rate > 0 {
			l.TaxName1, l.TaxRate1 = "Tax", rate
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	// The dealership as an Invoice Ninja client: reuse one with the same contact email, else create it.
	clientID, err := ninjaClient(ctx, baseURL, token, dealer, email, phone)
	if err != nil {
		return out, err
	}
	body := map[string]any{"client_id": clientID, "line_items": lines, "public_notes": strings.TrimSpace("Dapper invoice " + number + "\n" + notes), "po_number": truncate(number, 60)}
	data, err := ninjaCall(ctx, baseURL, token, http.MethodPost, "/api/v1/invoices", body)
	if err != nil {
		return out, err
	}
	var created struct {
		Data struct {
			ID     string `json:"id"`
			Number string `json:"number"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &created) != nil || created.Data.ID == "" {
		return out, ErrInvoiceNinjaRejected
	}
	err = s.pool.QueryRow(ctx, `INSERT INTO invoice_ninja_exports(invoice_id,organization_id,ninja_client_id,ninja_invoice_id,ninja_number,exported_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING invoice_id::text,ninja_invoice_id,ninja_number,exported_at`,
		invoiceID, scope.OrganizationID, clientID, created.Data.ID, created.Data.Number, scope.ActorID).Scan(&out.InvoiceID, &out.NinjaInvoiceID, &out.NinjaNumber, &out.ExportedAt)
	return out, err
}

// InvoiceNinjaExports lists which invoices of a franchise were already sent.
func (s *Store) InvoiceNinjaExports(ctx context.Context, scope Scope) ([]InvoiceNinjaExport, error) {
	out := []InvoiceNinjaExport{}
	if err := s.authorize(ctx, scope, "invoices.read"); err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT e.invoice_id::text,e.ninja_invoice_id,e.ninja_number,e.exported_at FROM invoice_ninja_exports e JOIN invoices i ON i.id=e.invoice_id WHERE i.organization_id=$1 AND i.location_id=$2 ORDER BY e.exported_at DESC LIMIT 1000`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e InvoiceNinjaExport
		if err = rows.Scan(&e.InvoiceID, &e.NinjaInvoiceID, &e.NinjaNumber, &e.ExportedAt); err != nil {
			return out, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func truncate(v string, n int) string {
	r := []rune(v)
	if len(r) > n {
		return string(r[:n])
	}
	return v
}

func ninjaClient(ctx context.Context, baseURL, token, name, email, phone string) (string, error) {
	data, err := ninjaCall(ctx, baseURL, token, http.MethodGet, "/api/v1/clients?per_page=20&filter="+url.QueryEscape(email), nil)
	if err != nil {
		return "", err
	}
	var found struct {
		Data []struct {
			ID       string `json:"id"`
			Contacts []struct {
				Email string `json:"email"`
			} `json:"contacts"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &found) == nil {
		for _, c := range found.Data {
			for _, contact := range c.Contacts {
				if strings.EqualFold(contact.Email, email) {
					return c.ID, nil
				}
			}
		}
	}
	data, err = ninjaCall(ctx, baseURL, token, http.MethodPost, "/api/v1/clients", map[string]any{"name": name, "contacts": []map[string]string{{"email": email, "phone": phone}}})
	if err != nil {
		return "", err
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &created) != nil || created.Data.ID == "" {
		return "", ErrInvoiceNinjaRejected
	}
	return created.Data.ID, nil
}

func ninjaCall(ctx context.Context, baseURL, token, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return nil, ErrInvoiceNinjaInput
	}
	req.Header.Set("X-API-TOKEN", token)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: unreachable", ErrInvoiceNinjaRejected)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil || res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, ErrInvoiceNinjaRejected
	}
	return data, nil
}
