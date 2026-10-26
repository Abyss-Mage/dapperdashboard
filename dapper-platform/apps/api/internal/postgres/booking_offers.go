package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrBookingOffer = errors.New("explicit valid reviewed booking offer required")
var ErrBookingOfferConflict = errors.New("booking offer changed, duplicated or has a stale scheduling rule")

type BookingOfferConfig struct {
	ResourceID        string `json:"resourceId"`
	RuleID            string `json:"ruleId"`
	RuleVersion       int    `json:"ruleVersion"`
	Title             string `json:"title"`
	EquipmentCategory string `json:"equipmentCategory"`
	Mode              string `json:"mode"`
	Currency          string `json:"currency"`
	SubtotalCents     int64  `json:"subtotalCents"`
	TaxCents          int64  `json:"taxCents"`
	DepositCents      int64  `json:"depositCents"`
	Terms             string `json:"terms"`
	CancellationTerms string `json:"cancellationTerms"`
	LeadMinutes       int    `json:"leadMinutes"`
	AdvanceDays       int    `json:"advanceDays"`
	SlotMinutes       int    `json:"slotMinutes"`
	Published         bool   `json:"published"`
}
type BookingOffer struct {
	BookingOfferConfig
	ID         string     `json:"id"`
	Version    int        `json:"version"`
	ReviewedBy *string    `json:"reviewedBy,omitempty"`
	ReviewedAt *time.Time `json:"reviewedAt,omitempty"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

const offerColumns = `id::text,resource_id::text,rule_id::text,rule_version,version,title,equipment_category,mode,currency,subtotal_cents,tax_cents,deposit_cents,terms,cancellation_terms,lead_minutes,advance_days,slot_minutes,published,reviewed_by::text,reviewed_at,updated_at`

func scanBookingOffer(row pgx.Row) (BookingOffer, error) {
	var o BookingOffer
	err := row.Scan(&o.ID, &o.ResourceID, &o.RuleID, &o.RuleVersion, &o.Version, &o.Title, &o.EquipmentCategory, &o.Mode, &o.Currency, &o.SubtotalCents, &o.TaxCents, &o.DepositCents, &o.Terms, &o.CancellationTerms, &o.LeadMinutes, &o.AdvanceDays, &o.SlotMinutes, &o.Published, &o.ReviewedBy, &o.ReviewedAt, &o.UpdatedAt)
	return o, err
}
func offerWriteError(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) && (p.Code == "23505" || p.Code == "23503") {
		return ErrBookingOfferConflict
	}
	return err
}
func authorizeOffer(ctx context.Context, tx pgx.Tx, scope Scope) error {
	for _, action := range []string{"catalogue.manage", "scheduling.manage"} {
		if err := authorizeTransaction(ctx, tx, scope, action); err != nil {
			return err
		}
	}
	return nil
}

// SaveBookingOffer configures publication; review is an accountable operator
// assertion, not evidence that a client has approved values outside this system.
func (s *Store) SaveBookingOffer(ctx context.Context, scope Scope, id string, expected int, reviewed bool, c BookingOfferConfig) (BookingOffer, error) {
	c.Title = strings.TrimSpace(c.Title)
	c.EquipmentCategory = strings.TrimSpace(c.EquipmentCategory)
	c.Terms = strings.TrimSpace(c.Terms)
	c.CancellationTerms = strings.TrimSpace(c.CancellationTerms)
	if c.ResourceID == "" || c.RuleID == "" || c.RuleVersion < 1 || expected < 0 || id == "" && expected != 0 || id != "" && expected < 1 || utf8.RuneCountInString(c.Title) < 2 || utf8.RuneCountInString(c.Title) > 120 || utf8.RuneCountInString(c.EquipmentCategory) < 1 || utf8.RuneCountInString(c.EquipmentCategory) > 60 || utf8.RuneCountInString(c.Terms) < 1 || utf8.RuneCountInString(c.Terms) > 10000 || utf8.RuneCountInString(c.CancellationTerms) < 1 || utf8.RuneCountInString(c.CancellationTerms) > 10000 || c.Currency != "CAD" || c.SubtotalCents < 0 || c.SubtotalCents > 9000000000000 || c.TaxCents < 0 || c.TaxCents > 9000000000000 || c.DepositCents < 0 || c.DepositCents > c.SubtotalCents+c.TaxCents || c.LeadMinutes < 0 || c.LeadMinutes > 525600 || c.AdvanceDays < 1 || c.AdvanceDays > 366 || c.SlotMinutes < 1 || c.SlotMinutes > 1440 || c.Published && !reviewed {
		return BookingOffer{}, ErrBookingOffer
	}
	if c.Mode != "fixed" && c.Mode != "quote_required" || c.Mode == "quote_required" && (c.SubtotalCents != 0 || c.TaxCents != 0 || c.DepositCents != 0) {
		return BookingOffer{}, ErrBookingOffer
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return BookingOffer{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeOffer(ctx, tx, scope); err != nil {
		return BookingOffer{}, err
	}
	// Follow scheduling's resource-before-rule order, before taking the offer row.
	var resource string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM scheduling_resources WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR SHARE`, scope.OrganizationID, scope.LocationID, c.ResourceID).Scan(&resource); errors.Is(err, pgx.ErrNoRows) {
		return BookingOffer{}, ErrBookingOfferConflict
	} else if err != nil {
		return BookingOffer{}, err
	}
	if c.Published {
		if _, err = resolveReservationRule(ctx, tx, scope, c.ResourceID, reservationRuleChoice{c.RuleID, c.RuleVersion}); err != nil {
			if errors.Is(err, ErrSchedulingRuleConflict) || errors.Is(err, ErrSchedulingRule) {
				return BookingOffer{}, ErrBookingOfferConflict
			}
			return BookingOffer{}, err
		}
	}
	if id != "" {
		var version int
		if err = tx.QueryRow(ctx, `SELECT version FROM booking_offers WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, id).Scan(&version); errors.Is(err, pgx.ErrNoRows) {
			return BookingOffer{}, ErrBookingOfferConflict
		} else if err != nil {
			return BookingOffer{}, err
		}
		if version != expected {
			return BookingOffer{}, ErrBookingOfferConflict
		}
	}
	args := []any{scope.OrganizationID, scope.LocationID, c.ResourceID, c.RuleID, c.RuleVersion, c.Title, c.EquipmentCategory, c.Mode, c.Currency, c.SubtotalCents, c.TaxCents, c.DepositCents, c.Terms, c.CancellationTerms, c.LeadMinutes, c.AdvanceDays, c.SlotMinutes, c.Published, scope.ActorID}
	var saved BookingOffer
	if id == "" {
		saved, err = scanBookingOffer(tx.QueryRow(ctx, `INSERT INTO booking_offers(organization_id,location_id,resource_id,rule_id,rule_version,title,equipment_category,mode,currency,subtotal_cents,tax_cents,deposit_cents,terms,cancellation_terms,lead_minutes,advance_days,slot_minutes,published,reviewed_by,reviewed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,CASE WHEN $18 THEN $19::uuid ELSE NULL END,CASE WHEN $18 THEN now() ELSE NULL END) RETURNING `+offerColumns, args...))
	} else {
		args = append(args, id)
		saved, err = scanBookingOffer(tx.QueryRow(ctx, `UPDATE booking_offers SET resource_id=$3,rule_id=$4,rule_version=$5,title=$6,equipment_category=$7,mode=$8,currency=$9,subtotal_cents=$10,tax_cents=$11,deposit_cents=$12,terms=$13,cancellation_terms=$14,lead_minutes=$15,advance_days=$16,slot_minutes=$17,published=$18,reviewed_by=CASE WHEN $18 THEN $19::uuid ELSE NULL END,reviewed_at=CASE WHEN $18 THEN now() ELSE NULL END,version=version+1,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$20 RETURNING `+offerColumns, args...))
	}
	if err != nil {
		return BookingOffer{}, offerWriteError(err)
	}
	snapshot, err := json.Marshal(saved)
	if err != nil {
		return BookingOffer{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_offer_revisions(organization_id,location_id,offer_id,version,snapshot,actor_id) VALUES($1,$2,$3,$4,$5,$6)`, scope.OrganizationID, scope.LocationID, saved.ID, saved.Version, snapshot, scope.ActorID); err != nil {
		return BookingOffer{}, err
	}
	if err = jobAudit(ctx, tx, scope, "booking_offer.saved", saved.ID); err != nil {
		return BookingOffer{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return BookingOffer{}, offerWriteError(err)
	}
	return saved, nil
}
func (s *Store) BookingOffers(ctx context.Context, scope Scope) ([]BookingOffer, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeOffer(ctx, tx, scope); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+offerColumns+` FROM booking_offers WHERE organization_id=$1 AND location_id=$2 ORDER BY title,id LIMIT 100`, scope.OrganizationID, scope.LocationID)
	if err != nil {
		return nil, err
	}
	items := []BookingOffer{}
	for rows.Next() {
		o, e := scanBookingOffer(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		items = append(items, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

type PortalBookingOffer struct {
	ID                string `json:"id"`
	Version           int    `json:"version"`
	Title             string `json:"title"`
	EquipmentCategory string `json:"equipmentCategory"`
	Mode              string `json:"mode"`
	Currency          string `json:"currency"`
	SubtotalCents     int64  `json:"subtotalCents"`
	TaxCents          int64  `json:"taxCents"`
	DepositCents      int64  `json:"depositCents"`
	Terms             string `json:"terms"`
	CancellationTerms string `json:"cancellationTerms"`
	DurationMinutes   int    `json:"durationMinutes"`
	Timezone          string `json:"timezone"`
}

func (s *Store) CustomerBookingOffers(ctx context.Context, actor, customer, vehicle string) ([]PortalBookingOffer, error) {
	if actor == "" || customer == "" || vehicle == "" {
		return nil, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var org, loc, category string
	err = tx.QueryRow(ctx, `SELECT a.organization_id::text,a.location_id::text,lower(btrim(v.category_description)) FROM customer_account_links a JOIN principals p ON p.id=a.principal_id AND p.active JOIN vehicles v ON (v.organization_id,v.location_id,v.customer_id)=(a.organization_id,a.location_id,a.customer_id) WHERE a.principal_id=$1 AND a.customer_id=$2 AND v.id=$3 FOR SHARE OF a,p,v`, actor, customer, vehicle).Scan(&org, &loc, &category)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT o.id::text,o.version,o.title,o.equipment_category,o.mode,o.currency,o.subtotal_cents,o.tax_cents,o.deposit_cents,o.terms,o.cancellation_terms,s.duration_minutes,r.timezone FROM booking_offers o JOIN service_scheduling_rules s ON (s.organization_id,s.location_id,s.resource_id,s.id,s.version)=(o.organization_id,o.location_id,o.resource_id,o.rule_id,o.rule_version) JOIN scheduling_resources r ON r.id=o.resource_id JOIN catalog_services c ON c.id=s.service_id WHERE o.organization_id=$1 AND o.location_id=$2 AND o.category_key=$3 AND o.published AND c.kind='main' AND NOT c.archived AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text(s.addon_ids) aid(value) WHERE NOT EXISTS(SELECT 1 FROM catalog_services ca WHERE ca.id::text=aid.value AND ca.organization_id=o.organization_id AND ca.location_id=o.location_id AND ca.kind='addon' AND ca.parent_id=s.service_id AND NOT ca.archived)) ORDER BY o.title,o.id LIMIT 100`, org, loc, category)
	if err != nil {
		return nil, err
	}
	items := []PortalBookingOffer{}
	for rows.Next() {
		var o PortalBookingOffer
		if err = rows.Scan(&o.ID, &o.Version, &o.Title, &o.EquipmentCategory, &o.Mode, &o.Currency, &o.SubtotalCents, &o.TaxCents, &o.DepositCents, &o.Terms, &o.CancellationTerms, &o.DurationMinutes, &o.Timezone); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
