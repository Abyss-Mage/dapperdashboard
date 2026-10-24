ALTER TABLE bookings ADD CONSTRAINT bookings_scoped_customer_key
 UNIQUE(organization_id,location_id,customer_id,id);
ALTER TABLE jobs ADD CONSTRAINT jobs_scoped_identity_key UNIQUE(organization_id,location_id,id);
ALTER TABLE jobs DROP CONSTRAINT jobs_booking_id_fkey;
ALTER TABLE jobs ADD CONSTRAINT jobs_booking_owner_fkey
 FOREIGN KEY(organization_id,location_id,customer_id,booking_id)
 REFERENCES bookings(organization_id,location_id,customer_id,id);

-- Retry receipts are per request, not a one-job-per-booking business policy.
CREATE TABLE booking_job_requests (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 16 AND 128),
 request_hash text NOT NULL CHECK(length(request_hash)=64),
 booking_id uuid NOT NULL REFERENCES bookings(id),
 job_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,location_id,request_key),
 FOREIGN KEY(organization_id,location_id,job_id) REFERENCES jobs(organization_id,location_id,id)
);
