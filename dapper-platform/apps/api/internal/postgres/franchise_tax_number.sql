-- Each franchise's own GST/HST number. Empty means it prints the number on its business details.
ALTER TABLE locations ADD COLUMN tax_registration text NOT NULL DEFAULT '' CHECK (length(tax_registration) <= 100);
