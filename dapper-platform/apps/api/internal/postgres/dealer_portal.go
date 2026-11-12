package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The dealership portal (error404 #26–#32, owner decisions 2026-11-11): dealership staff sign in with the email and password the owner
// gives them, or open it from the link in an invoice email (30 days, no password). They see their invoices (paid, unpaid, past due),
// open one, download PDFs, pay by card or with a saved card, see their payments and a statement, and manage saved cards.
// Everything here is limited to the one dealership the session belongs to; drafts are never shown.

var (
	ErrPortalInput    = errors.New("dealership portal input not acceptable")
	ErrPortalEmail    = errors.New("that email already has a dealership portal login")
	ErrPortalSession  = errors.New("not signed in to the dealership portal")
	portalLoginLife   = 7 * 24 * time.Hour
	portalLinkMaxLife = 30 * 24 * time.Hour
)

type DealerPortalUser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// dealershipInScope checks the dealership belongs to the franchise.
func dealershipInScope(ctx context.Context, tx pgx.Tx, scope Scope, dealership string) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM dealership_accounts WHERE organization_id=$1 AND location_id=$2 AND id::text=$3)`, scope.OrganizationID, scope.LocationID, dealership).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrPortalInput
	}
	return nil
}

// DealerPortalUsers lists a dealership's portal logins (never passwords).
func (s *Store) DealerPortalUsers(ctx context.Context, scope Scope, dealership string) ([]DealerPortalUser, error) {
	if err := s.authorize(ctx, scope, "dealership.read"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,email,created_at FROM dealer_portal_users WHERE organization_id=$1 AND location_id=$2 AND dealership_account_id::text=$3 ORDER BY name`, scope.OrganizationID, scope.LocationID, dealership)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DealerPortalUser{}
	for rows.Next() {
		var u DealerPortalUser
		if err = rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func portalEmail(email string) (string, bool) {
	e, ok := normalizedEmail(email)
	return e, ok
}

// CreateDealerPortalUser gives a dealership contact a portal login.
func (s *Store) CreateDealerPortalUser(ctx context.Context, scope Scope, dealership, name, email, password string) (DealerPortalUser, error) {
	name = strings.TrimSpace(name)
	email, ok := portalEmail(email)
	if !ok || len([]rune(name)) < 2 || len([]rune(name)) > 120 || len(password) < 10 || len(password) > 200 {
		return DealerPortalUser{}, ErrPortalInput
	}
	hash, err := shopHash(password)
	if err != nil {
		return DealerPortalUser{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DealerPortalUser{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return DealerPortalUser{}, err
	}
	if err = dealershipInScope(ctx, tx, scope, dealership); err != nil {
		return DealerPortalUser{}, err
	}
	var u DealerPortalUser
	err = tx.QueryRow(ctx, `INSERT INTO dealer_portal_users(organization_id,location_id,dealership_account_id,name,email,password_hash,created_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text,name,email,created_at`,
		scope.OrganizationID, scope.LocationID, dealership, name, email, hash, scope.ActorID).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return DealerPortalUser{}, ErrPortalEmail
	}
	if err != nil {
		return DealerPortalUser{}, err
	}
	return u, tx.Commit(ctx)
}

// SetDealerPortalPassword replaces a portal login's password and signs it out everywhere.
func (s *Store) SetDealerPortalPassword(ctx context.Context, scope Scope, userID, password string) error {
	if len(password) < 10 || len(password) > 200 {
		return ErrPortalInput
	}
	hash, err := shopHash(password)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE dealer_portal_users SET password_hash=$4,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id::text=$3`, scope.OrganizationID, scope.LocationID, userID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrPortalInput
	}
	if _, err = tx.Exec(ctx, `UPDATE dealer_portal_sessions SET revoked_at=now() WHERE user_id::text=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteDealerPortalUser removes a portal login (its sessions go with it).
func (s *Store) DeleteDealerPortalUser(ctx context.Context, scope Scope, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "dealership.manage"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM dealer_portal_users WHERE organization_id=$1 AND location_id=$2 AND id::text=$3`, scope.OrganizationID, scope.LocationID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) newPortalSession(ctx context.Context, dealership string, user *string, life time.Duration) (string, error) {
	token, digest, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO dealer_portal_sessions(token_hash,dealership_account_id,user_id,expires_at) VALUES($1,$2,$3,now()+make_interval(secs=>$4))`, digest, dealership, user, life.Seconds())
	return token, err
}

// DealerPortalSignIn checks a portal email and password and opens a 7-day session.
func (s *Store) DealerPortalSignIn(ctx context.Context, email, password string) (string, error) {
	email, _ = portalEmail(email)
	if len(password) > 200 {
		return "", ErrPortalSession
	}
	var id, dealership, hash string
	err := s.pool.QueryRow(ctx, `SELECT u.id::text,u.dealership_account_id::text,u.password_hash FROM dealer_portal_users u JOIN dealership_accounts d ON d.id=u.dealership_account_id AND d.active WHERE u.email=$1`, email).Scan(&id, &dealership, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	found := err == nil
	if !found {
		if shopDummyHash == "" {
			shopDummyHash, _ = shopHash("dapper-shop-dummy-password")
		}
		hash = shopDummyHash
	}
	shopHashSlots <- struct{}{}
	match, cerr := argon2id.ComparePasswordAndHash(password, hash)
	<-shopHashSlots
	if cerr != nil || !match || !found {
		return "", ErrPortalSession
	}
	return s.newPortalSession(ctx, dealership, &id, portalLoginLife)
}

// DealerPortalFromLink opens the dealership's portal from the link in an invoice email, for as long as the link lasts (at most 30 days),
// and returns the invoice it was sent for.
func (s *Store) DealerPortalFromLink(ctx context.Context, linkToken string) (session, invoiceID string, err error) {
	if len(linkToken) < 40 || len(linkToken) > 64 {
		return "", "", ErrPayLink
	}
	var dealership string
	var left time.Duration
	var secs float64
	err = s.pool.QueryRow(ctx, `SELECT i.dealership_account_id::text,i.id::text,EXTRACT(EPOCH FROM (l.expires_at-now()))::float8 FROM invoice_pay_links l JOIN invoices i ON i.id=l.invoice_id
 JOIN dealership_accounts d ON d.id=i.dealership_account_id AND d.active
 WHERE l.token_hash=$1 AND l.expires_at>now() AND i.status IN ('issued','partially_paid','paid')`, payLinkHash(linkToken)).Scan(&dealership, &invoiceID, &secs)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrPayLink
	}
	if err != nil {
		return "", "", err
	}
	left = min(time.Duration(secs*float64(time.Second)), portalLinkMaxLife)
	if left < time.Minute {
		return "", "", ErrPayLink
	}
	session, err = s.newPortalSession(ctx, dealership, nil, left)
	return session, invoiceID, err
}

func (s *Store) DealerPortalSignOut(ctx context.Context, token string) error {
	digest, err := tokenDigest(token)
	if err != nil {
		return nil
	}
	_, err = s.pool.Exec(ctx, `UPDATE dealer_portal_sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, digest)
	return err
}

// PortalWho is the dealership a portal session belongs to, and the franchise's website scope used to read and take payment for its invoices.
type PortalWho struct {
	DealershipID   string `json:"dealershipId"`
	DealershipName string `json:"dealershipName"`
	ContactEmail   string `json:"-"`
	Address        string `json:"-"`
	UserName       string `json:"userName"`
	ViaLink        bool   `json:"viaLink"`
	BusinessName   string `json:"businessName"`
	Scope          Scope  `json:"-"`
}

func (s *Store) DealerPortalWho(ctx context.Context, token string) (PortalWho, error) {
	digest, err := tokenDigest(token)
	if err != nil {
		return PortalWho{}, ErrPortalSession
	}
	var w PortalWho
	var org, loc string
	var user *string
	err = s.pool.QueryRow(ctx, `SELECT d.id::text,d.account_name,btrim(d.contact_email),d.address,u.name,d.organization_id::text,d.location_id::text,l.name FROM dealer_portal_sessions ps
 JOIN dealership_accounts d ON d.id=ps.dealership_account_id AND d.active JOIN locations l ON l.organization_id=d.organization_id AND l.id=d.location_id
 LEFT JOIN dealer_portal_users u ON u.id=ps.user_id
 WHERE ps.token_hash=$1 AND ps.revoked_at IS NULL AND ps.expires_at>now() AND (ps.user_id IS NULL OR u.id IS NOT NULL)`, digest).Scan(&w.DealershipID, &w.DealershipName, &w.ContactEmail, &w.Address, &user, &org, &loc, &w.BusinessName)
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalWho{}, ErrPortalSession
	}
	if err != nil {
		return PortalWho{}, err
	}
	w.ViaLink = user == nil
	if user != nil {
		w.UserName = *user
	}
	if w.Scope, err = s.websiteScopeAt(ctx, org, loc); err != nil {
		return PortalWho{}, err
	}
	return w, nil
}

// PortalInvoices are the dealership's invoices (drafts left out), newest first.
func (s *Store) PortalInvoices(ctx context.Context, w PortalWho) ([]DealerInvoiceSummary, error) {
	all, err := s.ListDealerInvoices(ctx, w.Scope, DealerInvoiceFilter{DealershipID: w.DealershipID})
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, x := range all {
		if x.Status != "draft" {
			out = append(out, x)
		}
	}
	return out, nil
}

// DealerPortalInvoice is one of the dealership's invoices (never a draft or another dealership's).
func (s *Store) DealerPortalInvoice(ctx context.Context, w PortalWho, id string) (DealerInvoice, error) {
	inv, err := s.GetDealerInvoice(ctx, w.Scope, id)
	if err != nil {
		return DealerInvoice{}, err
	}
	if inv.DealershipID != w.DealershipID || inv.Status == "draft" {
		return DealerInvoice{}, ErrInvoiceNotFound
	}
	// The first time the dealership opens it, it is "Viewed" (error404 #44).
	if inv.ViewedAt == nil {
		now := time.Now()
		if _, err = s.pool.Exec(ctx, `UPDATE invoices SET viewed_at=now() WHERE id=$1 AND viewed_at IS NULL`, inv.ID); err == nil {
			inv.ViewedAt = &now
		}
	}
	return inv, nil
}

type PortalPayment struct {
	ID            string    `json:"id"`
	InvoiceID     string    `json:"invoiceId"`
	InvoiceNumber string    `json:"invoiceNumber"`
	AmountCents   int64     `json:"amountCents"`
	Method        string    `json:"method"`
	Status        string    `json:"status"`
	RecordedAt    time.Time `json:"recordedAt"`
}

// PortalPayments are the payments on the dealership's invoices, newest first.
func (s *Store) PortalPayments(ctx context.Context, w PortalWho) ([]PortalPayment, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id::text,i.id::text,i.invoice_number,p.amount_cents,p.method,p.status,p.recorded_at FROM payments p JOIN invoices i ON i.id=p.invoice_id
 WHERE i.dealership_account_id::text=$1 AND i.status<>'draft' AND p.status IN ('completed','refunded') ORDER BY p.recorded_at DESC LIMIT 1000`, w.DealershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PortalPayment{}
	for rows.Next() {
		var p PortalPayment
		if err = rows.Scan(&p.ID, &p.InvoiceID, &p.InvoiceNumber, &p.AmountCents, &p.Method, &p.Status, &p.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type SavedCardView struct {
	ID        string `json:"id"`
	Brand     string `json:"brand"`
	Last4     string `json:"last4"`
	ExpMonth  int    `json:"expMonth"`
	ExpYear   int    `json:"expYear"`
	IsDefault bool   `json:"isDefault"`
	CreatedAt string `json:"createdAt"`
}

// PortalCards are the dealership's saved cards for the franchise's current Square account.
func (s *Store) PortalCards(ctx context.Context, w PortalWho, environment string) ([]SavedCardView, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,brand,last4,coalesce(exp_month,0),coalesce(exp_year,0),is_default,to_char(created_at,'YYYY-MM-DD') FROM dealer_saved_cards
 WHERE dealership_account_id::text=$1 AND environment=$2 AND removed_at IS NULL ORDER BY is_default DESC, created_at DESC`, w.DealershipID, environment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedCardView{}
	for rows.Next() {
		var c SavedCardView
		if err = rows.Scan(&c.ID, &c.Brand, &c.Last4, &c.ExpMonth, &c.ExpYear, &c.IsDefault, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PortalSquareCustomer is the dealership's Square customer id for an environment, or "" before the first card is saved.
func (s *Store) PortalSquareCustomer(ctx context.Context, w PortalWho, environment string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT square_customer_id FROM dealer_square_customers WHERE dealership_account_id::text=$1 AND environment=$2`, w.DealershipID, environment).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s *Store) SavePortalSquareCustomer(ctx context.Context, w PortalWho, environment, customerID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO dealer_square_customers(dealership_account_id,environment,square_customer_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, w.DealershipID, environment, customerID)
	return err
}

// AddPortalCard records a card Square saved; the first card becomes the default.
func (s *Store) AddPortalCard(ctx context.Context, w PortalWho, environment, customerID string, c SavedCardView, squareCardID string) (SavedCardView, error) {
	err := s.pool.QueryRow(ctx, `INSERT INTO dealer_saved_cards(dealership_account_id,environment,square_customer_id,square_card_id,brand,last4,exp_month,exp_year,is_default)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOT EXISTS(SELECT 1 FROM dealer_saved_cards WHERE dealership_account_id=$1 AND environment=$2 AND removed_at IS NULL AND is_default))
 RETURNING id::text,is_default,to_char(created_at,'YYYY-MM-DD')`, w.DealershipID, environment, customerID, squareCardID, c.Brand, c.Last4, c.ExpMonth, c.ExpYear).Scan(&c.ID, &c.IsDefault, &c.CreatedAt)
	return c, err
}

// PortalCardSource is the Square card and customer behind one of the dealership's saved cards.
func (s *Store) PortalCardSource(ctx context.Context, w PortalWho, cardID string) (squareCard, customer, environment string, err error) {
	err = s.pool.QueryRow(ctx, `SELECT square_card_id,square_customer_id,environment FROM dealer_saved_cards WHERE id::text=$1 AND dealership_account_id::text=$2 AND removed_at IS NULL`, cardID, w.DealershipID).Scan(&squareCard, &customer, &environment)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrPortalInput
	}
	return
}

// SetPortalDefaultCard makes one saved card the default.
func (s *Store) SetPortalDefaultCard(ctx context.Context, w PortalWho, cardID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var env string
	if err = tx.QueryRow(ctx, `SELECT environment FROM dealer_saved_cards WHERE id::text=$1 AND dealership_account_id::text=$2 AND removed_at IS NULL`, cardID, w.DealershipID).Scan(&env); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPortalInput
		}
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE dealer_saved_cards SET is_default=false WHERE dealership_account_id::text=$1 AND is_default`, w.DealershipID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE dealer_saved_cards SET is_default=true WHERE id::text=$1`, cardID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemovePortalCard forgets a saved card here (the caller also disables it at Square).
func (s *Store) RemovePortalCard(ctx context.Context, w PortalWho, cardID string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE dealer_saved_cards SET removed_at=now(),is_default=false WHERE id::text=$1 AND dealership_account_id::text=$2 AND removed_at IS NULL`, cardID, w.DealershipID)
	if err == nil && tag.RowsAffected() != 1 {
		err = ErrPortalInput
	}
	return err
}
