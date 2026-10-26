package paymentstate

import "errors"

var ErrSettlementBinding = errors.New("settlement does not match reviewed server payment binding")

// SettlementBinding must come from a scoped persisted server payment intent,
// never from customer/staff request fields. Merchant identity for a charge
// response must be independently bound to the credential used for that call.
type SettlementBinding struct {
	Environment        string
	OrganizationID     string
	LocationID         string
	MerchantID         string
	ProviderLocationID string
	ReferenceID        string
	Currency           string
	AmountCents        int64
}

// SettlementEvidence must be resolved by the trusted provider adapter: an
// authenticated charge lookup or signature-verified, merchant-bound event.
// This validator alone does not authenticate a caller or a provider payload.
type SettlementEvidence struct {
	SettlementBinding
	PaymentID string
	Status    string
}

func ValidateSettlement(expected SettlementBinding, observed SettlementEvidence) error {
	if (expected.Environment != "sandbox" && expected.Environment != "production") || expected.OrganizationID == "" || expected.LocationID == "" || expected.MerchantID == "" || expected.ProviderLocationID == "" || expected.ReferenceID == "" || expected.Currency == "" || expected.AmountCents <= 0 {
		return ErrSettlementBinding
	}
	if observed.Status != "COMPLETED" || observed.PaymentID == "" || observed.SettlementBinding != expected {
		return ErrSettlementBinding
	}
	return nil
}
