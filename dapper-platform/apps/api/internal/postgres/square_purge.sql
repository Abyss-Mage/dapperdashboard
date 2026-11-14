-- error404 #126 (owner 2026-11-14): "we will handle square on our own ... just delete when asked". A permanent delete the owner confirms
-- (dapper.purge_booking=on, set only inside PurgeBooking / PurgeDealerInvoice) also removes the invoice's Square payment attempts and
-- their history. Square itself is not touched: refunds there are the owner's to handle. Outside that transaction nothing changes.
CREATE OR REPLACE FUNCTION protect_square_payment_intent() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF coalesce(current_setting('dapper.purge_booking', true),'')='on' THEN RETURN OLD; END IF;
  RAISE EXCEPTION 'Payment intents retain durable recovery identity' USING ERRCODE='23514';
 END IF;
 IF (to_jsonb(NEW)-ARRAY['state','provider_payment_id','updated_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['state','provider_payment_id','updated_at']) THEN
  RAISE EXCEPTION 'Payment intent request binding is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION protect_square_payment_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND TG_TABLE_NAME<>'square_merchant_binding_revisions' AND coalesce(current_setting('dapper.purge_booking', true),'')='on' THEN RETURN OLD; END IF;
 RAISE EXCEPTION 'Payment configuration and attempt history are immutable' USING ERRCODE='23514';
END $$;
