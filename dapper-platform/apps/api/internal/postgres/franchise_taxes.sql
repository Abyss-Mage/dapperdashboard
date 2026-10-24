-- Taxes are configured per franchise (site). Prices are entered tax-exclusive by default; tax is added separately.
-- Rates are stored in basis points (1300 = 13.00 percent). Nothing is pre-filled: a franchise has no tax until someone adds one.
CREATE TABLE franchise_tax_rates (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 60),
 rate_bp integer NOT NULL CHECK(rate_bp BETWEEN 0 AND 5000),
 active boolean NOT NULL DEFAULT true,
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE UNIQUE INDEX franchise_tax_rates_name ON franchise_tax_rates(organization_id,location_id,lower(btrim(name)));
CREATE TABLE franchise_tax_settings (
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 prices_include_tax boolean NOT NULL DEFAULT false,
 rounding text CHECK(rounding IS NULL OR rounding IN ('half_up','half_even','down')),
 version integer NOT NULL DEFAULT 1 CHECK(version>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,location_id),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE franchise_tax_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL CHECK(action IN ('rate_created','rate_updated','rate_deleted','settings_saved')),
 entity_id text NOT NULL, snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
