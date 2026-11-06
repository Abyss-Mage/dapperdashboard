package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Public (signed-out) booking for the website. The website acts as a fixed, minimal "website" principal that only exists for a
// location whose public site is published. Every figure (prices, tax, deposit) is computed here from the catalogue, never taken
// from the browser.

var (
	ErrPublicSite                   = errors.New("the website booking is not available")
	ErrPublicBookingInput           = errors.New("check the booking details")
	ErrPublicBookingPrice           = errors.New("this service has no price set, so it cannot be booked online")
	ErrPhoneVerificationUnavailable = errors.New("phone verification is not set up")
	ErrPhoneVerification            = errors.New("the code is wrong or has expired")
	ErrPhoneVerificationLimit       = errors.New("too many attempts; try again later")
)

// DepositPercent is the share of the total taken as a card deposit when an online booking is made (owner decision, 2026-10-24).
const DepositPercent = 30

// PhoneVerifier delivers a one-time code by text message. No provider is wired up yet, so without one the site offers no
// returning-customer lookup.
type PhoneVerifier interface {
	SendCode(ctx context.Context, phone, code string) error
}

func (s *Store) SetPhoneVerifier(v PhoneVerifier) { s.phoneVerifier = v }

var websiteGrants = []string{"customers.read", "customers.create", "catalogue.read", "bookings.create", "bookings.read", "bookings.manage", "scheduling.reserve", "scheduling.release", "invoices.read", "invoices.create", "invoices.issue", "invoices.manage", "payments.record", "payments.read"}

// websiteScope returns the scope the website acts under for a published public site, creating its principal and minimal grants on
// first use. Publishing the site is the owner's decision to take online bookings.
func (s *Store) websiteScope(ctx context.Context, slug string) (Scope, error) {
	if !siteSlug.MatchString(slug) {
		return Scope{}, ErrPublicSite
	}
	var org, loc string
	err := s.pool.QueryRow(ctx, `SELECT organization_id::text,location_id::text FROM public_sites WHERE slug=$1 AND published`, slug).Scan(&org, &loc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scope{}, ErrPublicSite
	}
	if err != nil {
		return Scope{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Scope{}, err
	}
	defer tx.Rollback(ctx)
	var principal string
	if err = tx.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('system:website') ON CONFLICT(external_subject) DO UPDATE SET external_subject=excluded.external_subject RETURNING id::text`).Scan(&principal); err != nil {
		return Scope{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, org, principal); err != nil {
		return Scope{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) SELECT $1,$2,$3,a FROM unnest($4::text[]) a ON CONFLICT DO NOTHING`, principal, org, loc, websiteGrants); err != nil {
		return Scope{}, err
	}
	return Scope{ActorID: principal, OrganizationID: org, LocationID: loc}, tx.Commit(ctx)
}

type PublicService struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	ParentID   *string `json:"parentId"`
	PriceCents int64   `json:"priceCents"`
}
type PublicBookingOptions struct {
	Timezone          string                  `json:"timezone"`
	DepositPercent    int                     `json:"depositPercent"`
	PhoneVerification bool                    `json:"phoneVerification"`
	Rules             []BookingRuleChoice     `json:"rules"`
	Services          []PublicService         `json:"services"`
	Taxes             FranchiseTaxes          `json:"taxes"`
	Resources         []BookingResourceChoice `json:"resources"`
}

// PublicBookingOptions lists what can be booked online: base services that have a price, their priced add-ons and the franchise taxes.
func (s *Store) PublicBookingOptions(ctx context.Context, slug string) (PublicBookingOptions, error) {
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return PublicBookingOptions{}, err
	}
	choices, err := s.BookingCapacityChoices(ctx, scope)
	if err != nil {
		return PublicBookingOptions{}, err
	}
	all, err := s.Services(ctx, scope)
	if err != nil {
		return PublicBookingOptions{}, err
	}
	priced := map[string]PublicService{}
	for _, v := range all {
		if !v.Archived && v.PriceCents != nil {
			priced[v.ID] = PublicService{ID: v.ID, Name: v.Name, Kind: v.Kind, ParentID: v.ParentID, PriceCents: *v.PriceCents}
		}
	}
	out := PublicBookingOptions{DepositPercent: DepositPercent, PhoneVerification: s.phoneVerifier != nil, Rules: []BookingRuleChoice{}, Services: []PublicService{}, Resources: choices.Resources}
	if len(choices.Resources) > 0 {
		out.Timezone = choices.Resources[0].Timezone
	}
	keep := map[string]bool{}
	for _, r := range choices.Rules {
		if len(r.Addons) > 0 {
			continue
		}
		if _, ok := priced[r.ServiceID]; !ok {
			continue
		}
		extras := []JobAddon{}
		for _, a := range r.AvailableAddons {
			if _, ok := priced[a.ServiceID]; ok {
				extras = append(extras, a)
				keep[a.ServiceID] = true
			}
		}
		r.AvailableAddons = extras
		keep[r.ServiceID] = true
		out.Rules = append(out.Rules, r)
	}
	for id := range keep {
		out.Services = append(out.Services, priced[id])
	}
	out.Taxes, err = s.franchiseTaxesUnchecked(ctx, scope)
	return out, err
}

// PublicAvailability returns bookable days and times for one service over up to 31 days (the same check a booking save uses).
func (s *Store) PublicAvailability(ctx context.Context, slug, resourceID, ruleID string, ruleVersion int, firstDay string, days int) (RuleAvailability, error) {
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return RuleAvailability{}, err
	}
	return s.RuleAvailability(ctx, scope, resourceID, ruleID, ruleVersion, firstDay, days)
}

type PublicBookingInput struct {
	RequestKey  string    `json:"requestKey"`
	ResourceID  string    `json:"resourceId"`
	RuleID      string    `json:"ruleId"`
	RuleVersion int       `json:"ruleVersion"`
	Start       time.Time `json:"start"`
	AddonIDs    []string  `json:"addonIds"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Notes       string    `json:"notes"`
	// ServiceAddress is where mobile or valet work happens (empty for in-shop work).
	ServiceAddress string `json:"serviceAddress"`
	// CouponCode is an optional discount code; an invalid one stops the booking before anything is held.
	CouponCode string `json:"couponCode"`
}
type PublicBookingResult struct {
	BookingID     string `json:"bookingId"`
	InvoiceID     string `json:"invoiceId"`
	InvoiceNumber string `json:"invoiceNumber"`
	TotalCents    int64  `json:"totalCents"`
	DepositCents  int64  `json:"depositCents"`
	ServiceName   string `json:"serviceName"`
}

func phoneDigits(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// PublicBook holds the time, creates (or reuses) the customer and issues an invoice for the full price. The deposit itself is
// charged by the caller afterwards; if it fails the caller must call PublicBookingAbort so the time is released.
func (s *Store) PublicBook(ctx context.Context, slug string, in PublicBookingInput) (PublicBookingResult, error) {
	var out PublicBookingResult
	in.Name, in.Email, in.Phone, in.Notes = strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Notes)
	addr, aerr := mail.ParseAddress(in.Email)
	digits := phoneDigits(in.Phone)
	now := time.Now()
	if utf8.RuneCountInString(in.Name) < 2 || utf8.RuneCountInString(in.Name) > 100 || aerr != nil || addr.Address != in.Email || len(in.Email) > 254 || len(digits) < 7 || len(digits) > 15 || len(in.Phone) > 30 ||
		utf8.RuneCountInString(in.Notes) > 1500 || len(in.RequestKey) < 16 || len(in.RequestKey) > 100 || in.RuleVersion < 1 || len(in.AddonIDs) > 20 ||
		!in.Start.After(now.Add(time.Hour)) || in.Start.After(now.AddDate(1, 0, 0)) {
		return out, ErrPublicBookingInput
	}
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return out, err
	}
	serviceID, serviceName, items, subtotal, err := s.priceBooking(ctx, scope, in.RuleID, in.ResourceID, in.RuleVersion, in.AddonIDs)
	if err != nil {
		return out, err
	}
	tax, _, err := s.taxOnSubtotal(ctx, scope, subtotal)
	if err != nil {
		return out, err
	}
	// The customer: reuse the one with this email at this location (their saved details are not changed by a stranger), else add one.
	customerID, err := existingCustomer(ctx, s.pool, scope.OrganizationID, scope.LocationID, in.Email, in.Phone)
	if err == nil && customerID == "" {
		c, e := s.CreateCustomer(ctx, scope, in.Name, in.Email, in.Phone)
		if e != nil {
			return out, e
		}
		customerID = c.ID
	} else if err != nil {
		return out, err
	}
	if strings.TrimSpace(in.CouponCode) != "" {
		if _, err = s.evaluateCoupon(ctx, s.pool, scope, in.CouponCode, customerID, &serviceID, subtotal); err != nil {
			return out, err
		}
	}
	booking, err := s.ReserveBooking(ctx, scope, ReserveBookingInput{RequestKey: in.RequestKey, CustomerID: customerID, ResourceID: in.ResourceID, RuleID: in.RuleID, RuleVersion: in.RuleVersion, Start: in.Start, Channel: "online", Notes: in.Notes, ServiceAddress: in.ServiceAddress, AddonIDs: in.AddonIDs})
	if err != nil {
		return out, err
	}
	release := func() {
		_, _ = s.UpdateBookingStatus(context.WithoutCancel(ctx), scope, booking.ID, "cancelled", "The online booking could not be completed", "")
	}
	bookingID := booking.ID
	// The same request sent twice (a double click, a slow network): hand back the invoice that already exists for this booking.
	var invID, invNumber string
	var invTotal int64
	err = s.pool.QueryRow(ctx, `SELECT id::text,invoice_number,total_cents FROM invoices WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 AND status<>'void' ORDER BY created_at LIMIT 1`, scope.OrganizationID, scope.LocationID, bookingID).Scan(&invID, &invNumber, &invTotal)
	if err == nil {
		return PublicBookingResult{BookingID: bookingID, InvoiceID: invID, InvoiceNumber: invNumber, TotalCents: invTotal, DepositCents: (invTotal*DepositPercent + 50) / 100, ServiceName: serviceName}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	inv, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: customerID, BookingID: &bookingID, TaxCents: tax, Notes: "Booked on the website", Items: items})
	if err != nil {
		release()
		return out, err
	}
	if strings.TrimSpace(in.CouponCode) != "" {
		if _, err = s.ApplyCouponToInvoice(ctx, scope, inv.ID, in.CouponCode); err != nil {
			_, _ = s.VoidInvoice(context.WithoutCancel(ctx), scope, inv.ID, "The coupon could not be applied")
			release()
			return out, err
		}
	}
	issued, err := s.IssueInvoice(ctx, scope, inv.ID)
	if err != nil {
		_, _ = s.VoidInvoice(context.WithoutCancel(ctx), scope, inv.ID, "The online booking could not be completed")
		release()
		return out, err
	}
	out = PublicBookingResult{BookingID: bookingID, InvoiceID: issued.ID, InvoiceNumber: issued.InvoiceNumber, TotalCents: issued.TotalCents, DepositCents: (issued.TotalCents*DepositPercent + 50) / 100, ServiceName: serviceName}
	return out, nil
}

// PublicCouponPreview tells a visitor what a code would take off, before they book. The booking itself works the price out again on the server.
func (s *Store) PublicCouponPreview(ctx context.Context, slug, code, mainServiceID string, subtotal int64) (CouponApplication, error) {
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return CouponApplication{}, err
	}
	return s.evaluateCoupon(ctx, s.pool, scope, code, "", &mainServiceID, subtotal)
}

// priceBooking prices a rule's main service and each chosen add-on from the catalogue (never from what the browser says).
func (s *Store) priceBooking(ctx context.Context, scope Scope, ruleID, resourceID string, ruleVersion int, addonIDs []string) (serviceID, serviceName string, items []CreateInvoiceItemInput, subtotal int64, err error) {
	var mainPrice *int64
	err = s.pool.QueryRow(ctx, `SELECT c.id::text,c.name,c.price_cents FROM service_scheduling_rules r JOIN catalog_services c ON c.id=r.service_id AND c.organization_id=r.organization_id AND c.location_id=r.location_id WHERE r.organization_id=$1 AND r.location_id=$2 AND r.id=$3 AND r.resource_id=$4 AND r.version=$5 AND c.kind='main' AND NOT c.archived`, scope.OrganizationID, scope.LocationID, ruleID, resourceID, ruleVersion).Scan(&serviceID, &serviceName, &mainPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, 0, ErrPublicBookingInput
	}
	if err != nil {
		return "", "", nil, 0, err
	}
	if mainPrice == nil {
		return "", "", nil, 0, ErrPublicBookingPrice
	}
	items = []CreateInvoiceItemInput{{Description: serviceName, Quantity: 1, UnitPriceCents: *mainPrice}}
	subtotal = *mainPrice
	for _, id := range addonIDs {
		var name string
		var price *int64
		err = s.pool.QueryRow(ctx, `SELECT name,price_cents FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND kind='addon' AND catalog_addon_offered_with(id,$4::uuid) AND NOT archived`, scope.OrganizationID, scope.LocationID, id, serviceID).Scan(&name, &price)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, 0, ErrPublicBookingInput
		}
		if err != nil {
			return "", "", nil, 0, err
		}
		if price == nil {
			return "", "", nil, 0, ErrPublicBookingPrice
		}
		items = append(items, CreateInvoiceItemInput{Description: name, Quantity: 1, UnitPriceCents: *price})
		subtotal += *price
	}
	return serviceID, serviceName, items, subtotal, nil
}

// InvoiceStaffBooking is for a booking staff made with a coupon: it prices the booking, applies the coupon and issues the invoice, so the
// discount is on the books from the start. The coupon is checked first, so a bad code stops the booking before anything is held.
func (s *Store) CheckStaffCoupon(ctx context.Context, scope Scope, in ReserveBookingInput, code string) error {
	serviceID, _, _, subtotal, err := s.priceBooking(ctx, scope, in.RuleID, in.ResourceID, in.RuleVersion, in.AddonIDs)
	if err != nil {
		return err
	}
	_, err = s.evaluateCoupon(ctx, s.pool, scope, code, in.CustomerID, &serviceID, subtotal)
	return err
}

func (s *Store) InvoiceStaffBooking(ctx context.Context, scope Scope, in ReserveBookingInput, bookingID, code string) error {
	_, _, items, subtotal, err := s.priceBooking(ctx, scope, in.RuleID, in.ResourceID, in.RuleVersion, in.AddonIDs)
	if err != nil {
		return err
	}
	tax, _, err := s.taxOnSubtotal(ctx, scope, subtotal)
	if err != nil {
		return err
	}
	inv, err := s.CreateInvoice(ctx, scope, CreateInvoiceInput{CustomerID: in.CustomerID, BookingID: &bookingID, TaxCents: tax, Notes: "Booked by staff", Items: items})
	if err != nil {
		return err
	}
	if _, err = s.ApplyCouponToInvoice(ctx, scope, inv.ID, code); err != nil {
		_, _ = s.VoidInvoice(context.WithoutCancel(ctx), scope, inv.ID, "The coupon could not be applied")
		return err
	}
	if _, err = s.IssueInvoice(ctx, scope, inv.ID); err != nil {
		_, _ = s.VoidInvoice(context.WithoutCancel(ctx), scope, inv.ID, "The invoice could not be issued")
		return err
	}
	return nil
}

// PublicBookingScope is the website's scope for a published site (used to take the deposit with the card processor).
func (s *Store) PublicBookingScope(ctx context.Context, slug string) (Scope, error) {
	return s.websiteScope(ctx, slug)
}

// PublicBookingAbort voids the invoice and releases the held time after the deposit failed.
func (s *Store) PublicBookingAbort(ctx context.Context, slug, bookingID, invoiceID string) {
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return
	}
	c := context.WithoutCancel(ctx)
	_, _ = s.VoidInvoice(c, scope, invoiceID, "The deposit was not paid")
	_, _ = s.UpdateBookingStatus(c, scope, bookingID, "cancelled", "The deposit was not paid", "")
}

func hashOf(parts ...string) []byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}

// StartPhoneVerification texts a six-digit code. At most three codes per number per hour.
func (s *Store) StartPhoneVerification(ctx context.Context, slug, phone string) error {
	if s.phoneVerifier == nil {
		return ErrPhoneVerificationUnavailable
	}
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return err
	}
	d := phoneDigits(phone)
	if len(d) < 7 || len(d) > 15 {
		return ErrPublicBookingInput
	}
	var recent int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM phone_verifications WHERE organization_id=$1 AND location_id=$2 AND phone_digits=$3 AND created_at>now()-interval '1 hour'`, scope.OrganizationID, scope.LocationID, d).Scan(&recent); err != nil {
		return err
	}
	if recent >= 3 {
		return ErrPhoneVerificationLimit
	}
	raw := make([]byte, 4)
	if _, err = rand.Read(raw); err != nil {
		return err
	}
	n := (uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])) % 1000000
	code := []byte("000000")
	for i := 5; i >= 0; i-- {
		code[i] = byte('0' + n%10)
		n /= 10
	}
	var id string
	if err = s.pool.QueryRow(ctx, `INSERT INTO phone_verifications(organization_id,location_id,phone_digits,code_hash,expires_at) VALUES($1,$2,$3,'\x00',now()+interval '10 minutes') RETURNING id::text`, scope.OrganizationID, scope.LocationID, d).Scan(&id); err != nil {
		return err
	}
	if _, err = s.pool.Exec(ctx, `UPDATE phone_verifications SET code_hash=$2 WHERE id=$1`, id, hashOf(id, string(code))); err != nil {
		return err
	}
	if err = s.phoneVerifier.SendCode(ctx, phone, string(code)); err != nil {
		_, _ = s.pool.Exec(ctx, `DELETE FROM phone_verifications WHERE id=$1`, id)
		return err
	}
	return nil
}

// CheckPhoneVerification exchanges the right code for a short-lived token (30 minutes). Five wrong tries lock that code.
func (s *Store) CheckPhoneVerification(ctx context.Context, slug, phone, code string) (string, error) {
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return "", err
	}
	d := phoneDigits(phone)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id string
	var hash []byte
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id::text,code_hash,attempts FROM phone_verifications WHERE organization_id=$1 AND location_id=$2 AND phone_digits=$3 AND verified_at IS NULL AND expires_at>now() ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, scope.OrganizationID, scope.LocationID, d).Scan(&id, &hash, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPhoneVerification
	}
	if err != nil {
		return "", err
	}
	if attempts >= 5 {
		return "", ErrPhoneVerificationLimit
	}
	if subtle.ConstantTimeCompare(hash, hashOf(id, strings.TrimSpace(code))) != 1 {
		if _, err = tx.Exec(ctx, `UPDATE phone_verifications SET attempts=attempts+1 WHERE id=$1`, id); err != nil {
			return "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", ErrPhoneVerification
	}
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	if _, err = tx.Exec(ctx, `UPDATE phone_verifications SET verified_at=now(),token_hash=$2,expires_at=now()+interval '30 minutes' WHERE id=$1`, id, hashOf(id, token)); err != nil {
		return "", err
	}
	return id + "." + token, tx.Commit(ctx)
}

// ReturningCustomers shows the saved name and email for a number, but only to someone who proved they hold it.
func (s *Store) ReturningCustomers(ctx context.Context, slug, phone, token string) ([]Customer, error) {
	scope, err := s.websiteScope(ctx, slug)
	if err != nil {
		return nil, err
	}
	d := phoneDigits(phone)
	id, secret, ok := strings.Cut(token, ".")
	if !ok || len(d) < 7 {
		return nil, ErrPhoneVerification
	}
	var stored []byte
	err = s.pool.QueryRow(ctx, `SELECT token_hash FROM phone_verifications WHERE id::text=$1 AND organization_id=$2 AND location_id=$3 AND phone_digits=$4 AND verified_at IS NOT NULL AND expires_at>now()`, id, scope.OrganizationID, scope.LocationID, d).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && subtle.ConstantTimeCompare(stored, hashOf(id, secret)) != 1 {
		return nil, ErrPhoneVerification
	}
	if err != nil {
		return nil, err
	}
	tail := d
	if len(tail) > 10 {
		tail = tail[len(tail)-10:]
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,email,phone FROM customers WHERE organization_id=$1 AND location_id=$2 AND removed_at IS NULL AND right(regexp_replace(phone,'[^0-9]','','g'),10)=$3 ORDER BY created_at LIMIT 5`, scope.OrganizationID, scope.LocationID, tail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Customer{}
	for rows.Next() {
		var c Customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
