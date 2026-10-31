package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var (
	ErrBookingInput      = errors.New("valid customer, service, time, and channel required")
	ErrBookingNotFound   = errors.New("booking not found in this scope")
	ErrBookingTransition = errors.New("invalid booking status transition")
)

type Booking struct {
	ReservationVersion *int       `json:"reservationVersion,omitempty"`
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organizationId"`
	LocationID         string     `json:"locationId"`
	CustomerID         string     `json:"customerId"`
	VehicleID          *string    `json:"vehicleId,omitempty"`
	ServiceID          *string    `json:"serviceId,omitempty"`
	ServiceName        string     `json:"serviceName"`
	Addons             []JobAddon `json:"addons"`
	AddonIDs           []string   `json:"-"`
	ReservationID      *string    `json:"reservationId,omitempty"`
	Channel            string     `json:"channel"`
	Status             string     `json:"status"`
	BookingTime        time.Time  `json:"bookingTime"`
	Notes              string     `json:"notes"`
	ServiceAddress     string     `json:"serviceAddress,omitempty"`
	CancellationReason string     `json:"cancellationReason"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	CustomerName       string     `json:"customerName,omitempty"`
	CustomerEmail      string     `json:"customerEmail,omitempty"`
	CustomerPhone      string     `json:"customerPhone,omitempty"`
}

type BookingEvent struct {
	ID             string    `json:"id"`
	BookingID      string    `json:"bookingId"`
	OrganizationID string    `json:"organizationId"`
	LocationID     string    `json:"locationId"`
	ActorID        *string   `json:"actorId,omitempty"`
	FromStatus     *string   `json:"fromStatus,omitempty"`
	ToStatus       string    `json:"toStatus"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"createdAt"`
}

const bookingColumns = `b.id::text, b.organization_id::text, b.location_id::text, b.customer_id::text,
b.vehicle_id::text, b.service_id::text, b.service_name, b.addons, b.reservation_id::text,
b.channel, b.status, b.booking_time, b.notes, b.cancellation_reason, b.created_at, b.updated_at,
c.name, c.email, c.phone,(SELECT r.version FROM capacity_reservations r WHERE r.organization_id=b.organization_id AND r.location_id=b.location_id AND r.id=b.reservation_id), b.service_address`

func scanBooking(row pgx.Row) (Booking, error) {
	var b Booking
	err := row.Scan(
		&b.ID,
		&b.OrganizationID,
		&b.LocationID,
		&b.CustomerID,
		&b.VehicleID,
		&b.ServiceID,
		&b.ServiceName,
		&b.Addons,
		&b.ReservationID,
		&b.Channel,
		&b.Status,
		&b.BookingTime,
		&b.Notes,
		&b.CancellationReason,
		&b.CreatedAt,
		&b.UpdatedAt,
		&b.CustomerName,
		&b.CustomerEmail,
		&b.CustomerPhone,
		&b.ReservationVersion,
		&b.ServiceAddress,
	)
	return b, err
}

func (s *Store) CreateBooking(ctx context.Context, scope Scope, input Booking) (Booking, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	created, err := s.createBookingInTransaction(ctx, tx, scope, input)
	if err != nil {
		return Booking{}, bookingWriteError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	return created, nil
}
func (s *Store) createBookingInTransaction(ctx context.Context, tx pgx.Tx, scope Scope, input Booking) (Booking, error) {
	input.ServiceName = strings.TrimSpace(input.ServiceName)
	input.Notes = strings.TrimSpace(input.Notes)
	input.ServiceAddress = strings.TrimSpace(input.ServiceAddress)
	if input.CustomerID == "" || input.BookingTime.IsZero() || utf8.RuneCountInString(input.Notes) > 2000 || utf8.RuneCountInString(input.ServiceAddress) > 300 {
		return Booking{}, ErrBookingInput
	}
	switch input.Channel {
	case "online", "internal", "phone", "walk_in", "dealership":
	default:
		return Booking{}, ErrBookingInput
	}
	if input.Status == "" {
		input.Status = "confirmed"
	}
	switch input.Status {
	case "pending", "awaiting_payment", "confirmed", "scheduled", "in_progress", "completed", "cancelled":
	default:
		return Booking{}, ErrBookingInput
	}
	if len(input.AddonIDs) > 100 || len(input.AddonIDs) > 0 && input.ServiceID == nil {
		return Booking{}, ErrBookingInput
	}

	var err error
	if err = authorizeTransaction(ctx, tx, scope, "bookings.create"); err != nil {
		return Booking{}, err
	}

	// Verify customer exists
	var customerExists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE organization_id=$1 AND location_id=$2 AND id=$3)`,
		scope.OrganizationID, scope.LocationID, input.CustomerID).Scan(&customerExists)
	if err != nil || !customerExists {
		return Booking{}, ErrBookingInput
	}

	// Resolve service name if serviceId provided
	if input.VehicleID != nil {
		var vehicleID string
		err = tx.QueryRow(ctx, `SELECT id::text FROM vehicles WHERE organization_id=$1 AND location_id=$2 AND customer_id=$3 AND id=$4 FOR SHARE`, scope.OrganizationID, scope.LocationID, input.CustomerID, *input.VehicleID).Scan(&vehicleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingInput
		}
		if err != nil {
			return Booking{}, err
		}
	}

	if input.ServiceID != nil {
		err = tx.QueryRow(ctx, `SELECT name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='main' AND NOT archived FOR SHARE`,
			scope.OrganizationID, scope.LocationID, *input.ServiceID).Scan(&input.ServiceName)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT name FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND kind='main' AND NOT archived FOR SHARE`,
				scope.OrganizationID, *input.ServiceID).Scan(&input.ServiceName)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingInput
		}
		if err != nil {
			return Booking{}, err
		}
	}
	if n := utf8.RuneCountInString(input.ServiceName); n < 2 || n > 120 {
		return Booking{}, ErrBookingInput
	}

	// Resolve addons
	addons := []JobAddon{}
	ids := append([]string(nil), input.AddonIDs...)
	sort.Strings(ids)
	for i, addonID := range ids {
		if i > 0 && addonID == ids[i-1] {
			return Booking{}, ErrBookingInput
		}
		var name string
		err = tx.QueryRow(ctx, `SELECT name FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND kind='addon' AND parent_id=$4 AND NOT archived FOR SHARE`,
			scope.OrganizationID, scope.LocationID, addonID, *input.ServiceID).Scan(&name)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT name FROM central_catalog_services WHERE organization_id=$1 AND id=$2 AND kind='addon' AND parent_id=$3 AND NOT archived FOR SHARE`,
				scope.OrganizationID, addonID, *input.ServiceID).Scan(&name)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingInput
		}
		if err != nil {
			return Booking{}, err
		}
		addons = append(addons, JobAddon{ServiceID: addonID, Name: name})
	}
	input.Addons = addons

	// Verify reservation if provided
	if input.ReservationID != nil {
		var resExists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND id=$3 AND cancelled_at IS NULL FOR SHARE)`,
			scope.OrganizationID, scope.LocationID, *input.ReservationID).Scan(&resExists)
		if err != nil || !resExists {
			return Booking{}, ErrBookingInput
		}
	}

	var newID string
	err = tx.QueryRow(ctx, `INSERT INTO bookings(organization_id, location_id, customer_id, vehicle_id, service_id, service_name, addons, reservation_id, channel, status, booking_time, notes, service_address)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id::text`,
		scope.OrganizationID, scope.LocationID, input.CustomerID, input.VehicleID, input.ServiceID, input.ServiceName,
		input.Addons, input.ReservationID, input.Channel, input.Status, input.BookingTime, input.Notes, input.ServiceAddress).Scan(&newID)
	if err != nil {
		return Booking{}, err
	}

	// A service address (mobile booking) is also remembered on the customer for next time.
	if input.ServiceAddress != "" {
		if _, err = tx.Exec(ctx, `UPDATE customers SET address=$4 WHERE organization_id=$1 AND location_id=$2 AND id=$3`, scope.OrganizationID, scope.LocationID, input.CustomerID, input.ServiceAddress); err != nil {
			return Booking{}, err
		}
	}

	// Record creation event
	_, err = tx.Exec(ctx, `INSERT INTO booking_events(booking_id, organization_id, location_id, actor_id, from_status, to_status, notes)
VALUES ($1, $2, $3, $4, NULL, $5, 'Booking created')`, newID, scope.OrganizationID, scope.LocationID, scope.ActorID, input.Status)
	if err != nil {
		return Booking{}, err
	}

	// Every booking is part of this location's work, so staff see it on their own screens whoever created it.
	if _, err = tx.Exec(ctx, `INSERT INTO employee_booking_worksites(booking_id,organization_id,location_id,dealership_id,linked_by)
SELECT $1,$2,$3,NULL,p.id FROM principals p WHERE p.id=NULLIF($4,'')::uuid ON CONFLICT(booking_id) DO NOTHING`, newID, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return Booking{}, err
	}

	row := tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id WHERE b.id=$1`, newID)
	created, err := scanBooking(row)
	if err != nil {
		return Booking{}, err
	}
	s.autoInvoiceDraft(ctx, tx, scope, created)

	return created, nil
}

func (s *Store) ListBookings(ctx context.Context, scope Scope, from, to time.Time, status string, limit int) ([]Booking, error) {
	if err := scope.validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.read"); err != nil {
		return nil, err
	}

	query := `SELECT ` + bookingColumns + ` FROM bookings b
JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id
WHERE b.organization_id=$1 AND b.location_id=$2 AND b.booking_time >= $3 AND b.booking_time <= $4`
	args := []any{scope.OrganizationID, scope.LocationID, from, to}

	if status != "" {
		query += fmt.Sprintf(" AND b.status=$%d", len(args)+1)
		args = append(args, status)
	}
	query += fmt.Sprintf(" ORDER BY b.booking_time ASC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) GetBooking(ctx context.Context, scope Scope, id string) (Booking, []BookingEvent, error) {
	if err := scope.validate(); err != nil {
		return Booking{}, nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Booking{}, nil, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.read"); err != nil {
		return Booking{}, nil, err
	}

	row := tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b
JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id
WHERE b.organization_id=$1 AND b.location_id=$2 AND b.id=$3`, scope.OrganizationID, scope.LocationID, id)
	booking, err := scanBooking(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, nil, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, nil, err
	}

	evRows, err := tx.Query(ctx, `SELECT id::text, booking_id::text, organization_id::text, location_id::text, actor_id::text, from_status, to_status, notes, created_at
FROM booking_events WHERE organization_id=$1 AND location_id=$2 AND booking_id=$3 ORDER BY created_at ASC`,
		scope.OrganizationID, scope.LocationID, id)
	if err != nil {
		return Booking{}, nil, err
	}
	defer evRows.Close()

	var events []BookingEvent
	for evRows.Next() {
		var ev BookingEvent
		if err = evRows.Scan(&ev.ID, &ev.BookingID, &ev.OrganizationID, &ev.LocationID, &ev.ActorID, &ev.FromStatus, &ev.ToStatus, &ev.Notes, &ev.CreatedAt); err != nil {
			return Booking{}, nil, err
		}
		events = append(events, ev)
	}
	if err = evRows.Err(); err != nil {
		return Booking{}, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, nil, err
	}
	return booking, events, nil
}

func (s *Store) UpdateBookingStatus(ctx context.Context, scope Scope, id string, toStatus, cancellationReason, notes string, reservationVersions ...int) (Booking, error) {
	toStatus = strings.TrimSpace(toStatus)
	cancellationReason = strings.TrimSpace(cancellationReason)
	notes = strings.TrimSpace(notes)
	if toStatus == "cancelled" {
		return s.cancelBooking(ctx, scope, id, cancellationReason, notes, reservationVersions)
	}
	if len(reservationVersions) > 0 || utf8.RuneCountInString(notes) > 2000 || utf8.RuneCountInString(cancellationReason) > 2000 {
		return Booking{}, ErrBookingInput
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "bookings.manage"); err != nil {
		return Booking{}, err
	}

	previewReservation, err := lockBookingAllocation(ctx, tx, scope, id)
	if err != nil {
		return Booking{}, err
	}
	var currentStatus string
	var currentReservation *string
	err = tx.QueryRow(ctx, `SELECT status,reservation_id::text FROM bookings WHERE organization_id=$1 AND location_id=$2 AND id=$3 FOR UPDATE`,
		scope.OrganizationID, scope.LocationID, id).Scan(&currentStatus, &currentReservation)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if (previewReservation == nil) != (currentReservation == nil) || previewReservation != nil && *previewReservation != *currentReservation {
		return Booking{}, ErrReservationConflict
	}

	// Validate status transition
	if currentStatus == toStatus {
		return Booking{}, nil
	}
	if currentStatus == "completed" || currentStatus == "cancelled" {
		return Booking{}, ErrBookingTransition
	}

	validTransition := false
	switch currentStatus {
	case "pending":
		validTransition = (toStatus == "confirmed" || toStatus == "awaiting_payment" || toStatus == "completed" || toStatus == "cancelled")
	case "awaiting_payment":
		validTransition = (toStatus == "confirmed" || toStatus == "completed" || toStatus == "cancelled")
	case "confirmed":
		validTransition = (toStatus == "scheduled" || toStatus == "in_progress" || toStatus == "completed" || toStatus == "cancelled")
	case "scheduled":
		validTransition = (toStatus == "in_progress" || toStatus == "completed" || toStatus == "cancelled")
	case "in_progress":
		validTransition = (toStatus == "completed" || toStatus == "cancelled")
	}
	if !validTransition {
		return Booking{}, ErrBookingTransition
	}

	_, err = tx.Exec(ctx, `UPDATE bookings
SET status=$1, cancellation_reason=CASE WHEN $1='cancelled' THEN $2 ELSE cancellation_reason END,
    notes=CASE WHEN $3='' THEN notes WHEN notes='' THEN $3 ELSE notes || ' | ' || $3 END,
    updated_at=now()
WHERE organization_id=$4 AND location_id=$5 AND id=$6`,
		toStatus, cancellationReason, notes, scope.OrganizationID, scope.LocationID, id)
	if err != nil {
		return Booking{}, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO booking_events(booking_id, organization_id, location_id, actor_id, from_status, to_status, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, scope.OrganizationID, scope.LocationID, scope.ActorID, currentStatus, toStatus, notes)
	if err != nil {
		return Booking{}, err
	}

	row := tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b
JOIN customers c ON c.organization_id=b.organization_id AND c.location_id=b.location_id AND c.id=b.customer_id
WHERE b.id=$1`, id)
	updated, err := scanBooking(row)
	if err != nil {
		return Booking{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	return updated, nil
}
