-- The sales strip shown above the website header: one per site, edited from the dashboard (text, optional link, colour, switch, end date).
ALTER TABLE public_sites ADD COLUMN banner jsonb NOT NULL DEFAULT '{}'::jsonb;
