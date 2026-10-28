# Public website rebuilt as Next.js pages (2026-10-28)

User direction (2026-10-28): make the main website look and feel like a Next.js + GSAP site, keeping the scroll-trigger story section as the exception. The user chose real Next.js pages (not upgraded static HTML) and picked the "50/50 Tape Line" visual direction from a decision round.

## What changed
- `dapper-platform/apps/web/app` now has two root layouts: `(app)` holds every dashboard/account/booking route unchanged (moved, imports untouched); `(site)` holds the public website with its own `site.css`, so neither surface's CSS reaches the other. Navigating between them is a full page load.
- Public routes are React pages: `/`, `/services`, the ten category pages (`/[service]`, static params only), `/portfolio`, `/about-us`, `/contact-us`. `/studio` is still the generated static document (`app/studio/route.ts`). `app/route.ts` and `app/[publicPage]` were removed; `/home`, `/my-bookings`, `/book-appointment` redirect in `next.config.ts`.
- Copy comes from `(site)/content.ts`, carried over from `main website/build-pages.py`. No prices, results or claims were added. Before views are labelled "Before · illustration" and the footer says they are made from the same photograph.
- Motion: GSAP 3.15 via `@gsap/react` (ScrollTrigger, SplitText), Lenis 1.3.4 smooth scroll (off for touch and reduced motion), tape-sweep page transition in `(site)/template.tsx`. New dependencies: `lenis`, `@fontsource/barlow-condensed`.
- Home story: `story.js` + `index.html` story markup ported to `_components/Story.tsx`. The renderer is still `main website/home-stage.js` → `home-frames.js` (if `assets/frames/manifest.json` exists) or `home-scene.js`; both now release their canvas/WebGL context on a `dapper:dispose` event so client navigation away and back works. The `three` import map is in the `(site)` layout head.
- Design record: surface brief `.impeccable/surfaces/dapper-platform-apps-web-app-site.md` (direction contract, seed f85950ca3cfea9bd).

## Evidence
- Static: `tsc --noEmit` clean; `next build` succeeds (26 routes); design detector found nothing.
- Runtime (this Windows machine only): headless Chrome captures of all six page types at 1440 and 390 wide with no page errors; built-in browser checks of the mobile menu, client navigation, and the 3D story restarting on return to home.
- Filled states (portfolio, packages, instant quote) were only seen with a synthetic test fixture injected by the capture script; the local database has no published content and online booking is off, so live data is unverified.
- Not tested: real phones, Safari/Firefox, live provider data.
- Independent design review: first verdict "rebuild" (imitation swirl overlay, mislabelled reused photos, no drag cue, category-default close panel, red overuse). After two fix rounds the final verdict is "fix" with three items open: masked raster before-plates not produced (no image tool in this environment), and the home story's step labels / duplicate CTA bar, which were left because the story was scoped out by the user.

## Open
- Only four real studio photos exist; vehicle-type panels and category heroes reuse them. Real photos per vehicle type are needed.
- The AI frame sequence for the story is still to be produced (see HOME_FRAME_SEQUENCE_2026-10-25).
- `build-pages.py` still generates the old static pages into `pages.generated.ts`; only `studio` is served from it now.
