package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"dapper.local/platform/internal/scheduling"
	"github.com/jackc/pgx/v5"
)

var ErrEmployeeContact = errors.New("provide a full name, valid email and international phone number")
var ErrEmployeeLocalTime = errors.New("choose a valid unambiguous future time in the assigned worksite timezone")
var employeePhone = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
var employeeDomain = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+$`)
var employeeRequestKey = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{16,96}$`)

type EmployeeWalkInInput struct {
	RequestKey  string `json:"requestKey"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	RuleID      string `json:"ruleId"`
	RuleVersion int    `json:"ruleVersion"`
	ResourceID  string `json:"resourceId"`
	LocalStart  string `json:"localStart"`
	// AddonIDs attach add-ons to a base rule. Add-ons do not change the appointment length (the legacy catalogue gave them no time).
	AddonIDs []string `json:"addonIds"`
	// Notes carry the vehicle, where the work happens, any referral and the customer's own notes (shown to staff on the booking).
	Notes string `json:"notes"`
}

func normalizeEmployeeContact(name, email, phone string) (string, string, string, error) {
	name, email, phone = strings.TrimSpace(name), strings.TrimSpace(email), strings.TrimSpace(phone)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 || len(email) > 254 || len(phone) > 40 {
		return "", "", "", ErrEmployeeContact
	}
	for _, c := range name {
		if c < 32 || c == 127 {
			return "", "", "", ErrEmployeeContact
		}
	}
	address, err := mail.ParseAddress(email)
	at := strings.LastIndexByte(email, '@')
	if err != nil || address.Address != email || address.Name != "" || at < 1 || !employeeDomain.MatchString(email[at+1:]) || strings.ContainsAny(email, "\r\n\t ") {
		return "", "", "", ErrEmployeeContact
	}
	email = email[:at+1] + strings.ToLower(email[at+1:])
	var out strings.Builder
	for _, c := range phone {
		switch {
		case c == '+' || c >= '0' && c <= '9':
			out.WriteRune(c)
		case c == ' ' || c == '(' || c == ')' || c == '-' || c == '.':
		default:
			return "", "", "", ErrEmployeeContact
		}
	}
	phone = out.String()
	if !employeePhone.MatchString(phone) {
		return "", "", "", ErrEmployeeContact
	}
	return name, email, phone, nil
}
func employeeLocalStart(value, zone string) (time.Time, error) {
	z, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, ErrEmployeeLocalTime
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", value, z)
	if err != nil || t.Format("2006-01-02T15:04") != value {
		return time.Time{}, ErrEmployeeLocalTime
	}
	// Refuse repeated civil times rather than silently choosing a DST occurrence.
	for _, d := range []time.Duration{-2 * time.Hour, -time.Hour, -30 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour} {
		if t.Add(d).In(z).Format("2006-01-02T15:04") == value {
			return time.Time{}, ErrEmployeeLocalTime
		}
	}
	return t.UTC(), nil
}

func (s *Store) EmployeeWalkInOptions(ctx context.Context, actor string) (BookingCapacityOptions, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return BookingCapacityOptions{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return BookingCapacityOptions{}, err
	}
	if err = authorizeTransaction(ctx, tx, employeeScope(a), "customers.create"); err != nil {
		return BookingCapacityOptions{}, err
	}
	choices, err := s.BookingCapacityChoices(ctx, employeeScope(a))
	if err != nil {
		return choices, err
	}
	return choices, tx.Commit(ctx)
}

// One transaction owns customer, allocation, pending booking, dealership link,
// audit and retry receipt. Contact syntax is checked; ownership is not verified.
func (s *Store) CreateEmployeeWalkIn(ctx context.Context, actor string, input EmployeeWalkInInput) (Booking, error) {
	var err error
	input.Name, input.Email, input.Phone, err = normalizeEmployeeContact(input.Name, input.Email, input.Phone)
	if err != nil {
		return Booking{}, err
	}
	input.Notes = strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(input.Notes) > 2000 {
		return Booking{}, ErrBookingInput
	}
	if !employeeRequestKey.MatchString(input.RequestKey) || input.RuleID == "" || input.ResourceID == "" || input.RuleVersion < 1 {
		return Booking{}, ErrBookingInput
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)
	a, err := employeeWorksite(ctx, tx, actor)
	if err != nil {
		return Booking{}, err
	}
	scope := employeeScope(a)
	for _, action := range []string{"customers.create", "bookings.create", "scheduling.reserve"} {
		if err = authorizeTransaction(ctx, tx, scope, action); err != nil {
			return Booking{}, err
		}
	}
	start, err := employeeLocalStart(input.LocalStart, a.Timezone)
	if err != nil {
		return Booking{}, err
	}
	encoded, _ := json.Marshal(struct {
		Kind, Dealer string
		Input        EmployeeWalkInInput
	}{"employee.walk_in", a.DealershipID, input})
	hash := sha256.Sum256(encoded)
	key := "employee.walkin:" + input.RequestKey
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.OrganizationID+"/"+a.LocationID+"/booking-capacity/"+key); err != nil {
		return Booking{}, err
	}
	var existing, existingActor string
	var previous []byte
	err = tx.QueryRow(ctx, `SELECT booking_id::text,actor_id::text,request_hash FROM booking_capacity_requests WHERE organization_id=$1 AND location_id=$2 AND request_key=$3`, a.OrganizationID, a.LocationID, key).Scan(&existing, &existingActor, &previous)
	if err == nil {
		if existingActor != actor || string(previous) != string(hash[:]) {
			return Booking{}, ErrBookingCapacityRequest
		}
		if err = employeeBookingLock(ctx, tx, a, existing); err != nil {
			return Booking{}, err
		}
		b, e := scanBooking(tx.QueryRow(ctx, `SELECT `+bookingColumns+` FROM bookings b JOIN customers c ON (c.organization_id,c.location_id,c.id)=(b.organization_id,b.location_id,b.customer_id) WHERE b.id=$1`, existing))
		if e != nil {
			return Booking{}, e
		}
		return bookingMutationSummary(b), tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, err
	}
	if !start.After(time.Now()) {
		return Booking{}, ErrEmployeeLocalTime
	}
	reservation, err := s.writeReservationInTransaction(ctx, tx, scope, input.ResourceID, "", 0, scheduling.Request{Start: start}, &reservationRuleChoice{ID: input.RuleID, Version: input.RuleVersion})
	if err != nil {
		return Booking{}, err
	}
	if reservation.Rule == nil {
		return Booking{}, ErrSchedulingRule
	}
	var customer string
	if err = tx.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email,phone) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, a.OrganizationID, a.LocationID, input.Name, input.Email, input.Phone).Scan(&customer); err != nil {
		return Booking{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id) VALUES($1,$2,$3,'customer.created',$4)`, a.OrganizationID, a.LocationID, actor, customer); err != nil {
		return Booking{}, err
	}
	rule := reservation.Rule
	addons := rule.AddonIDs
	if len(input.AddonIDs) > 0 {
		if len(rule.AddonIDs) > 0 || len(input.AddonIDs) > 20 {
			return Booking{}, ErrBookingInput
		}
		seen := map[string]bool{}
		for _, id := range input.AddonIDs {
			var ok bool
			if seen[id] || tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_services WHERE organization_id=$1 AND location_id=$2 AND id::text=$3 AND kind='addon' AND parent_id::text=$4 AND NOT archived)`, a.OrganizationID, a.LocationID, id, rule.ServiceID).Scan(&ok) != nil || !ok {
				return Booking{}, ErrBookingInput
			}
			seen[id] = true
		}
		addons = append([]string{}, input.AddonIDs...)
	}
	b, err := s.createBookingInTransaction(ctx, tx, scope, Booking{CustomerID: customer, ServiceID: &rule.ServiceID, AddonIDs: addons, ReservationID: &reservation.ID, Channel: "walk_in", Status: "pending", BookingTime: start, Notes: input.Notes})
	if err != nil {
		return Booking{}, bookingWriteError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO employee_booking_worksites(booking_id,organization_id,location_id,dealership_id,linked_by) VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5)`, b.ID, a.OrganizationID, a.LocationID, a.DealershipID, actor); err != nil {
		return Booking{}, err
	}
	if err = employeeEvent(ctx, tx, actor, a.OrganizationID, b.ID, "employee.walk_in_created", map[string]string{"dealershipId": a.DealershipID}); err != nil {
		return Booking{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO booking_capacity_requests(organization_id,location_id,request_key,request_hash,actor_id,booking_id) VALUES($1,$2,$3,$4,$5,$6)`, a.OrganizationID, a.LocationID, key, hash[:], actor, b.ID); err != nil {
		return Booking{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, bookingWriteError(err)
	}
	return bookingMutationSummary(b), nil
}
