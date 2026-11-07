-- The business can get an email for new, cancelled and rescheduled bookings, each switched on or off, at one or more addresses.
ALTER TABLE messaging_settings DROP CONSTRAINT IF EXISTS messaging_settings_smtp_notify_email_check;
ALTER TABLE messaging_settings ADD CONSTRAINT messaging_settings_smtp_notify_email_check CHECK(length(smtp_notify_email)<=1000);
ALTER TABLE messaging_settings ADD COLUMN notify_new boolean NOT NULL DEFAULT true;
ALTER TABLE messaging_settings ADD COLUMN notify_cancelled boolean NOT NULL DEFAULT true;
ALTER TABLE messaging_settings ADD COLUMN notify_rescheduled boolean NOT NULL DEFAULT true;
