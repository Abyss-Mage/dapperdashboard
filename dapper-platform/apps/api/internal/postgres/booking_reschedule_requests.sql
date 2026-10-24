CREATE TABLE booking_reschedule_requests (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 actor_id uuid NOT NULL REFERENCES principals(id),
 booking_id uuid NOT NULL,
 from_version integer NOT NULL CHECK(from_version>0),
 to_version integer NOT NULL CHECK(to_version=from_version+1),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,location_id,request_key),
 FOREIGN KEY(organization_id,location_id,booking_id) REFERENCES bookings(organization_id,location_id,id)
);
