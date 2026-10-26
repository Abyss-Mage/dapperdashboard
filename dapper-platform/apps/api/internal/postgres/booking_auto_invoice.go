package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"log"
	"strings"
)

// autoInvoiceDraft raises a draft invoice the moment a booking is created, so billing never depends on someone remembering to do it.
// The lines are the service and its extras at their catalogue prices (a missing price is billed as 0.00 for the team to fill in before
// issuing), plus the franchise's active taxes. It runs inside its own savepoint: if anything about it fails (for example prices that
// include tax, which invoices do not support yet) the booking is still created and the invoice can be made by hand as before.
// autoInvoiceEnabled is on in the application; tests that build their own invoices for bookings turn it off.
var autoInvoiceEnabled = true

func (s *Store) autoInvoiceDraft(ctx context.Context, tx pgx.Tx, scope Scope, b Booking) {
	if !autoInvoiceEnabled || b.ID == "" || b.Status == "cancelled" {
		return
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return
	}
	defer sp.Rollback(ctx)
	priceOf := func(id *string) int64 {
		if id == nil {
			return 0
		}
		var p int64
		_ = sp.QueryRow(ctx, `SELECT COALESCE((SELECT price_cents FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id=$3),(SELECT price_cents FROM central_catalog_services WHERE organization_id=$1 AND id=$3),0)`,
			scope.OrganizationID, scope.LocationID, *id).Scan(&p)
		return p
	}
	type line struct {
		desc  string
		price int64
	}
	lines := []line{{strings.TrimSpace(b.ServiceName), priceOf(b.ServiceID)}}
	for _, a := range b.Addons {
		id := a.ServiceID
		lines = append(lines, line{strings.TrimSpace(a.Name), priceOf(&id)})
	}
	var subtotal int64
	for _, l := range lines {
		subtotal += l.price
	}
	tax, _, err := s.taxOnSubtotal(ctx, scope, subtotal)
	if err != nil {
		if !errors.Is(err, ErrTaxInclusiveUnsupported) {
			log.Print("Automatic invoice draft skipped: tax lookup failed")
		}
		return
	}
	number, err := generateDefaultInvoiceNumber()
	if err != nil {
		return
	}
	var id string
	if err = sp.QueryRow(ctx, `INSERT INTO invoices(organization_id,location_id,customer_id,booking_id,invoice_number,status,currency,subtotal_cents,tax_cents,total_cents,paid_cents,due_cents,notes,issued_at,updated_at)
VALUES($1,$2,$3,$4,$5,'draft','CAD',$6,$7,$8,0,$8,$9,NULL,now()) RETURNING id::text`,
		scope.OrganizationID, scope.LocationID, b.CustomerID, b.ID, number, subtotal, tax, subtotal+tax, "Created automatically with the booking. Check the prices before issuing.").Scan(&id); err != nil {
		return
	}
	for _, l := range lines {
		if _, err = sp.Exec(ctx, `INSERT INTO invoice_items(invoice_id,organization_id,location_id,description,quantity,unit_price_cents,total_price_cents) VALUES($1,$2,$3,$4,1,$5,$5)`,
			id, scope.OrganizationID, scope.LocationID, l.desc, l.price); err != nil {
			return
		}
	}
	if _, err = sp.Exec(ctx, `INSERT INTO invoice_events(invoice_id,organization_id,location_id,actor_id,from_status,to_status,notes) VALUES($1,$2,$3,$4,NULL,'draft','Invoice draft created automatically with the booking')`,
		id, scope.OrganizationID, scope.LocationID, scope.ActorID); err != nil {
		return
	}
	_ = sp.Commit(ctx)
}
