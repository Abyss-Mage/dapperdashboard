package postgres

import (
	"context"
	"errors"
	"testing"
)

func verifyCustomerAccountManagement(t *testing.T, ctx context.Context, s *Store, a, b Scope, customer string) {
	t.Helper()
	var target, foreignCustomer string
	if err := s.pool.QueryRow(ctx, `INSERT INTO principals(external_subject) VALUES('isolated-managed-customer-account') RETURNING id::text`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewPasswordAuthenticator(s)
	if err != nil {
		t.Fatal(err)
	}
	if err = authenticator.SetPassword(ctx, target, "owned-account@example.invalid", "Isolated ownership account password"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ManagedCustomerAccounts(ctx, a); !errors.Is(err, ErrForbidden) {
		t.Fatal("customer ownership authority implicit", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES($1,$2,$3,'customer_accounts.manage')`, a.ActorID, a.OrganizationID, a.LocationID); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, true, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("location-only authority could link customer", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO organization_grants(principal_id,organization_id,action) VALUES($1,$2,'access.manage') ON CONFLICT DO NOTHING`, a.ActorID, a.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, true, false); !errors.Is(err, ErrCustomerAccountReview) {
		t.Fatal("link skipped explicit ownership review", err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, true, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("nonmember account linked by selector", err)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO organization_memberships(organization_id,principal_id) VALUES($1,$2)`, a.OrganizationID, target); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `INSERT INTO customers(organization_id,location_id,name,email) VALUES($1,$2,'Isolated foreign linked customer','owned-account@example.invalid') RETURNING id::text`, b.OrganizationID, b.LocationID).Scan(&foreignCustomer); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, foreignCustomer, target, true, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign customer linked", err)
	}
	if _, err = s.CustomerRecords(ctx, target, customer); !errors.Is(err, ErrForbidden) {
		t.Fatal("email/membership inferred customer ownership", err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- s.SetCustomerAccountLink(ctx, a, customer, target, true, true) }()
	}
	for i := 0; i < 2; i++ {
		if err = <-results; err != nil {
			t.Fatal("concurrent idempotent link failed", err)
		}
	}
	records, err := s.CustomerRecords(ctx, target, customer)
	if err != nil || records.Account.CustomerID != customer {
		t.Fatal("explicit ownership not effective", err)
	}
	if _, err = s.CustomerRecords(ctx, target, foreignCustomer); !errors.Is(err, ErrForbidden) {
		t.Fatal("new link exposed foreign matching-email customer", err)
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM customer_account_events WHERE customer_id=$1 AND principal_id=$2 AND enabled`, customer, target).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate customer link audit", err)
	}
	management, err := s.ManagedCustomerAccounts(ctx, a)
	if err != nil || len(management.Links) < 1 {
		t.Fatal("authorized customer link read failed", err)
	}
	if _, err = s.ManagedCustomerAccounts(ctx, b); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign customer link metadata exposed", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE customer_account_events ADD CONSTRAINT isolated_link_audit_failure CHECK(enabled) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, false, false); err == nil {
		t.Fatal("link revoke audit failure committed")
	}
	if _, err = s.CustomerRecords(ctx, target, customer); err != nil {
		t.Fatal("failed revoke removed portal ownership", err)
	}
	if _, err = s.pool.Exec(ctx, `ALTER TABLE customer_account_events DROP CONSTRAINT isolated_link_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM organization_memberships WHERE organization_id=$1 AND principal_id=$2`, a.OrganizationID, target); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE principals SET active=false WHERE id=$1`, target); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, false, false); err != nil {
		t.Fatal("cannot revoke after target disabled/membership removed", err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, false, false); err != nil {
		t.Fatal("idempotent revoke failed", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM customer_account_links WHERE customer_id=$1 AND principal_id=$2`, customer, target).Scan(&count); err != nil || count != 0 {
		t.Fatal("disabled target retained link", err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM customer_account_events WHERE customer_id=$1 AND principal_id=$2 AND NOT enabled`, customer, target).Scan(&count); err != nil || count != 1 {
		t.Fatal("revocation audit duplicated or missing", err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, true, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("disabled/nonmember target relinked", err)
	}
	if _, err = s.pool.Exec(ctx, `DELETE FROM location_grants WHERE principal_id=$1 AND action='customer_accounts.manage'`, a.ActorID); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCustomerAccountLink(ctx, a, customer, target, false, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked operator retained ownership control", err)
	}
}
