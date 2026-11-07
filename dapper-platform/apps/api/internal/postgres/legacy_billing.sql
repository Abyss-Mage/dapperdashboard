-- Bookings imported from the old BookingPress site keep their billing as one small row of the figures that site recorded.
-- Their invoice is built from this row when it is opened: no invoice, payment or event rows are stored for them, and
-- nothing here counts toward reports (the old payments were never recorded as payments here).
CREATE TABLE legacy_booking_billing (
 booking_id uuid PRIMARY KEY REFERENCES bookings(id),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 source_id text NOT NULL,
 invoice_number text NOT NULL CHECK(length(btrim(invoice_number)) BETWEEN 3 AND 60),
 status text NOT NULL CHECK(status IN ('paid','partially_paid','void')),
 items jsonb NOT NULL CHECK(jsonb_typeof(items)='array'),
 subtotal_cents bigint NOT NULL CHECK(subtotal_cents >= 0),
 discount_cents bigint NOT NULL DEFAULT 0 CHECK(discount_cents >= 0),
 coupon_code text NOT NULL DEFAULT '' CHECK(length(coupon_code) <= 32),
 tax_cents bigint NOT NULL CHECK(tax_cents >= 0),
 total_cents bigint NOT NULL CHECK(total_cents >= 0),
 paid_cents bigint NOT NULL DEFAULT 0 CHECK(paid_cents >= 0 AND paid_cents <= total_cents),
 payment_method text NOT NULL DEFAULT '' CHECK(payment_method IN ('','cash','square_card')),
 gateway text NOT NULL DEFAULT '',
 note text NOT NULL DEFAULT '',
 UNIQUE(organization_id,invoice_number),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
