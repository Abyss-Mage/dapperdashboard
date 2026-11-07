# Production public website and shared publishing

Updated 2026-10-24. Direct user direction: the existing cinematic website is the intended production homepage; add the current Services/Portfolio/About/Contact/Account/Appointment pages and shared dashboard service/image management. No separate website catalogue or demo-product direction.

## Source delivered

- Existing Next app root serves the cinematic document. Fourteen public detail/directory/information pages preserve the existing ten service category URLs and core information pages. Existing authenticated `/account` and `/booking` are reused; legacy aliases remain same-origin. No iframe, JSON identity/session, guest checkout, fake booking or copied operational database.
- Shared central catalogue imports at a chosen location create explicit identity links. Name/archive revisions propagate transactionally into linked current catalogue records and public projection. Existing history stays immutable. No mapping of old services is inferred by name.
- Exact `catalogue.manage` grants control service changes/public descriptions; exact `content.manage` controls site/media publishing. Organization `catalogue.read` is additionally required to import a central service at a location. No capabilities were provisioned for real users in this task.
- Shared editors in Workspace and Master link to `/content`. Its APIs persist revisioned service presentation, bounded JPEG/PNG uploads, portfolio title/alt/category/order and publication state, with transactional audit. Drafts are private; public media requires both published entry and site.
- Homepage and Portfolio share the published gallery API. Eighteen current public portfolio source images are prepared with hashes for reviewed import; none were automatically inserted/published. Customer data was not exported or migrated.
- Canonical/social metadata, public sitemap and robots controls are prepared. Prelaunch indexing stays disabled. Legal policy pages, final content approval, real prices/durations/booking terms, account recovery and provider acceptance remain gates.

## Verification

- Full native disposable-schema Go/PostgreSQL suite35788: nine tested packages PASS; postgres48.840s, workspace15.816s. Tests include central rename/archive propagation, preserved history, linked add-ons/dependencies, public/draft media boundaries, permission revocation/foreign scopes, stale revisions and failed-audit rollback. HTTP tests cover anonymous access, Origin, actor injection and projection boundaries.
- Go vet24096 PASS. Later focused dependent-central-service409 response test PASS (workspace3.550s).
- Final isolated Next build72510 PASS (webpack, TypeScript, root/public/account/booking/content plus robots/sitemap routes). No shared `.next` rebuilt.
- Fourteen generated pages pass source/route/anchor/ARIA/asset checks;18prepared images verified. Built HTTP checks pass28route/asset/relative-redirect/canonical/prelaunch cases.
- Browser1440/390 checks on representative directory/detail/about/contact layouts: no horizontal overflow or broken source images. Mobile Menu Escape/focus and public-data unavailable/retry controls pass. Authenticated browser CRUD, real device performance, provider delivery and production acceptance are not claimed.

Evidence: `main website/verification/{public-page-checks,public-runtime-checks,public-browser-checks}.json`; screenshots there; `main website/{SITE_AUDIT,PRODUCTION_PLAN}.md`; `portfolio-import.json` has original/output media hashes. Website source/runtime assets are copied reproducibly by `build-pages.py`; isolated build preparation is separate from the active runtime.

## Shared migration/runtime boundary

Read-only application inspection found43alreadyapplied with checksum9612701def2fd67638a7d2ab6fc1fa9e7ff9a74a5b699e97db243caf33471431. This website increment did not apply it. Restored the exact immutable43source; moved later central-link additions to45. Migration42 remains the other payment candidate;44 belongs to Antigravity roles. No database checksum rewrite, shared runtime restart, real grant/publication provisioning or live WordPress/DNS/hosting change was performed by website.

Coordinate role44/central-link45 activation after shared-owner acceptance and guarded backup/migration/build/restart. New service scanner requires45; do not launch it against the older schema. Role-only transaction/central-grant integration was observed incomplete while44was being developed; its owner must close that gap rather than synthesizing direct grants. Source/native passes are not acceptance of the entire role system.

Then configure the actual public location/principal, review catalogue/price/duration/terms, import/review/publish desired photos, and run authenticated combined-origin browser acceptance. Existing legal/privacy/recovery/cutover gates remain open. Explicit launch approval is still required for live cutover.


## Final integration update — 2026-10-24

The other worker added role queries to transactional/central authority; the earlier direct-only observation is superseded. Independent lock/revocation concurrency acceptance is still open. Shared DeleteService/DeleteCentralService now retain publication metadata and archived linked identities; new native regressions pass. The18source-photo archive is private to the Go content API and requires content.manage; it is excluded from Next public assets. Final native suite13366 PASS9packages (PG42.658s/workspace14.893s), final vet PASS, isolated build56942/TypeScript PASS,28HTTP checks PASS.

The final shared local homepage read returnedHTTP500 and DB still showed41/43. A concrete guarded repair was prepared: identity-verified stop, encrypted backup+isolated restore,44/45migration, shared rebuild/restart with existing settings, then updated pinned-CA ingress verification. Automatic approval review rejected the stop/migration operation because that exact shared runtime/schema change was not explicitly authorized; the stated risks were service interruption and schema/data changes. No stop/migration ran. Shared activation therefore awaits user approval. Isolated preview remains available; it is not proof of shared runtime readiness. No live WordPress/DNS/hosting cutover or real grants/publications were performed by website.
