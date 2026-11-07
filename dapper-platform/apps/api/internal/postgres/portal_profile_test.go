package postgres

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"
)

// verifyPortalProfile: a customer keeps several saved addresses (one default, mirrored to the customer record) and one profile picture, all
// only through their own account link.
func verifyPortalProfile(t *testing.T, ctx context.Context, s *Store, actor, customer, foreign string) {
	t.Helper()
	first, err := s.SavePortalAddress(ctx, actor, customer, PortalAddress{Label: " Home ", Address: " 1 Synthetic Road, Peterborough "})
	if err != nil || !first.IsDefault || first.Label != "Home" || first.Address != "1 Synthetic Road, Peterborough" {
		t.Fatal("first saved address is not the trimmed default", first, err)
	}
	second, err := s.SavePortalAddress(ctx, actor, customer, PortalAddress{Label: "Work", Address: "2 Synthetic Avenue, Peterborough"})
	if err != nil || second.IsDefault {
		t.Fatal("second address took over the default", second, err)
	}
	second.IsDefault = true
	if second, err = s.SavePortalAddress(ctx, actor, customer, second); err != nil || !second.IsDefault {
		t.Fatal("default address could not be changed", second, err)
	}
	list, err := s.PortalAddresses(ctx, actor, customer)
	if err != nil || len(list) != 2 || list[0].ID != second.ID || !list[0].IsDefault || list[1].IsDefault {
		t.Fatal("saved addresses are not listed default first with one default", list, err)
	}
	records, err := s.CustomerRecords(ctx, actor, customer)
	if err != nil || records.Account.Address != second.Address || len(records.Addresses) != 2 || records.HasAvatar {
		t.Fatalf("records do not carry the default address: %+v %v", records.Account, err)
	}
	if _, err = s.SavePortalAddress(ctx, actor, foreign, PortalAddress{Label: "Intruder", Address: "9 Synthetic Street, Nowhere"}); !errors.Is(err, ErrForbidden) {
		t.Fatal("an address was saved on a record that is not linked", err)
	}
	if _, err = s.PortalAddresses(ctx, actor, foreign); !errors.Is(err, ErrForbidden) {
		t.Fatal("addresses of an unlinked customer were readable", err)
	}
	if _, err = s.SavePortalAddress(ctx, actor, customer, PortalAddress{Label: "", Address: "x"}); !errors.Is(err, ErrPortalAddress) {
		t.Fatal("an empty address was accepted", err)
	}
	if err = s.DeletePortalAddress(ctx, actor, customer, second.ID); err != nil {
		t.Fatal(err)
	}
	if list, err = s.PortalAddresses(ctx, actor, customer); err != nil || len(list) != 1 || !list[0].IsDefault || list[0].ID != first.ID {
		t.Fatal("the remaining address did not become the default", list, err)
	}
	if records, err = s.CustomerRecords(ctx, actor, customer); err != nil || records.Account.Address != first.Address {
		t.Fatal("record address did not follow the new default", records.Account.Address, err)
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if err = s.SetPortalAvatar(ctx, actor, customer, []byte("not a picture")); !errors.Is(err, ErrPortalAvatar) {
		t.Fatal("a non-image avatar was accepted", err)
	}
	if err = s.SetPortalAvatar(ctx, actor, foreign, encoded.Bytes()); !errors.Is(err, ErrForbidden) {
		t.Fatal("an avatar was set on an unlinked customer", err)
	}
	if err = s.SetPortalAvatar(ctx, actor, customer, encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, mime, e := s.PortalAvatar(ctx, actor, customer); e != nil || mime != "image/png" {
		t.Fatal("avatar not readable", e)
	}
	if _, _, e := s.PortalAvatar(ctx, actor, foreign); !errors.Is(e, ErrForbidden) {
		t.Fatal("avatar of an unlinked customer was readable", e)
	}
	if records, err = s.CustomerRecords(ctx, actor, customer); err != nil || !records.HasAvatar {
		t.Fatal("records do not show the avatar", err)
	}
	if err = s.DeletePortalAvatar(ctx, actor, customer); err != nil {
		t.Fatal(err)
	}
	if _, _, e := s.PortalAvatar(ctx, actor, customer); !errors.Is(e, ErrCustomerNotFound) {
		t.Fatal("avatar still readable after removal", e)
	}
}
