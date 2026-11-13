-- error404 #98 (2026-11-14): a customer is identified by email; several customers may share a phone number, so the one-per-phone rule goes.
DROP INDEX IF EXISTS customers_unique_phone;
