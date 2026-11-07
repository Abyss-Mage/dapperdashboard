# Dapper public website

**2026-10-28:** the public pages are now real Next.js routes in `dapper-platform/apps/web/app/(site)` (50/50 tape-line design, GSAP + Lenis). This folder still supplies the home story renderer (`home-stage.js`, `home-frames.js`, `home-scene.js`), the runtime assets copied by `build-pages.py`, and the static `/studio` page. The static Services/Portfolio/About/Contact documents generated below are no longer served. See `Dapper Detailing Knowledge/03 Engineering/PUBLIC_SITE_NEXT_REBUILD_2026-10-28.md`.

Updated 2026-10-24. This is the production public website implementation for the existing Go/PostgreSQL/Next.js application. The Porsche studio is the homepage; Services (ten category pages), Portfolio, About Us and Contact Us preserve the current public page structure. Account and appointment entry use the existing authenticated customer application.

Shared central-to-location services and public descriptions use operational IDs. The shared dashboard service editors link to `/content` for permission-controlled service presentation and portfolio uploads/edits/order/publication. Both public galleries use the same database projection. Eighteen original public-site photos are staged for reviewed import through protected Go routes; no source-photo archive is served from the production public asset tree.

See [SITE_AUDIT.md](SITE_AUDIT.md) for findings and evidence, and [PRODUCTION_PLAN.md](PRODUCTION_PLAN.md) for activation gates. Native API/PostgreSQL tests, isolated production builds/typechecks, page/route/asset checks and representative desktop/mobile checks pass within their documented boundaries. Shared activation, authenticated browser mutations, content/recovery/provider approval and production acceptance remain open. No live WordPress/DNS/hosting cutover was performed here.

## Local review

- `node serve.mjs` serves the website-only source preview on http://127.0.0.1:4174/. Its account/booking503 pages explicitly indicate that it is not an authentication/API proxy.
- `python build-pages.py` regenerates public HTML/Next route data and copies only runtime assets into the existing web app. Run the verification cleanup helper if older generated source-photo copies remain; originals are retained.
- `python prepare-validation.py` prepares an isolated copy for a Next production build, leaving the running app’s `.next` untouched. The built review instance runs on http://127.0.0.1:4175/ when started; it has no API proxy or synthetic session.
- Source/behaviour checks: `python verification/check-source.py`, `python verification/check-pages.py`, `node verification/check-public-site.mjs`. Built HTTP checks: `python verification/check-runtime.py`.

Public indexing stays disabled until explicit launch configuration and acceptance. Migration43 must retain its applied checksum; central-link schema45 and the separate role44 require coordinated activation. Do not launch new API source against the earlier schema.

## Retained history and attribution

[ASSET_SOURCES.md](ASSET_SOURCES.md) and the vendored licences retain all source credits. `QA_RESULTS.md`, `verification/studio-checks.json` and `verification/historical/README-prototype.md` describe the imported prototype and are historical evidence only. They are not current production acceptance.
