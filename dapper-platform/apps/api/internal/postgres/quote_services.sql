-- error404 #163 (owner 2026-11-19): the services offered in the contact page's "Request a quote" list, edited from the dashboard.
-- Empty means the website's built-in list.
ALTER TABLE public_sites ADD COLUMN quote_services jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(quote_services)='array');
