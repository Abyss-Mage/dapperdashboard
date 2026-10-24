ALTER TABLE scoped_quotations ADD CONSTRAINT scoped_quotation_customer_identity UNIQUE(organization_id,location_id,customer_id,id);
-- Migration 48: durable Square merchant bindings, payment intents, capture observations and allocations.
CREATE TABLE square_merchant_binding_revisions (
 organization_id uuid NOT NULL,location_id uuid NOT NULL,version integer NOT NULL CHECK(version>0),
 environment text NOT NULL CHECK(environment IN ('sandbox','production')),
 merchant_id text NOT NULL CHECK(length(btrim(merchant_id)) BETWEEN 1 AND 128),
 square_location_id text NOT NULL CHECK(length(btrim(square_location_id)) BETWEEN 1 AND 128),
 credential_reference text NOT NULL CHECK(length(btrim(credential_reference)) BETWEEN 1 AND 128),
 reviewed_by uuid NOT NULL REFERENCES principals(id),reviewed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,location_id,version),
 FOREIGN KEY(organization_id,location_id) REFERENCES locations(organization_id,id)
);
CREATE TABLE square_payment_intents (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),organization_id uuid NOT NULL,location_id uuid NOT NULL,
 customer_id uuid NOT NULL,actor_id uuid NOT NULL REFERENCES principals(id),
 invoice_id uuid,quotation_id uuid,
 binding_version integer NOT NULL,
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 provider_request_key uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
 amount_cents bigint NOT NULL CHECK(amount_cents>0),currency text NOT NULL CHECK(currency='CAD'),
 state text NOT NULL CHECK(state IN ('prepared','submitting','uncertain','captured','settled','failed')),
 provider_payment_id text,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK((invoice_id IS NOT NULL)::integer+(quotation_id IS NOT NULL)::integer=1),
 CHECK(state NOT IN ('captured','settled') OR (provider_payment_id IS NOT NULL AND length(btrim(provider_payment_id))>0)),
 UNIQUE(organization_id,location_id,id),UNIQUE(organization_id,location_id,actor_id,request_key),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,binding_version) REFERENCES square_merchant_binding_revisions(organization_id,location_id,version),
 FOREIGN KEY(organization_id,location_id,customer_id,invoice_id) REFERENCES invoices(organization_id,location_id,customer_id,id),
 FOREIGN KEY(organization_id,location_id,customer_id,quotation_id) REFERENCES scoped_quotations(organization_id,location_id,customer_id,id)
);
CREATE TABLE square_payment_intent_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,location_id uuid NOT NULL,intent_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN ('prepared','submitting','uncertain','captured','settled','failed')),
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id,intent_id) REFERENCES square_payment_intents(organization_id,location_id,id)
);
CREATE FUNCTION protect_square_payment_intent() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Payment intents retain durable recovery identity' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-ARRAY['state','provider_payment_id','updated_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['state','provider_payment_id','updated_at']) THEN
  RAISE EXCEPTION 'Payment intent request binding is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER square_payment_intent_binding_immutable BEFORE UPDATE OR DELETE ON square_payment_intents FOR EACH ROW EXECUTE FUNCTION protect_square_payment_intent();
CREATE FUNCTION protect_square_payment_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Payment configuration and attempt history are immutable' USING ERRCODE='23514'; END $$;
CREATE TRIGGER square_binding_revision_immutable BEFORE UPDATE OR DELETE ON square_merchant_binding_revisions FOR EACH ROW EXECUTE FUNCTION protect_square_payment_history();
CREATE TRIGGER square_intent_history_immutable BEFORE UPDATE OR DELETE ON square_payment_intent_events FOR EACH ROW EXECUTE FUNCTION protect_square_payment_history();
CREATE INDEX square_payment_intents_recovery ON square_payment_intents(state,updated_at) WHERE state IN ('submitting','uncertain');


CREATE FUNCTION guard_square_payment_intent_state() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  IF NEW.state<>'prepared' OR NEW.provider_payment_id IS NOT NULL THEN RAISE EXCEPTION 'Payment intent must begin prepared' USING ERRCODE='23514'; END IF;
  RETURN NEW;
 END IF;
 IF OLD.provider_payment_id IS NOT NULL AND NEW.provider_payment_id IS DISTINCT FROM OLD.provider_payment_id THEN RAISE EXCEPTION 'Provider payment identity cannot change' USING ERRCODE='23514'; END IF;
 IF NEW.provider_payment_id IS NOT NULL AND NEW.state NOT IN ('captured','settled') THEN RAISE EXCEPTION 'Provider settlement identity requires settled state' USING ERRCODE='23514'; END IF;
 IF NEW.state=OLD.state THEN RETURN NEW; END IF;
 IF (OLD.state='prepared' AND NEW.state='submitting') OR
    (OLD.state='submitting' AND NEW.state IN ('uncertain','captured','settled','failed')) OR
    (OLD.state='uncertain' AND NEW.state IN ('submitting','captured','settled','failed')) THEN RETURN NEW; END IF;
 IF OLD.state='captured' AND NEW.state='settled' THEN RETURN NEW; END IF;
 RAISE EXCEPTION 'Invalid durable payment intent transition' USING ERRCODE='23514';
END $$;
CREATE TRIGGER square_payment_intent_state_guard BEFORE INSERT OR UPDATE ON square_payment_intents FOR EACH ROW EXECUTE FUNCTION guard_square_payment_intent_state();


CREATE TABLE square_payment_capture_observations (
 intent_id uuid PRIMARY KEY,organization_id uuid NOT NULL,location_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('sandbox','production')),
 merchant_id text NOT NULL,square_location_id text NOT NULL,
 provider_payment_id text NOT NULL CHECK(length(btrim(provider_payment_id))>0),
 amount_cents bigint NOT NULL CHECK(amount_cents>0),currency text NOT NULL CHECK(currency='CAD'),
 observed_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(environment,merchant_id,provider_payment_id),
 FOREIGN KEY(organization_id,location_id,intent_id) REFERENCES square_payment_intents(organization_id,location_id,id)
);
CREATE TRIGGER square_capture_observation_immutable BEFORE UPDATE OR DELETE ON square_payment_capture_observations FOR EACH ROW EXECUTE FUNCTION protect_square_payment_history();


CREATE TABLE square_payment_allocations (
 intent_id uuid PRIMARY KEY REFERENCES square_payment_capture_observations(intent_id),
 organization_id uuid NOT NULL,location_id uuid NOT NULL,payment_id uuid NOT NULL UNIQUE,
 amount_cents bigint NOT NULL CHECK(amount_cents>0),created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id,intent_id) REFERENCES square_payment_intents(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,payment_id,amount_cents) REFERENCES payments(organization_id,location_id,id,amount_cents)
);
CREATE TRIGGER square_payment_allocation_immutable BEFORE UPDATE OR DELETE ON square_payment_allocations FOR EACH ROW EXECUTE FUNCTION protect_square_payment_history();

CREATE FUNCTION enforce_square_intent_proof() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target uuid; current_intent square_payment_intents%ROWTYPE; valid_capture boolean; valid_allocation boolean;
BEGIN
 IF TG_TABLE_NAME='square_payment_intents' THEN target:=NEW.id; ELSE target:=NEW.intent_id; END IF;
 SELECT * INTO STRICT current_intent FROM square_payment_intents WHERE id=target;
 SELECT EXISTS(SELECT 1 FROM square_payment_capture_observations o JOIN square_merchant_binding_revisions b ON (b.organization_id,b.location_id,b.version)=(current_intent.organization_id,current_intent.location_id,current_intent.binding_version) WHERE o.intent_id=target AND (o.organization_id,o.location_id,o.environment,o.merchant_id,o.square_location_id,o.provider_payment_id,o.amount_cents,o.currency)=(current_intent.organization_id,current_intent.location_id,b.environment,b.merchant_id,b.square_location_id,current_intent.provider_payment_id,current_intent.amount_cents,current_intent.currency)) INTO valid_capture;
 IF current_intent.state IN ('captured','settled') THEN
  IF NOT valid_capture THEN RAISE EXCEPTION 'Captured intent requires matching immutable provider evidence' USING ERRCODE='23514'; END IF;
 ELSE
  IF EXISTS(SELECT 1 FROM square_payment_capture_observations WHERE intent_id=target) THEN RAISE EXCEPTION 'Capture evidence requires captured intent state' USING ERRCODE='23514'; END IF;
 END IF;
 SELECT EXISTS(SELECT 1 FROM square_payment_allocations a JOIN payments p ON p.id=a.payment_id WHERE a.intent_id=target AND (p.organization_id,p.location_id,p.customer_id,p.invoice_id,p.provider_reference,p.amount_cents,p.currency)=(current_intent.organization_id,current_intent.location_id,current_intent.customer_id,current_intent.invoice_id,current_intent.provider_payment_id,current_intent.amount_cents,current_intent.currency)) INTO valid_allocation;
 IF current_intent.state='settled' THEN
  IF NOT valid_allocation THEN RAISE EXCEPTION 'Settled intent requires matching ledger allocation' USING ERRCODE='23514'; END IF;
 ELSE
  IF EXISTS(SELECT 1 FROM square_payment_allocations WHERE intent_id=target) THEN RAISE EXCEPTION 'Ledger allocation requires settled intent state' USING ERRCODE='23514'; END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER square_intent_proof_consistency AFTER INSERT OR UPDATE ON square_payment_intents DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION enforce_square_intent_proof();
CREATE CONSTRAINT TRIGGER square_capture_proof_consistency AFTER INSERT ON square_payment_capture_observations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION enforce_square_intent_proof();
CREATE CONSTRAINT TRIGGER square_allocation_proof_consistency AFTER INSERT ON square_payment_allocations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION enforce_square_intent_proof();
