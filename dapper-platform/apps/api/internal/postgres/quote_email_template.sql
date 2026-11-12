-- The owner can reword every email the dashboard lists, including the one sent with an issued quotation.
ALTER TABLE email_templates DROP CONSTRAINT email_templates_template_key_check;
ALTER TABLE email_templates ADD CONSTRAINT email_templates_template_key_check CHECK(template_key IN ('password_reset','invitation','booking_confirmation','booking_admin','booking_admin_cancelled','booking_admin_rescheduled','booking_completed','booking_cancelled','booking_rescheduled','quotation_issued'));
