-- Service address for mobile bookings, and the customer's remembered address.
ALTER TABLE customers ADD COLUMN address text NOT NULL DEFAULT '' CHECK (length(address) <= 300);
ALTER TABLE bookings ADD COLUMN service_address text NOT NULL DEFAULT '' CHECK (length(service_address) <= 300);

-- Quotations no longer need a customer booking request or a vehicle: staff can quote a client by name. (The composite
-- vehicle and request foreign keys are not enforced while those columns are NULL.)
ALTER TABLE scoped_quotations ALTER COLUMN request_id DROP NOT NULL, ALTER COLUMN vehicle_id DROP NOT NULL;
