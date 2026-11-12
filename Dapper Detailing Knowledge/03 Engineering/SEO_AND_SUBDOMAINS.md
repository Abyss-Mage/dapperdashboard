# Search settings, sitemaps and app subdomains

**Owner decisions, 2026-11-13:**
- The owner dashboard is served at `dashboard.dapperdetailing.ca` and the staff workspace at `workspace.dapperdetailing.ca`, from the new server (201.18.205.129). The main site `dapperdetailing.ca` stays on the legacy host until the owner moves it.
- Each subdomain has its own XML sitemap and robots.txt, listing only its sign-in page (`/dashboard`, `/workspace`); everything behind sign-in stays out of search. The website sitemap lists its public pages, minus any hidden in the search settings.
- Search settings cover the whole website (title, description, keywords, share image, Google and Bing verification codes) and each public page (title, description, keywords, share image, hide from search).
- The Search & sitemaps tab and the settings are for anyone with `access.manage` (the owner and administrators).

## How it works
- Settings: `public_sites.seo` (jsonb, migration 120), keyed by the website's slug (`NEXT_PUBLIC_DAPPER_SITE_SLUG`), so the owner edits the public website whichever franchise the dashboard has open. `GET/PUT /api/v1/organizations/{org}/sites/{slug}/seo`. Saving needs the organization-level `access.manage`, checks lengths, rejects `<`/`>`, allows only `/…` or `https://` images and plain verification codes, and is written to `content_events`.
- The website reads them on its server from the API on loopback (`127.0.0.1:8121`) and refreshes every minute. If they cannot be read, pages keep their built-in titles and descriptions.
- `apps/web/proxy.ts` sends the bare `dashboard.` and `workspace.` addresses to `/dashboard` and `/workspace`. `app/sitemap.ts` and `app/robots.ts` answer per hostname. The page list is in `app/seo-pages.ts`.
- Extra addresses: optional `DAPPER_EXTRA_ORIGINS` in the server's `.env` (comma-separated `https://host`). The gateway accepts those hosts, and the API and sign-in accept those origins. Sessions are per address, so staff sign in on each subdomain separately.

## Still to do on the server (not done: production config changes were not permitted from the agent session)
1. DNS at the domain registrar: A records `dashboard` and `workspace` → `201.18.205.129`.
2. In `/opt/dapper/app/.env`, add `DAPPER_EXTRA_ORIGINS=https://dashboard.dapperdetailing.ca,https://workspace.dapperdetailing.ca`.
3. In `/etc/caddy/Caddyfile`, add a site for `dashboard.dapperdetailing.ca, workspace.dapperdetailing.ca` with the same `reverse_proxy` as the existing IP site (automatic HTTPS). Reload Caddy and restart `dapper`.

## Evidence
- Static checks: Go build/vet and the TypeScript check passed.
- Runtime tests: the database test covers SEO save, permissions, input checks and serving; a unit test covers origin and host matching.
- Deployed: website pages, robots.txt and sitemap answer on the IP address as before.
- Not verified: the subdomains (waiting on DNS and the server steps above), the settings page in a browser, and search engines reading the sitemaps.
