package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type reservationCursor struct {
	From, To, Start time.Time
	ID              string
}
type ReservationPage struct {
	Items      []CapacityReservation
	NextCursor string
}

// Reservations reads occupied-window overlaps, including retained cancellations.
// Cursor ordering is stable by stored occupied start/id, never a client count.
func (s *Store) Reservations(ctx context.Context, scope Scope, from, to time.Time, cursor string, limit int) (ReservationPage, error) {
	if !exactTime(from) || !exactTime(to) || from.IsZero() || !from.Before(to) || to.Sub(from) > 32*24*time.Hour || limit < 1 || limit > 100 || len(cursor) > 2048 {
		return ReservationPage{}, ErrReservationInput
	}
	var after any
	afterID := "00000000-0000-0000-0000-000000000000"
	if cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return ReservationPage{}, ErrReservationInput
		}
		var c reservationCursor
		if json.Unmarshal(data, &c) != nil || !c.From.Equal(from) || !c.To.Equal(to) || !exactTime(c.Start) || c.Start.IsZero() {
			return ReservationPage{}, ErrReservationInput
		}
		var id pgtype.UUID
		if id.Scan(c.ID) != nil || !id.Valid {
			return ReservationPage{}, ErrReservationInput
		}
		after = c.Start
		afterID = c.ID
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReservationPage{}, err
	}
	defer tx.Rollback(ctx)
	if err = authorizeTransaction(ctx, tx, scope, "scheduling.read"); err != nil {
		return ReservationPage{}, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,resource_id::text,request,occupied_start,occupied_end,version,cancelled_at,cancelled_by::text,rule_snapshot FROM capacity_reservations WHERE organization_id=$1 AND location_id=$2 AND occupied_start<$4 AND occupied_end>$3 AND ($5::timestamptz IS NULL OR (occupied_start,id)>($5,$6::uuid)) ORDER BY occupied_start,id LIMIT $7`, scope.OrganizationID, scope.LocationID, from, to, after, afterID, limit+1)
	if err != nil {
		return ReservationPage{}, err
	}
	items := []CapacityReservation{}
	for rows.Next() {
		var item CapacityReservation
		if err = rows.Scan(&item.ID, &item.ResourceID, &item.Request, &item.Occupied.Start, &item.Occupied.End, &item.Version, &item.CancelledAt, &item.CancelledBy, &item.Rule); err != nil {
			rows.Close()
			return ReservationPage{}, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ReservationPage{}, err
	}
	page := ReservationPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		encoded, err := json.Marshal(reservationCursor{from, to, last.Occupied.Start, last.ID})
		if err != nil {
			return ReservationPage{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	if err = tx.Commit(ctx); err != nil {
		return ReservationPage{}, err
	}
	return page, nil
}
