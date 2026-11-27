-- error404 #175 (owner 2026-11-27): the online booking offers "We come to you" (mobile) and "Pick-up and drop-off" (valet) only while
-- they are switched on in the dashboard. In the shop is always offered. Staff bookings are not limited.
ALTER TABLE public_sites ADD COLUMN offer_mobile boolean NOT NULL DEFAULT true, ADD COLUMN offer_valet boolean NOT NULL DEFAULT true;
