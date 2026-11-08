-- Marks a booking email as really sent, so one that failed can be tried again. Older rows count as sent, so nothing old goes out now.
ALTER TABLE booking_email_log ADD COLUMN delivered_at timestamptz;
UPDATE booking_email_log SET delivered_at=sent_at;
