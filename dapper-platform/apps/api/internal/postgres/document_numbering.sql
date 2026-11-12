-- Quotation numbers can follow a scheme of the owner's, kept beside the invoice scheme. An empty prefix keeps the automatic QTE- number.
ALTER TABLE invoice_issuers ADD COLUMN quote_prefix text NOT NULL DEFAULT '' CHECK (quote_prefix ~ '^[A-Z0-9-]{0,24}$');
ALTER TABLE invoice_issuers ADD COLUMN quote_next_number bigint NOT NULL DEFAULT 1 CHECK (quote_next_number > 0 AND quote_next_number < 9223372036854775807);
ALTER TABLE scoped_quotations DROP CONSTRAINT scoped_quotations_reference_number_check;
ALTER TABLE scoped_quotations ADD CONSTRAINT scoped_quotations_reference_number_check CHECK (length(reference_number) BETWEEN 2 AND 60);
