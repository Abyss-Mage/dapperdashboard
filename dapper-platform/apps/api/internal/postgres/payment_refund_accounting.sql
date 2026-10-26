-- Candidate migration: register only after coordinated version allocation.
-- Existing refunded rows cannot safely reveal partial/cumulative refund totals.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM payments WHERE status='refunded') THEN
  RAISE EXCEPTION 'Reviewed historical refund reconciliation required before migration';
 END IF;
END $$;
ALTER TABLE payments ADD COLUMN refunded_cents bigint NOT NULL DEFAULT 0
 CHECK(refunded_cents>=0 AND refunded_cents<=amount_cents);
ALTER TABLE payments ADD CONSTRAINT payments_scope_identity UNIQUE(organization_id,location_id,id,amount_cents);
CREATE TABLE payment_refund_adjustments (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,location_id uuid NOT NULL,payment_id uuid NOT NULL,
 captured_cents bigint NOT NULL CHECK(captured_cents>0),
 previous_refunded_cents bigint NOT NULL CHECK(previous_refunded_cents>=0),
 refunded_cents bigint NOT NULL CHECK(refunded_cents<=captured_cents),
 delta_cents bigint NOT NULL CHECK(delta_cents>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(refunded_cents=previous_refunded_cents+delta_cents),
 FOREIGN KEY(organization_id,location_id,payment_id,captured_cents) REFERENCES payments(organization_id,location_id,id,amount_cents),
 UNIQUE(payment_id,refunded_cents)
);
CREATE FUNCTION protect_payment_refund_adjustment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Refund adjustment history is immutable' USING ERRCODE='23514'; END $$;
CREATE TRIGGER payment_refund_adjustment_immutable BEFORE UPDATE OR DELETE ON payment_refund_adjustments FOR EACH ROW EXECUTE FUNCTION protect_payment_refund_adjustment();

