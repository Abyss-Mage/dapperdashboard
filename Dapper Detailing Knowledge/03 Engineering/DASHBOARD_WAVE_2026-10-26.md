# Dashboard changes from the owner's list (2026-10-26)

Status: source + Go/PostgreSQL tests (all packages pass) + runtime checks in the browser on this machine. Not tested on a phone or another computer. Migration 65 (`owner_wave_2026_10.sql`) was applied to the dev database with the encrypted backup and isolated restore proof.

Done
- Customers: email/phone beside the name; Export (CSV of what is on screen, formula-safe).
- Pages renamed/added: Bookings -> Calendar; Online requests -> Appointments (all appointments with filters, row opens a popup, waiting online requests on top); Jobs page removed; Staff photos split into Dealership tags and Customer cars, each with filters (search, posted by, dealership or pre/post, dates).
- Staff availability: one row per person (name, never email), popup with the week table and Approve/Reject. `/access` members now carry `name`.
- Map: tiles are fetched by the server (`/api/v1/map-tiles/z/x/y`, signed-in only, identifying User-Agent, cache) because OpenStreetMap blocked direct browser requests ("Access blocked"). Staff hours table white header cells fixed.
- A booking now raises a draft invoice in the same transaction (service + extras at catalogue price, 0.00 if none; franchise taxes; skipped quietly when prices include tax). Prices must be checked before issuing.
- Every booking is linked to its location's staff screens (`employee_booking_worksites`, dealership NULL), backfilled for older bookings. Consequence to review: dealership-focused employees already see site-level bookings, so they now also see main-dashboard bookings of that site.
- Quotes form: pick the booking request (fills customer and vehicle) instead of typing UUIDs, labelled line table with totals.
- Square application keys for testing (sandbox) and live (production) are entered in the dashboard; the secret is sealed with the server key, write-only, in use immediately, reloaded at start. The server-set keys still work until replaced.
- Dealerships: tax exempt, pay-within and the Invoice Ninja feature removed (Go files, routes, UI); per-car rate; several contact people (first is the primary, kept in the old columns).
- Services: time to complete in minutes (central and franchise); the application keeps a base booking rule per main service and resource in step with it.
- Opening hours: booking slots (capacity per resource; creates the franchise's booking resource if none) and a holiday calendar (whole-day closures on every resource; existing bookings are not cancelled).
- New franchise: full address required; contact people (up to 10).

Not done / open
- Add-on durations are not summed into a booking's time (rules for add-on combinations are still explicit).
- Slots are one number per franchise applied to every resource, not per time of day.
- The older `/workspace` structure editor (StructureEditor.tsx) still posts franchises without contacts and will now be refused if the address is incomplete.
- Square keys: live mode and the redirect URL still need to be set up in Square; sandbox/live OAuth connection itself is untested against Square.
- Phone-size and screen-reader checks of the new popups and calendars.
