-- A booking is billed once: at most one invoice that is not void may exist for it. Voiding an invoice frees the booking for a replacement.
CREATE UNIQUE INDEX invoices_one_open_per_booking ON invoices(organization_id,location_id,booking_id) WHERE booking_id IS NOT NULL AND status<>'void';
