package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"dapper.local/platform/internal/scheduling"
	"github.com/jackc/pgx/v5"
)

// ErrBookingPaid: money has been taken on the booking's invoice, so its package and add-ons are not changed here (the books must match).
var ErrBookingPaid = errors.New("the booking has payments")

// ChangeBookingService changes an open booking's package and add-ons (owner, error404 #167): an upgrade, another service, or add-ons
// added or taken off. The booking keeps its time. Its calendar hold follows the new package's timing at the same start, and is refused
// (nothing changes) if the new package does not fit then; with no timing rule for the new package the hold is given back. An unpaid
// invoice is voided and a new draft is made at the current prices; a booking with any payment is refused. Needs bookings.manage.
// It reports whether an issued invoice was replaced, so staff know to send the new one.
func (s *Store) ChangeBookingService(ctx context.Context, scope Scope, id, serviceID string, addonIDs []string) (Booking, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Booking{}, false, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return Booking{}, false, err
	}
	b, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id::text=$3 FOR UPDATE OF b`, scope.OrganizationID, scope.LocationID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, false, err
	}
	switch b.Status {
	case "pending", "awaiting_payment", "confirmed", "scheduled":
	default:
		return Booking{}, false, ErrBookingTransition
	}
	var paid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices WHERE booking_id=$1 AND status<>'void' AND paid_cents>0) OR EXISTS(SELECT 1 FROM square_payment_intents p JOIN invoices i ON i.id=p.invoice_id WHERE i.booking_id=$1 AND p.state IN ('submitting','uncertain','captured','settled'))`, b.ID).Scan(&paid); err != nil {
		return Booking{}, false, err
	}
	if paid {
		return Booking{}, false, ErrBookingPaid
	}
	var name string
	err = tx.QueryRow(ctx, `SELECT name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND kind='main' AND NOT archived`, scope.OrganizationID, scope.LocationID, serviceID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, ErrBookingInput
	}
	if err != nil {
		return Booking{}, false, err
	}
	addons := []JobAddon{}
	seen := map[string]bool{}
	for _, a := range addonIDs {
		if seen[a] {
			return Booking{}, false, ErrBookingInput
		}
		seen[a] = true
		var an string
		err = tx.QueryRow(ctx, `SELECT name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND kind='addon' AND NOT archived AND catalog_addon_offered_with(id,$4::uuid)`, scope.OrganizationID, scope.LocationID, a, serviceID).Scan(&an)
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, false, ErrBookingInput
		}
		if err != nil {
			return Booking{}, false, err
		}
		addons = append(addons, JobAddon{ServiceID: a, Name: an})
	}
	addonsJSON, err := json.Marshal(addons)
	if err != nil {
		return Booking{}, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bookings SET service_id=$4::uuid,service_name=$5,addons=$6,updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, b.ID, serviceID, name, addonsJSON); err != nil {
		return Booking{}, false, bookingWriteError(err)
	}

	// The calendar hold follows the new package at the same start, or is given back if the package has no timing rule there.
	if b.ReservationID != nil {
		var resource string
		var version int
		var cancelled bool
		if err = tx.QueryRow(ctx, `SELECT resource_id::text,version,cancelled_at IS NOT NULL FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`, scope.OrganizationID, scope.LocationID, *b.ReservationID).Scan(&resource, &version, &cancelled); err != nil {
			return Booking{}, false, ErrReservationConflict
		}
		if !cancelled {
			var rule string
			var ruleVersion int
			err = tx.QueryRow(ctx, `SELECT id::text,version FROM service_scheduling_rules WHERE organization_id=$1 AND location_id=$2 AND resource_id=$3 AND service_id::text=$4 AND addon_ids='[]'::jsonb`, scope.OrganizationID, scope.LocationID, resource, serviceID).Scan(&rule, &ruleVersion)
			switch {
			case err == nil:
				if _, err = tx.Exec(ctx, `UPDATE capacity_reservations SET rule_id=$2,rule_version=$3 WHERE id=$1`, *b.ReservationID, rule, ruleVersion); err != nil {
					return Booking{}, false, err
				}
				choice := reservationRuleChoice{ID: rule, Version: ruleVersion}
				if _, err = s.writeReservationInTransaction(ctx, tx, scope, resource, *b.ReservationID, version, scheduling.Request{Start: b.BookingTime}, &choice); err != nil {
					return Booking{}, false, err
				}
			case errors.Is(err, pgx.ErrNoRows):
				if _, err = tx.Exec(ctx, `UPDATE bookings SET reservation_id=NULL WHERE id=$1`, b.ID); err != nil {
					return Booking{}, false, err
				}
				if _, err = s.releaseCapacityInTransaction(ctx, tx, scope, resource, *b.ReservationID, version); err != nil {
					return Booking{}, false, err
				}
			default:
				return Booking{}, false, err
			}
		}
	}

	// The unpaid invoice is replaced by a draft at the new package's prices.
	var replacedIssued bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invoices WHERE booking_id=$1 AND status IN ('issued','partially_paid'))`, b.ID).Scan(&replacedIssued); err != nil {
		return Booking{}, false, err
	}
	rows, err := tx.Query(ctx, `UPDATE invoices SET status='void',updated_at=now() WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 AND status<>'void' RETURNING id::text,status`, scope.OrganizationID, scope.LocationID, b.ID)
	if err != nil {
		return Booking{}, false, err
	}
	var voided []string
	for rows.Next() {
		var vid, st string
		if err = rows.Scan(&vid, &st); err != nil {
			rows.Close()
			return Booking{}, false, err
		}
		voided = append(voided, vid)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return Booking{}, false, err
	}
	for _, vid := range voided {
		if _, err = tx.Exec(ctx, `INSERT INTO invoice_events(invoice_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,NULL,'void','Replaced: the booking''s package or add-ons were changed')`, vid, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
			return Booking{}, false, err
		}
	}
	b.ServiceID, b.ServiceName, b.Addons = &serviceID, name, addons
	s.autoInvoiceDraft(ctx, tx, scope, b)

	names := make([]string, 0, len(addons))
	for _, a := range addons {
		names = append(names, a.Name)
	}
	note := "Package changed to " + name
	if len(names) > 0 {
		note += " with " + strings.Join(names, ", ")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_events(organization_id,location_id,booking_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,$5,$5,$6)`, scope.OrganizationID, scope.LocationID, b.ID, scope.ActorID, b.Status, note); err != nil {
		return Booking{}, false, err
	}
	if err = jobAudit(ctx, tx, scope, "bookings.service_changed", b.ID); err != nil {
		return Booking{}, false, err
	}
	out, err := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id=$1`, b.ID))
	if err != nil {
		return Booking{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, false, bookingWriteError(err)
	}
	return out, replacedIssued, nil
}
