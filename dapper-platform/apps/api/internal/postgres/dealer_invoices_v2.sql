-- Dealership invoices, second round (error404 #18/#19, 2026-11-11): an invoice date and a due date picked on a calendar (instead of
-- "due in N days"), a PO number, an optional hand-typed invoice number, a discount as an amount or a percent (taken off before tax),
-- an item description under each line, and a private pay link the email carries so the dealership can view and pay without signing in.
ALTER TABLE invoices ADD COLUMN invoice_date date;
ALTER TABLE invoices ADD COLUMN due_date date;
ALTER TABLE invoices ADD COLUMN po_number text NOT NULL DEFAULT '' CHECK (length(po_number) <= 40);
ALTER TABLE invoices ADD COLUMN manual_number text NOT NULL DEFAULT '' CHECK (manual_number = '' OR manual_number ~ '^[A-Za-z0-9][A-Za-z0-9 ./#-]{0,39}$');
ALTER TABLE invoices ADD COLUMN discount_percent numeric(5,2) CHECK (discount_percent > 0 AND discount_percent <= 100);
ALTER TABLE invoices ADD CONSTRAINT invoices_dealer_dates CHECK (due_date IS NULL OR (invoice_date IS NOT NULL AND due_date >= invoice_date AND dealership_account_id IS NOT NULL));

ALTER TABLE invoice_items ADD COLUMN detail text NOT NULL DEFAULT '' CHECK (length(detail) <= 500);

-- Only a hash of the link's secret is kept. A link opens one invoice: view, PDF and card payment.
CREATE TABLE invoice_pay_links (
 token_hash text PRIMARY KEY,
 invoice_id uuid NOT NULL REFERENCES invoices(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL
);
CREATE INDEX invoice_pay_links_invoice_idx ON invoice_pay_links(invoice_id);
