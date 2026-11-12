-- Owner 2026-11-13: each franchise connects Square with its own Application ID and Access token, with no app secret, so the secret
-- is optional (kept only for franchises that connected through Square's sign-in page before).
ALTER TABLE location_square_apps ALTER COLUMN secret_sealed DROP NOT NULL;
