-- B2B dealership invoices (error404 #17; decisions 2026-11-11, see the vault's B2B_DEALERSHIP_INVOICING_PLAN). A dealership invoice is an
-- ordinary invoice for the dealership's customer record, tied to the dealership, with payment terms (its due date is set when it is
-- issued), its own number series, VIN / stock number on each line, and its own printed template.
ALTER TABLE invoices ADD COLUMN dealership_account_id uuid REFERENCES dealership_accounts(id);
ALTER TABLE invoices ADD COLUMN terms_days integer CHECK (terms_days BETWEEN 0 AND 180);
ALTER TABLE invoices ADD CONSTRAINT invoices_terms_only_for_dealers CHECK (terms_days IS NULL OR dealership_account_id IS NOT NULL);
CREATE INDEX invoices_dealership_idx ON invoices(organization_id, location_id, dealership_account_id, issued_at DESC) WHERE dealership_account_id IS NOT NULL;

ALTER TABLE invoice_items ADD COLUMN vin text NOT NULL DEFAULT '' CHECK (vin ~ '^[A-Z0-9]{0,20}$');
ALTER TABLE invoice_items ADD COLUMN stock_number text NOT NULL DEFAULT '' CHECK (length(stock_number) <= 40);

-- The dealership number series, apart from the customer invoice series (no row: DLR-1, DLR-2, ...).
CREATE TABLE dealer_invoice_series (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 prefix text NOT NULL CHECK (prefix ~ '^[A-Z0-9-]{1,24}$'),
 next_number bigint NOT NULL CHECK (next_number > 0 AND next_number < 9223372036854775807),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);

-- The dealership invoice's printed template; same shape as the invoice and quotation templates. No row: it starts from the invoice template.
CREATE TABLE dealer_print_templates (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id),
 config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object'),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE location_dealer_templates (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object'),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
