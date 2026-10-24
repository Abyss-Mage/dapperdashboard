package paymentstate

import (
	"errors"
	"testing"
)

func TestSettlementBinding(t *testing.T) {
	expected := SettlementBinding{Environment: "sandbox", OrganizationID: "synthetic-org", LocationID: "synthetic-location", MerchantID: "synthetic-merchant", ProviderLocationID: "synthetic-provider-location", ReferenceID: "synthetic-intent", Currency: "CAD", AmountCents: 5000}
	evidence := SettlementEvidence{SettlementBinding: expected, PaymentID: "synthetic-payment", Status: "COMPLETED"}
	if err := ValidateSettlement(expected, evidence); err != nil {
		t.Fatal(err)
	}
	mutations := []func(*SettlementEvidence){func(v *SettlementEvidence) { v.Environment = "production" }, func(v *SettlementEvidence) { v.OrganizationID = "other" }, func(v *SettlementEvidence) { v.LocationID = "other" }, func(v *SettlementEvidence) { v.MerchantID = "other" }, func(v *SettlementEvidence) { v.ProviderLocationID = "other" }, func(v *SettlementEvidence) { v.ReferenceID = "other" }, func(v *SettlementEvidence) { v.Currency = "USD" }, func(v *SettlementEvidence) { v.AmountCents++ }, func(v *SettlementEvidence) { v.PaymentID = "" }, func(v *SettlementEvidence) { v.Status = "APPROVED" }, func(v *SettlementEvidence) { v.Status = "PENDING" }}
	for index, mutate := range mutations {
		value := evidence
		mutate(&value)
		if !errors.Is(ValidateSettlement(expected, value), ErrSettlementBinding) {
			t.Fatalf("accepted mismatched evidence%d", index)
		}
	}
	for _, mutate := range []func(*SettlementBinding){func(v *SettlementBinding) { v.OrganizationID = "" }, func(v *SettlementBinding) { v.LocationID = "" }, func(v *SettlementBinding) { v.MerchantID = "" }, func(v *SettlementBinding) { v.ProviderLocationID = "" }, func(v *SettlementBinding) { v.ReferenceID = "" }, func(v *SettlementBinding) { v.Currency = "" }, func(v *SettlementBinding) { v.AmountCents = 0 }, func(v *SettlementBinding) { v.AmountCents = -1 }} {
		binding := expected
		mutate(&binding)
		matching := evidence
		matching.SettlementBinding = binding
		if ValidateSettlement(binding, matching) == nil {
			t.Fatal("accepted incomplete server binding")
		}
	}
}
