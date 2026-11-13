package postgres

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// The business details every email signs off with.
const (
	CompanyName    = "Dapper Detailing Pros"
	CompanyPhone   = "+1 (705) 977-6217"
	CompanyWebsite = "https://dapperdetailing.ca"
	CompanyEmail   = "info@dapperdetailing.ca"
	ReviewLink     = "https://share.google/avpHywVq7AkU0QNas"
)

var emailZone = func() *time.Location {
	if z, err := time.LoadLocation("America/Toronto"); err == nil {
		return z
	}
	return time.UTC
}()

// CompanyVars are the placeholders every email can use for the business itself; origin is the app's address (for the logo).
func CompanyVars(origin string) map[string]string {
	return map[string]string{"company_name": CompanyName, "company_phone": CompanyPhone, "company_website": CompanyWebsite, "company_email": CompanyEmail,
		"logo_url": strings.TrimRight(origin, "/") + "/icon-192.png", "review_link": ReviewLink, "year": strconv.Itoa(time.Now().In(emailZone).Year())}
}

var bookingStatusText = map[string]string{"pending": "PENDING", "confirmed": "APPROVED", "completed": "COMPLETED", "cancelled": "CANCELLED", "no_show": "NO SHOW"}

// BookingEmailVars is everything a booking email can say about the booking, keyed by placeholder name, and the customer's saved address
// to send it to. The figures come from the booking's latest live invoice; with none yet the money fields are empty.
func (s *Store) BookingEmailVars(ctx context.Context, bookingID, origin string) (map[string]string, string, error) {
	var (
		name, emailTo, phone, service, address, shop, status, extras string
		number                                                       *int64
		when                                                         time.Time
	)
	err := s.pool.QueryRow(ctx, `SELECT c.name,c.email,c.phone,b.service_name,b.booking_time,coalesce(b.service_address,''),b.status,b.number,
 coalesce((SELECT string_agg(a->>'name', ', ') FROM jsonb_array_elements(CASE WHEN jsonb_typeof(b.addons)='array' THEN b.addons ELSE '[]'::jsonb END) a),''),
 coalesce((SELECT coalesce(NULLIF(ii.address,''),concat_ws(', ',NULLIF(l.address_street,''),NULLIF(l.address_city,''),NULLIF(l.address_region,''),NULLIF(l.address_postal_code,'')))
   FROM locations l LEFT JOIN location_invoice_issuers li ON li.organization_id=l.organization_id AND li.location_id=l.id LEFT JOIN invoice_issuers ii ON ii.id=li.issuer_id
   WHERE l.organization_id=b.organization_id AND l.id=b.location_id),'')
 FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id::text=$1`, bookingID).
		Scan(&name, &emailTo, &phone, &service, &when, &address, &status, &number, &extras, &shop)
	if err != nil {
		return nil, "", err
	}
	v := CompanyVars(origin)
	first, last, _ := strings.Cut(strings.TrimSpace(name), " ")
	local := when.In(emailZone)
	v["name"], v["first_name"], v["last_name"], v["email"], v["phone"] = name, first, strings.TrimSpace(last), emailTo, phone
	v["booking_id"] = "#" + bookingID[:8]
	if number != nil {
		v["booking_id"] = "#" + strconv.FormatInt(*number, 10)
	}
	v["service"], v["extras"] = service, extras
	if extras == "" {
		v["extras"] = "None"
	}
	v["when"], v["date"], v["time"] = local.Format("Monday, January 2 at 3:04 PM"), local.Format("Monday, January 2, 2006"), local.Format("3:04 PM")
	// Where the work happens: the customer's address for a mobile booking, else the shop's address.
	if strings.TrimSpace(address) != "" {
		v["location"], v["address"] = "Mobile service", address
	} else {
		v["location"], v["address"] = "At our shop", shop
	}
	v["where"] = v["location"] + " - " + v["address"]
	if v["address"] == "" {
		v["where"] = v["location"]
	}
	v["status"] = bookingStatusText[status]
	if v["status"] == "" {
		v["status"] = strings.ToUpper(status)
	}
	v["total"], v["payment"] = s.BookingPaymentText(ctx, bookingID)
	var discount, tax, paid, due, refunded int64
	var coupon string
	if s.pool.QueryRow(ctx, `SELECT coalesce(i.coupon_code,''),i.discount_cents,i.tax_cents,i.paid_cents,i.due_cents,
 coalesce((SELECT sum(p.refunded_cents) FROM payments p WHERE p.invoice_id=i.id),0)
 FROM invoices i WHERE i.booking_id::text=$1 AND i.status<>'void' ORDER BY i.created_at DESC LIMIT 1`, bookingID).Scan(&coupon, &discount, &tax, &paid, &due, &refunded) == nil {
		v["discount"], v["tax"], v["paid"], v["due"], v["refund"] = emailMoney(discount), emailMoney(tax), emailMoney(paid), emailMoney(due), emailMoney(refunded)
		v["coupon"] = coupon
	}
	if v["coupon"] == "" {
		v["coupon"] = "None"
	}
	for _, k := range []string{"discount", "tax", "paid", "due", "refund"} {
		if v[k] == "" {
			v[k] = "$0.00"
		}
	}
	return v, emailTo, nil
}
