-- Owner-only permanent delete of test bookings (owner 2026-11-14, error404 #82 #84 #86). Issued invoices, their items and refund history stay
-- protected as before, except inside the one transaction where the app has set dapper.purge_booking=on for a booking the owner confirmed by
-- typing its number (PurgeBooking). Square card history is never touched: such bookings are refused before this is set.
CREATE OR REPLACE FUNCTION protect_issued_invoice() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF OLD.status<>'draft' AND coalesce(current_setting('dapper.purge_booking', true),'')<>'on' THEN RAISE EXCEPTION 'Issued invoices must be retained'; END IF;
  RETURN OLD;
 END IF;
 IF TG_OP='INSERT' OR (OLD.status='draft' AND NEW.status IN ('issued','partially_paid','paid')) THEN
  IF NEW.status IN ('issued','partially_paid','paid') AND
   (NEW.issuer_id IS NULL OR NEW.issuer_snapshot IS NULL OR NEW.customer_snapshot IS NULL OR NEW.issued_by IS NULL OR NEW.issued_at IS NULL) THEN
   RAISE EXCEPTION 'Explicit invoice issuance snapshot required';
  END IF;
 END IF;
 IF TG_OP='UPDATE' AND OLD.status<>'draft' AND ROW(OLD.organization_id,OLD.location_id,OLD.customer_id,OLD.booking_id,OLD.job_id,OLD.invoice_number,OLD.currency,OLD.subtotal_cents,OLD.tax_cents,OLD.total_cents,OLD.notes,OLD.issuer_id,OLD.issuer_snapshot,OLD.customer_snapshot,OLD.issued_by,OLD.issued_at)
  IS DISTINCT FROM ROW(NEW.organization_id,NEW.location_id,NEW.customer_id,NEW.booking_id,NEW.job_id,NEW.invoice_number,NEW.currency,NEW.subtotal_cents,NEW.tax_cents,NEW.total_cents,NEW.notes,NEW.issuer_id,NEW.issuer_snapshot,NEW.customer_snapshot,NEW.issued_by,NEW.issued_at) THEN
  RAISE EXCEPTION 'Issued invoice details are immutable';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION protect_issued_invoice_items() RETURNS trigger LANGUAGE plpgsql SET search_path FROM CURRENT AS $$
DECLARE invoice_status text;
BEGIN
 IF TG_OP='DELETE' AND coalesce(current_setting('dapper.purge_booking', true),'')='on' THEN RETURN OLD; END IF;
 IF TG_OP<>'INSERT' THEN
  SELECT status INTO invoice_status FROM invoices WHERE id=OLD.invoice_id FOR SHARE;
  IF invoice_status<>'draft' THEN RAISE EXCEPTION 'Issued invoice items are immutable'; END IF;
 END IF;
 IF TG_OP<>'DELETE' THEN
  SELECT status INTO invoice_status FROM invoices WHERE id=NEW.invoice_id FOR SHARE;
  IF invoice_status<>'draft' THEN RAISE EXCEPTION 'Issued invoice items are immutable'; END IF;
  RETURN NEW;
 END IF;
 RETURN OLD;
END $$;

CREATE OR REPLACE FUNCTION protect_payment_refund_adjustment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND coalesce(current_setting('dapper.purge_booking', true),'')='on' THEN RETURN OLD; END IF;
 RAISE EXCEPTION 'Refund adjustment history is immutable' USING ERRCODE='23514';
END $$;
