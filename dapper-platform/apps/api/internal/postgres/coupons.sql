-- Discount coupons for a franchise. A coupon can be limited to a period, one customer, some services and a number of uses.
-- An invoice records the discount it was given (discount_cents, coupon_code); the invoice total is subtotal minus discount plus tax.
CREATE TABLE coupons (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 title text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 100),
 code text NOT NULL CHECK (code ~ '^[A-Z0-9-]{3,32}$'),
 kind text NOT NULL CHECK (kind IN ('percent','fixed')),
 value integer NOT NULL CHECK (value > 0),
 period text NOT NULL CHECK (period IN ('range','never')),
 starts_on date,
 ends_on date,
 customer_id uuid,
 max_uses integer NOT NULL DEFAULT 0 CHECK (max_uses >= 0),
 active boolean NOT NULL DEFAULT true,
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (organization_id, location_id, code),
 CHECK (kind <> 'percent' OR value <= 100),
 CHECK (period <> 'range' OR (starts_on IS NOT NULL AND ends_on IS NOT NULL AND starts_on <= ends_on)),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id),
 FOREIGN KEY (organization_id, location_id, customer_id) REFERENCES customers(organization_id, location_id, id)
);
CREATE TABLE coupon_services (
 coupon_id uuid NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
 service_id uuid NOT NULL,
 PRIMARY KEY (coupon_id, service_id)
);
CREATE TABLE coupon_redemptions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 coupon_id uuid NOT NULL REFERENCES coupons(id),
 invoice_id uuid NOT NULL UNIQUE REFERENCES invoices(id),
 customer_id uuid NOT NULL,
 amount_cents bigint NOT NULL CHECK (amount_cents > 0),
 redeemed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coupon_redemptions_coupon_idx ON coupon_redemptions(coupon_id);
ALTER TABLE invoices ADD COLUMN discount_cents bigint NOT NULL DEFAULT 0 CHECK (discount_cents >= 0);
ALTER TABLE invoices ADD COLUMN coupon_code text NOT NULL DEFAULT '' CHECK (length(coupon_code) <= 32);
