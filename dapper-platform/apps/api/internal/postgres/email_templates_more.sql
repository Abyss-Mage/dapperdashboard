-- More system emails (a new booking to the business, a cancelled booking, a rescheduled booking), emails written as HTML, the address new-booking
-- alerts go to, and a small log so a booking is never told "cancelled" twice.
ALTER TABLE email_templates DROP CONSTRAINT email_templates_template_key_check;
ALTER TABLE email_templates ADD CONSTRAINT email_templates_template_key_check CHECK(template_key IN ('password_reset','invitation','booking_confirmation','booking_admin','booking_cancelled','booking_rescheduled'));
ALTER TABLE email_templates ADD COLUMN is_html boolean NOT NULL DEFAULT false;
ALTER TABLE messaging_settings ADD COLUMN smtp_notify_email text NOT NULL DEFAULT '' CHECK(length(smtp_notify_email)<=254);
CREATE TABLE booking_email_log (
 booking_id uuid NOT NULL, kind text NOT NULL, sent_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX booking_email_log_idx ON booking_email_log(booking_id, kind, sent_at);
