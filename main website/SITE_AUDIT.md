# Current-site audit and replacement implementation

Reviewed 2026-10-23/24. This is the production website implementation lane, not a demo-product brief. WordPress, its bookings, DNS and hosting were not modified.

The current [home navigation](https://dapperdetailing.ca/) exposes ten service pages, Portfolio, About Us, Contact Us, customer Account and appointments. The replacement keeps those page identities and the established Porsche studio as the homepage; it does not reproduce the older WordPress visual layout.

| Before | After | Why |
| --- | --- | --- |
| WordPress public pages and a separately maintained BookingPress catalogue | Shared Go/PostgreSQL service authority, with public descriptions attached to operational service IDs | A service should not require independent edits in the website, Workspace and Master |
| Separate central and location service records without a relationship | Explicit central-service imports create linked location records | Central name/archive revisions propagate; independent historical commercial snapshots stay intact |
| Portfolio page imagery embedded in page content | Published gallery API shared by homepage and Portfolio, with a permission-controlled image manager | Staff can upload, edit, order and unpublish images without changing source |
| Generic appointment links | Existing account-owned offers, availability and booking-request workflow | Customer identity, prices, terms and confirmation status stay server-authoritative |
| Small-screen navigation hidden | Accessible Menu disclosure, Escape/return focus and persistent contact/account paths | The full page structure remains reachable on a phone |

## Preserved public routes

`/`, `/services/`, `/2-wheeler-detailing/`, `/hatchback-detailing/`, `/sedan-detailing/`, `/suv-detailing/`, `/pickup-detailing/`, `/van-detailing/`, `/ceramic-coating/`, `/ppf/`, `/aircraft-detailing/`, `/boat-detailing/`, `/portfolio/`, `/about-us/`, `/contact-us/`.

Existing `/my-bookings/` and `/book-appointment/` enter `/account` and `/booking`; `/home` redirects to `/`. Redirects use relative locations so an HTTPS gateway does not send a customer back to an internal HTTP address.

The [About page](https://dapperdetailing.ca/about-us/) supplies the business journey and public leadership names. The [Contact page](https://dapperdetailing.ca/contact-us/) supplies contact/address/hours. Replacement copy is adapted; actual business details and opening hours still need final content sign-off before cutover.

## Important live findings

- The live booking form renders four stages in the browser: service, date/time, details and summary. A crawler's sparse page text is not evidence that booking is missing. Inspection was read-only; no slot, account or transaction was submitted.
- Pricing is inconsistent between marketing and booking. For example, the [sedan page](https://dapperdetailing.ca/sedan-detailing/) lists an exterior wash at $59; the [live appointment form](https://dapperdetailing.ca/book-appointment/) displays $75. Legacy values remain references, not automatically approved new-system rates or scheduling rules.
- The current [account entry](https://dapperdetailing.ca/my-bookings/) includes login, password recovery and verification-code controls. The new account already uses the managed session/customer-ownership authority, but recovery/sender configuration and actual delivery acceptance remain unfinished. No fake recovery success was added.
- Eighteen unique public portfolio image sources were inventoried from the current HTML and locally prepared with per-source and output hashes. They are selectable in the new dashboard for reviewed import; they have not been inserted or published automatically. All active gallery entries come from the database, including those shown on the homepage.
- Existing [privacy](https://dapperdetailing.ca/privacy-policy/), [terms](https://dapperdetailing.ca/terms-conditions/) and [refund/cancellation](https://dapperdetailing.ca/refund-and-cancellation/) pages also need approved migration before cutover. Their legacy claims and terms were not silently adopted as new application policy.

## Shared management and permissions

`/content` is linked from the shared service editors used by both dashboards. Existing catalogue editors remain the place to create/edit the actual main services and add-ons.

- Organization `catalogue.manage`: central service editing.
- Organization `catalogue.read` plus exact-location `catalogue.manage`: explicitly enable a central service at that location.
- Exact-location `catalogue.manage`: public service category, description and publication revision; the service name is always read from the operational record.
- Exact-location `content.manage`: configure the public location and upload/edit/order/publish/unpublish portfolio media. Publishing requires an explicit image-permission review. No capability was automatically assigned to any user.

Central identity changes with linked records and archives with active dependent location add-ons are rejected. Every successful metadata/media change records an audit event transactionally; stale versions return a conflict. JPEG/PNG uploads are bounded, decoded and re-encoded to strip embedded metadata/payloads; SVG and arbitrary remote-image fetches are rejected. Draft images require scoped management access; anonymous image access stops when the image or site is unpublished.

The separate role-management worker owns migration 44. Its role-only transaction/central-grant integration needs acceptance before claiming that assigning a named role alone enables all publishing mutations. Existing explicit grants remain enforced; no direct grants were synthesized to hide this integration gap.

## Evidence and deployment boundary

Native disposable PostgreSQL/API suite: 9 tested packages PASS; final run35788, PostgreSQL48.840s, workspace15.816s. Go vet PASS. A later focused409 dependency-response test PASS. Isolated Next production builds/type checks PASS. Fourteen generated pages pass route/anchor/ARIA/local-asset checks. Twenty-six built HTTP route/asset/relative-redirect checks PASS. Browser checks at1440 and390 widths show no horizontal overflow/broken source images on inspected directory/detail/about/contact pages; mobile Menu Escape/focus and public-data error/retry states pass. These are local checks, not authenticated browser mutation, real-device, provider or production acceptance.

Read-only shared DB inspection found an early migration43 already applied (not by this website increment). Its exact original source/checksum was restored; later central-link SQL is now migration45. This chat did not rewrite DB checksums or migrate/restart the shared application. Coordinate44/45 deployment with the other owners, preserve an encrypted backup/restore path, then verify the combined runtime before provisioning real publication settings or launching.

See [PRODUCTION_PLAN.md](PRODUCTION_PLAN.md), `verification/public-page-checks.json`, `public-runtime-checks.json`, `public-browser-checks.json` and `portfolio-import.json`.


## Final integration update — 2026-10-24

The other worker added role queries to transactional/central authority; the earlier direct-only observation is superseded. Independent lock/revocation concurrency acceptance is still open. Shared DeleteService/DeleteCentralService now retain publication metadata and archived linked identities; new native regressions pass. The18source-photo archive is private to the Go content API and requires content.manage; it is excluded from Next public assets. Final native suite13366 PASS9packages (PG42.658s/workspace14.893s), final vet PASS, isolated build56942/TypeScript PASS,28HTTP checks PASS.

The final shared local homepage read returnedHTTP500 and DB still showed41/43. A concrete guarded repair was prepared: identity-verified stop, encrypted backup+isolated restore,44/45migration, shared rebuild/restart with existing settings, then updated pinned-CA ingress verification. Automatic approval review rejected the stop/migration operation because that exact shared runtime/schema change was not explicitly authorized; the stated risks were service interruption and schema/data changes. No stop/migration ran. Shared activation therefore awaits user approval. Isolated preview remains available; it is not proof of shared runtime readiness. No live WordPress/DNS/hosting cutover or real grants/publications were performed by website.
