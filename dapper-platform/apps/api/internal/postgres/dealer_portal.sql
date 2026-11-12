-- The dealership portal (error404 #25–#32, owner decisions 2026-11-11).
-- #25 "separate number lists": dealership invoices and customer invoices are numbered apart, so a number may exist once in each list.
ALTER TABLE invoices DROP CONSTRAINT invoices_organization_id_invoice_number_key;
CREATE UNIQUE INDEX invoices_customer_number_key ON invoices(organization_id, invoice_number) WHERE dealership_account_id IS NULL;
CREATE UNIQUE INDEX invoices_dealer_number_key ON invoices(organization_id, invoice_number) WHERE dealership_account_id IS NOT NULL;

-- #26/#30 portal access "login + email link": the owner gives each dealership contact an email and password; the link in an invoice email
-- opens the dealership's portal for 30 days without the password. Only hashes are kept.
CREATE TABLE dealer_portal_users (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_account_id uuid NOT NULL REFERENCES dealership_accounts(id),
 name text NOT NULL CHECK (length(btrim(name)) BETWEEN 2 AND 120),
 email text NOT NULL UNIQUE CHECK (email = lower(email) AND email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$'),
 password_hash text NOT NULL,
 created_by uuid NOT NULL REFERENCES principals(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
CREATE TABLE dealer_portal_sessions (
 token_hash bytea PRIMARY KEY,
 dealership_account_id uuid NOT NULL REFERENCES dealership_accounts(id),
 user_id uuid REFERENCES dealer_portal_users(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 revoked_at timestamptz
);

-- #27/#32 saved cards: the dealership saves cards with Square (card on file) and pays with them; Dapper never charges one on its own.
-- Only Square's ids and what the card shows (brand, last 4, expiry) are kept, never the card number.
CREATE TABLE dealer_saved_cards (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 dealership_account_id uuid NOT NULL REFERENCES dealership_accounts(id),
 environment text NOT NULL CHECK (environment IN ('sandbox','production')),
 square_customer_id text NOT NULL,
 square_card_id text NOT NULL,
 brand text NOT NULL DEFAULT '',
 last4 text NOT NULL DEFAULT '' CHECK (last4 ~ '^[0-9]{0,4}$'),
 exp_month integer,
 exp_year integer,
 is_default boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 removed_at timestamptz,
 UNIQUE (square_card_id)
);
CREATE UNIQUE INDEX dealer_saved_cards_one_default ON dealer_saved_cards(dealership_account_id) WHERE is_default AND removed_at IS NULL;
-- The dealership's Square customer, made the first time a card is saved.
CREATE TABLE dealer_square_customers (
 dealership_account_id uuid NOT NULL REFERENCES dealership_accounts(id),
 environment text NOT NULL CHECK (environment IN ('sandbox','production')),
 square_customer_id text NOT NULL,
 PRIMARY KEY (dealership_account_id, environment)
);

-- #36: the dealership's address, printed in Client Details on its invoices and statements.
ALTER TABLE dealership_accounts ADD COLUMN address text NOT NULL DEFAULT '' CHECK (length(address) <= 300);

-- #34: an upgrade offer may carry a discount, a percent or an amount off the upgraded package, taken off the booking's invoice when the
-- customer takes the offer online.
ALTER TABLE package_upsells ADD COLUMN discount_percent numeric(5,2) CHECK (discount_percent > 0 AND discount_percent <= 100);
ALTER TABLE package_upsells ADD COLUMN discount_cents bigint NOT NULL DEFAULT 0 CHECK (discount_cents >= 0 AND discount_cents <= 100000000);
ALTER TABLE package_upsells ADD CONSTRAINT package_upsells_one_discount CHECK (discount_percent IS NULL OR discount_cents = 0);
