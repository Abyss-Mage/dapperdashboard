# Public website redesign (2026-10-30/31)

User direction: colours from deltacardetails.ca, every section and word from the current dapperdetailing.ca (core values, partners, transformations, why choose us, process, testimonials), premium Next.js/GSAP feel, no WordPress/AI look. After several rejected directions the user asked for Delta's look mixed with atelierdetail.netlify.app. Delta's code, images and wording were not copied; the look (dark/red rounded panels on off-white, pill buttons, chips) was rebuilt in our own code with Dapper content.

## What shipped (local and server, 2026-10-31)
- `app/(site)` now uses `_v3/` (Home3, blocks, Chrome3, Gallery3, Packages3, v3.css), fonts Sora + Manrope. The tape-line design, the scroll-film Story/showroom hero and `/lab` prototypes were removed.
- Home: live WebGL clear-coat hero (ClearCoat), core values with rotating service names, service cards with "Starting from" prices (Sedan/SUV/Pickup $189, Ceramic $499, Aircraft $249 — user approved showing the current site's prices), partners (Angevaare Mazda, Jack McGee Peterborough, Subaru, Hyundai), 5 transformation videos, why choose us (6), how we do it (4), all 8 testimonials, booking call, footer; phone dock.
- Inner pages: Services (10 categories), 10 vehicle pages (published packages from the API or a contact prompt), Portfolio (18 photos + videos), About (timeline, founders, values, partners), Contact.
- Media downloaded with user approval from dapperdetailing.ca: main website/assets/{partners,transformations,services}, sources in assets/SOURCES-2026-10-30.md. Portfolio photos (main website/assets/portfolio) now served under /site/assets/portfolio.
- Category images: real photos where the vehicle type matches; otherwise other Dapper work with alt "Vehicle detailed by Dapper" (hatchback, van, boat).
- Bug found: exporting a constant named `process` from dapper.ts shadowed Node's `process` in Next's server build (TypeError reading __NEXT_PRIVATE_MINIMIZE_MACRO_FALSE). Renamed to `processSteps`.
- deploy.sh now skips files deleted but not yet committed.

## Evidence
- Static: tsc clean; next build passes, all public pages static.
- Runtime: local and server — all public pages, dashboards and media return 200; headless desktop/phone captures of home and all inner pages, no page errors. Not checked: real phones, Safari, WebGL on low-end devices.

## Added 2026-10-31: remaining Delta-style home sections (Dapper wording, not Delta's copy)
- Booking band "Ready for precise detailing?" (the current site's own line).
- "Your detail, booked in minutes": 3 booking steps + live availability calendars, one per booking resource, from the public API (`/api/v1/public/booking/dapper/options` and `/availability`, next 7 days). Online booking is switched off for the site today, so the calendars show a "book now or call" fallback until it is enabled in the dashboard.
- Mobile detailing ("We’ll come to you — no waiting rooms", the current site's line) with the driveway wash photo; no service areas claimed beyond Peterborough.
- "The finish, in motion": large transformation film + two side films, Instagram/YouTube links.
- "Get in touch with the team": a form that composes a ready-addressed email in the visitor's mail app (no public enquiry API exists; a server-side enquiry inbox would be new backend work).
- Local and server: built, all pages 200, new sections present in the served HTML; desktop/phone captures without errors.
