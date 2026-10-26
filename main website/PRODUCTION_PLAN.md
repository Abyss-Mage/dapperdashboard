# Production website closeout plan

Updated 2026-10-24. The cinematic homepage and the requested public page structure are implemented in source inside the existing Next.js application. They are not a separate demo product. Live WordPress/cutover remains unchanged.

## Implemented and locally checked

- Root homepage is a complete studio document; its CSS is isolated from account and dashboard pages. Public pages retain the existing ten service category URLs plus Services, Portfolio, About Us and Contact Us. Account, appointment and legacy aliases use the existing customer application.
- Shared central services can be explicitly enabled at a location. Central name/archive revisions update linked current catalogue rows; historical job/invoice/catalogue snapshots are retained. No name-based matching or grants are inferred.
- Public service descriptions reference those operational IDs. Booking offers, scheduling rules and prices continue to use the Go authority; no copied website price is treated as an approved charge.
- The shared service editors link to /content. Separate exact capabilities control catalogue presentation and portfolio/site publishing. Image upload/edit/order/draft/publish/unpublish uses PostgreSQL and transactional auditing.
- Homepage and Portfolio use the same published gallery. Eighteen current public-site photos are staged in portfolio-import.json for an authorized user's reviewed import, without automatic database insertion or publication.
- Canonical/social metadata is prepared for dapperdetailing.ca. Default robots/noindex blocks prelaunch indexing. DAPPER_PUBLIC_INDEXING=enabled is a launch configuration, supplied at build/start only after approval; sitemap then contains public pages, excluding internal/account/API paths.
- Native PostgreSQL/API regressions, vet, isolated web builds/typechecks, source checks, built-route checks and bounded desktop/mobile browser checks passed. Exact boundaries are in SITE_AUDIT.md and verification/.

## Required coordinated activation

1. Finish/review role migration44 and its transactional authorization integration with the role owner. Retain content.manage and central-catalogue scope checks. Do not generate direct grants as a workaround.
2. Confirm immutable applied43 checksum9612701def2fd67638a7d2ab6fc1fa9e7ff9a74a5b699e97db243caf33471431. Central-link schema is45, not an edit to43. Payment candidate42 remains outside this lane. Source validation needs the registered migrations; do not restart the new API against the earlier schema.
3. Agree a shared local maintenance window, use the existing guarded encrypted backup/restore/migration/build/runtime procedure, and verify all existing Master/Workspace routes as well as the new public routes. This chat has not performed that shared transition.
4. Select the real publishing principal/scope and deliberately assign supported capabilities through the authorized administration flow. Configure the public site key dapper for the correct location. No default privileged user or automatic account adoption.
5. Review actual catalogue mappings, marketing descriptions, legacy price/duration discrepancies, offer terms and service availability. Publish approved offers using the existing booking authority. Configure real customer enrollment/ownership links and verify account recovery/email delivery.
6. Review the18 source photos in /content, set useful titles/alt text/categories/order and publication permission, then publish desired entries. New uploads use the same workflow. Verify edits/unpublish persist and propagate to both public galleries after reload.
7. Run authenticated browser acceptance against the combined secure origin: service edit through both dashboards, central rename propagation, customer-owned vehicles/offers/time requests/status, media lifecycle, revoked/foreign-role denial, failure recovery and mobile keyboard/focus.

## Before any live launch

Approve migrated legal pages, final content/contact/hours, real prices/durations/deposit/cancellation policy, actual sender/provider configuration, account recovery, backup/recovery/monitoring, asset rights, redirect inventory, accessibility and measured mobile performance/Core Web Vitals. Server-rendered published catalogue/gallery content and SEO coverage need final acceptance alongside the client-side projection.

Publishing/cutover requires explicit launch approval and a reviewed deployment/rollback plan. No WordPress, DNS, hosting, real customer records, charges or external messages were changed by this website work.

## Reproduce

- Generate pages/assets: python build-pages.py (main website folder).
- Isolated website-only preview: node serve.mjs on4174. Account/booking show an explicit503 connection page there; it is not an auth proxy.
- Build review: python prepare-validation.py, create the documented node_modules junction, then next build --webpack in .cache/website-validation/dapper-platform/apps/web. This avoids the running app's .next.
- Native API/DB: scripts/test-postgres.ps1 uses private credentials and disposable schemas.
- Source/behaviour checks: verification/check-source.py,check-pages.py,check-public-site.mjs.
- Built HTTP checks: verification/check-runtime.py against isolated Next on4175. No API proxy or synthetic authenticated browser session is used.


## Final activation hold
Automatic approval review blocked the guarded shared stop/migration because exact runtime/schema changes require explicit authorization. Last shared root read:HTTP500; DB41/43. No shared stop/migration ran. Concrete reviewable procedure is prepared in existing stop/migrate/start scripts; ingress verification source now expects the public homepage plus authenticated Workspace. User approval is required for this local repair; no live WordPress/DNS/hosting action is included. Final source/build/test evidence is production-increment.json.
