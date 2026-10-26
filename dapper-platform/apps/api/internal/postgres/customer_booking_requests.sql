CREATE TABLE customer_booking_requests (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 customer_id uuid NOT NULL, vehicle_id uuid NOT NULL,
 vehicle_snapshot jsonb NOT NULL CHECK(jsonb_typeof(vehicle_snapshot)='object'),
 actor_id uuid NOT NULL REFERENCES principals(id),
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 offer_id uuid NOT NULL, offer_version integer NOT NULL CHECK(offer_version>0),
 offer_snapshot jsonb NOT NULL CHECK(jsonb_typeof(offer_snapshot)='object'),
 rule_snapshot jsonb NOT NULL CHECK(jsonb_typeof(rule_snapshot)='object'),
 requested_start timestamptz NOT NULL,
 customer_note text NOT NULL CHECK(length(customer_note)<=2000),
 terms_reviewed boolean NOT NULL CHECK(terms_reviewed),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(actor_id,request_key),
 UNIQUE(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id),
 FOREIGN KEY(organization_id,location_id,customer_id,vehicle_id) REFERENCES vehicles(organization_id,location_id,customer_id,id),
 FOREIGN KEY(organization_id,location_id,offer_id) REFERENCES booking_offers(organization_id,location_id,id),
 FOREIGN KEY(offer_id,offer_version) REFERENCES booking_offer_revisions(offer_id,version)
);
CREATE TABLE customer_booking_request_events (
 request_id uuid PRIMARY KEY REFERENCES customer_booking_requests(id),
 organization_id uuid NOT NULL, location_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL CHECK(action='submitted'),created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id,request_id) REFERENCES customer_booking_requests(organization_id,location_id,id)
);
CREATE FUNCTION protect_customer_booking_request() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Submitted customer request snapshots are immutable' USING ERRCODE='23514'; END $$;
CREATE TRIGGER customer_booking_request_immutable BEFORE UPDATE OR DELETE ON customer_booking_requests FOR EACH ROW EXECUTE FUNCTION protect_customer_booking_request();
CREATE TRIGGER customer_booking_request_event_immutable BEFORE UPDATE OR DELETE ON customer_booking_request_events FOR EACH ROW EXECUTE FUNCTION protect_customer_booking_request();
CREATE INDEX customer_booking_requests_location_history ON customer_booking_requests(organization_id,location_id,created_at DESC,id);
CREATE INDEX customer_booking_requests_actor_history ON customer_booking_requests(actor_id,customer_id,created_at DESC,id);
