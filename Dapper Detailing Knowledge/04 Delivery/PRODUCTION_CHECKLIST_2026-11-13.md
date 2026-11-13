# Full production checklist (2026-11-13)

User direction (2026-11-13): move from the dev-production setup (bare IP 201.18.205.129) to full production. Backups first, then this checklist.

Legend: `[x]` done and checked · `[ ]` to do. **Who:** *You* (owner, needs your accounts or passwords) · *Claude* (code or server work I can do) · *Client* (business decision). Facts below were read from the server on 2026-11-13; secret values were never read or recorded.

---

## 0. Backups before any change (done 2026-11-13)
- [x] Fresh database dump on the server: `/opt/dapper/backups/pre-production-20261113-094233.dump`. Also on the server: the matching `.env` copy, and `pre-production-files-….tar.gz` (uploads, SOP PDFs, the app's own backups folder, Caddyfile, backup cron, dapper.service). All mode 600, owner `dapper`.
- [x] Copied off the server to `%USERPROFILE%\DapperBackups\pre-production-20261113-094233\` on the owner's PC. SHA-256 matches the server for all three files. The folder is limited to the owner's Windows account. It holds secrets (`.env`): keep it out of the project, cloud-synced folders and chat.
- [x] Restore tested: the dump was loaded into a throwaway local database and then deleted. It held 152 tables, schema at migration 120, 1 organization, 3 franchises, 605 bookings, 34 invoices and 471 customers.
- [x] Code: git tag `pre-production-2026-11-13` (commit 66bb97da) pushed to GitHub. The server also keeps the previous Go binaries in `.dapper/bin.prev`.
- [ ] *You*: download an owner backup from Dashboard → Settings → Backups (encrypted `.dapbk`, your own passphrase). Store it somewhere other than this PC.

## 1. Decisions to make before go-live (Client)
- [ ] Hosting: keep this VPS (Ubuntu 24.04, 4 vCPU, 15 GB RAM, 4% disk used) as production, or move to a new server. Everything below assumes this one.
- [ ] Move `dapperdetailing.ca` from the legacy host (88.223.84.117) to this server, and pick the date. The legacy site is not changed or retired without your go-ahead.
- [ ] Settle the open decisions that change live behaviour (`02 Decisions/OPEN_DECISIONS.md`): prices, durations and resources (DEC-05); deposits, cancellation and no-show (DEC-06); tax rounding (DEC-04); legal issuer and invoice numbering (DEC-03); Square merchants (DEC-08); email accounts (DEC-12, blocked); booking history migration (DEC-13, blocked).
- [ ] Which current records are real and which are test data (see section 7).

## 2. Server security
- [x] Firewall: only SSH, 80 and 443 open. PostgreSQL listens on localhost only. Automatic security updates are on.
- [ ] *You*: change the root password (it was shared in chat on 2026-10-30).
- [ ] *You + Claude*: turn off SSH password login and direct root login (currently **both on**), keys only. First confirm a second key session works, so you can't be locked out.
- [ ] *Claude*: install fail2ban for SSH.
- [ ] *You*: apply the 4 pending updates and **reboot** (kernel 6.8.0-149 is waiting; the server has been up 13 days). Then check that `dapper`, `apache2`, `postgresql` and `dapper-import-mcp` all come back on their own. A reboot has never been tested.
- [ ] *Claude*: make the extra `pre-*.dump` files in `/opt/dapper/backups` private (three are group-readable).

## 3. Domain and HTTPS
- [ ] *You*: a day before, lower the DNS TTL of `dapperdetailing.ca` to 300 s so a switch or rollback spreads fast.
- [ ] *You*: DNS A records → 201.18.205.129: `@`, `www`, `dashboard`, `workspace`.
- [ ] *You*: Apache sites with Let's Encrypt (certbot) for `dashboard.` and `workspace.` at setup. On go-live day add `dapperdetailing.ca` and `www` (redirect to the main site). See `NEW_PRODUCTION_SERVER_SETUP.md` and `scripts/server/apache-dapper.conf`.
- [ ] *Claude*: `.env`: set `DAPPER_SITE_URL=https://dapperdetailing.ca`, which drives email links, invitations, pay links and the sign-in origin. Also `DAPPER_EXTRA_ORIGINS=https://dashboard.dapperdetailing.ca,https://workspace.dapperdetailing.ca`. Then restart.
- [ ] *Claude*: after all four addresses load with valid certificates, turn on HSTS.
- [ ] Note: sign-ins are per address, so staff sign in again after the switch.

## 4. App settings
- [ ] *Claude (code)*: pass `DAPPER_PUBLIC_INDEXING`, `DAPPER_SQUARE_WEBHOOK_URL` and `DAPPER_SQUARE_WEBHOOK_SIGNATURE_KEY` from `.env` to the services. The launcher does not pass them today, so adding them to `.env` alone does nothing.
- [ ] *Claude*: `DAPPER_PUBLIC_INDEXING=enabled` **only after** the domain is live. Until then the site tells search engines to stay away.
- [ ] Check: session lifetime 8 h, timezone America/Toronto, website slug `dapper-detailing-pros-peterborough` (all set now).
- [ ] *You*: Redis (optional cache): install and set `DAPPER_REDIS_URL` (commands in `03 Engineering/SEO_AND_SUBDOMAINS.md`).

## 5. Email
- [ ] *You*: get email working. The Outlook mail service was failing on 2026-11-13, and dealership invoice emails did not send. Either fix the Outlook account or switch to a transactional provider (e.g. Postmark, SES, Brevo) and update the `DAPPER_SMTP_*` settings.
- [ ] *You*: SPF, DKIM and DMARC records for the sending domain, so mail doesn't land in spam.
- [ ] *Claude + You*: send one of each to a real inbox: booking confirmation, reminder, invoice, dealership invoice with PDF and pay link, staff invitation, password reset, quote. Check links point at the real domain.

## 6. Card payments (Square)
- [ ] *You*: for each franchise, under Card payments → **Live**, paste the Square Application ID and Access token, then choose the Square location.
- [ ] *You*: in the Square Developer Console, add the webhook `https://dapperdetailing.ca/api/v1/integrations/square/webhooks` and give its signature key to Claude for `.env` (needs the code item in section 4).
- [ ] *You + Claude*: a real **$1 charge and refund** per franchise: online booking, invoice pay link, dealership portal (new card and saved card), and the staff card reader if used.
- [ ] *Claude*: confirm no franchise still points at sandbox keys.

## 7. Data
- [ ] *Client + Claude*: list which bookings, invoices, customers and dealerships are test or synthetic. Remove them or keep them separate before go-live; never relabel them as real.
- [ ] *Claude*: final booking history import from BookingPress once DEC-13 is unblocked. Reconcile counts and totals against the legacy export.
- [ ] *You*: check services, prices, add-ons, durations and taxes for every franchise. Set each franchise's invoice and quote starting numbers and its legal details (issuer, address, tax number).
- [ ] *You*: invite every staff member. Give roles, require two-factor for admins, set shop iPad PINs, upload SOP and policy documents.
- [ ] *You*: dealership accounts, contact emails and portal logins.

## 8. Website and SEO
- [ ] *You*: publish services, portfolio and banner, and fill Search & sitemaps (titles, descriptions, keywords, share image).
- [ ] *Claude*: 301 redirects from the old WordPress addresses to the new pages, so search rankings and bookmarks carry over. This needs the legacy URL list.
- [ ] *You*: verify Google Search Console and Bing (codes go in Search & sitemaps), then submit the three sitemaps.
- [ ] *You*: tracking tags (Google Tag, Meta Pixel), the Mailchimp popup, and review of the legal pages.

## 9. Backups and recovery (ongoing)
- [x] Nightly database and `.env` backup at 03:15, kept 14 days, **same disk only**.
- [ ] *Claude*: add uploads and SOP files to the nightly backup (it covers the database and `.env` only).
- [ ] *You*: an off-server copy every day, either by scheduling `scripts/pull-backup.ps1` on an always-on PC or by sending to cloud storage (e.g. Backblaze B2 or S3, encrypted).
- [ ] *You + Claude*: a restore test every quarter, plus the owner `.dapbk` backup monthly.

## 10. Monitoring
- [ ] *You*: an external uptime check (e.g. UptimeRobot) on `https://dapperdetailing.ca/` and `/api/v1/health`, alerting your phone.
- [ ] *Claude*: disk-space and failed-backup alerts by email, and log rotation for `.dapper/logs`.
- [ ] Keep error404 reports open for testers for the first weeks.

## 11. Go-live day
1. Announce a short pause for bookings on the legacy site. Take a final legacy export and import it (section 7).
2. Take a fresh backup (as in section 0).
3. Switch DNS, update `.env` (section 3) and restart. Wait for certificates.
4. Smoke test on the real domain:
   - owner sign-in and two-factor
   - staff sign-in on the workspace and on the shop iPad
   - an online booking
   - an invoice with the $1 card charge and refund
   - a dealership invoice email, then the portal payment
   - a photo upload
   - an SOP acknowledgment
   - the website pages and sitemaps
5. Turn on indexing and submit the sitemaps.
6. Watch logs and error404 for 48 hours.

## 12. Rollback
- DNS back to the legacy host (fast while TTL is 300 s).
- App code: `git checkout pre-production-2026-11-13` and deploy, or restore `.dapper/bin.prev`.
- Data: `pg_restore` the pre-go-live dump. Bookings taken after it would need re-entering, so note them during the watch window.

---

## Progress (2026-11-13, later)
- [x] **Legacy import MCP server** (`tools/legacy-import-mcp/server.py`, registered in `.mcp.json` as `dapper-legacy-import`). Owner decisions:
  - Sign-in to the live site uses a WordPress Application Password in the Windows user environment (`DAPPER_LEGACY_WP_USER`, `DAPPER_LEGACY_WP_APP_PASSWORD`). The live site is only read.
  - It imports all four groups: catalogue, content and media, customers, and booking and payment history.
  - Every import is a dry run first, then applied only with the plan's confirm code, in one transaction, under a batch id that can be undone.
  - Sources:
    - Catalogue, content and media come from the live site's `dapper-export/v1` API.
    - Customers, appointments and payments come from the BookingPress export file. The BookingPress REST API is not used: its routes are POST-only and many change data.
  - Mapping reuses the approved scripts (`import_bookingpress_history.py`, `import_bookingpress_invoices.py`, `cmd/seed-catalogue`). Vehicles are not imported (approved 2026-11-04 history mapping).
  - Content and media are fetched and saved locally for review. The new site's pages are built in code, so they are placed by hand.
  - The target is set by `DAPPER_IMPORT_DATABASE_URL`.
  - Evidence:
    - 7 unit tests pass.
    - End-to-end on a throwaway local database with the production schema and the cached export: the plan showed 453 customers, 584 bookings (364 completed, 143 confirmed, 77 cancelled) and 584 billing rows. A wrong confirm and a repeated apply were refused. Undo removed everything. The throwaway database was dropped.
    - Not verified: live-site sign-in (no Application Password yet) and the catalogue parts against a database.
- [x] Section 4 code item: `DAPPER_PUBLIC_INDEXING`, `DAPPER_SQUARE_WEBHOOK_URL` and `DAPPER_SQUARE_WEBHOOK_SIGNATURE_KEY` now pass from `.env` to the services (install test). Not deployed yet.
- [ ] **Blocked:** at about 20:30 on 2026-11-13 every SSH host key of 201.18.205.129 had changed (ED25519 `SHA256:OP/ZS5lF…`, ECDSA `SHA256:WOd4M+1X…`; on record: ED25519 `SHA256:FDOf0C3n…`, ECDSA `SHA256:U52tWOZo…`), and HTTPS no longer answered. Server work (fail2ban, making the old dumps private, adding uploads to the nightly backup, deploy) stopped until the owner confirms the server was rebuilt and its new key fingerprint matches the provider console. The pre-production backups are on the owner's PC, so a rebuilt server can be restored from them.

## Platform change (2026-11-13, owner)
Full production runs on a **brand-new Ubuntu 26.04 LTS server with Apache 2.4** (Let's Encrypt through certbot) and a **clean database**. Setup steps are in `NEW_PRODUCTION_SERVER_SETUP.md`. Claude has no shell access there; its way in is the legacy import MCP server over HTTPS with a token. Server items above marked *Claude* are now done by the owner from that guide. The old dev server at 201.18.205.129 was rebuilt; its data is in the pre-production backup on the owner's PC.

## Production import (2026-11-13, through the import MCP server at dashboard.dapperdetailing.ca)
Target: the new clean database `ddp` (migration 120), organization "Dapper Detailing Pros", franchise "Dapper Detailing Pros" (code DDP). Source: the BookingPress export downloaded on 2026-11-13 (477 customers, 609 appointments, 79 services, 12 categories, 228 add-ons). Every step was a dry run first, then applied after the owner's yes.
- [x] Customers and bookings: batch `bookingpress-customers_bookings-20261113-162521`. 477 customers (6 shared phone numbers kept on the first customer only); 609 bookings (364 completed, 167 confirmed, 78 cancelled; 105 with the service name kept as text). Vehicles are not imported. A count check after applying found all 609 bookings, 477 customers and 609 billing rows. The batch can be undone with `undo_import`.
- [x] Billing: batch `bookingpress-billing-20261113-162525`. 609 invoices (525 paid, 6 part-paid, 78 void; 33 shared old numbers given a suffix). New numbering continues after 630.
- [x] Catalogue: 70 central services (9 hidden ones skipped) and 228 add-ons. They reached the franchise automatically when saved.
- [x] Prices: **owner decision 2026-11-13: the legacy website's prices are approved as the live prices** (before tax). 298 set, all matched. This settles DEC-05 for prices; durations and resources are still open.
- [ ] *You*: check the confirmed bookings dated from today onward in the calendar. Imported bookings don't hold their time slot, so block or re-book any real upcoming jobs.
- [ ] Website content and media: fetch with `fetch_live_site` for review and place them by hand (the new site's pages are built in code).
- [ ] Replace the import token (it was pasted in chat). New value in `/etc/dapper/import-mcp.env`, then `systemctl restart dapper-import-mcp`. On the PC, set `DAPPER_IMPORT_MCP_TOKEN` to the same value.
