-- now() is the same for every row of a transaction, so lines saved together had no order and the main service could come out as an add-on.
ALTER TABLE invoice_items ALTER COLUMN created_at SET DEFAULT clock_timestamp();
