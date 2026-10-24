# Booking wizard reference (from the owner's screen recording, 2026-10-24)

Source: a 71-second recording of another company's public site booking pop-up (deltacardetails.ca). It is a **UX reference only**: no text, images, prices or branding are to be copied. The owner wants the same style of guided booking for the public website, the Workspace (staff) and Master (admin).

## What the reference does (8 steps in one pop-up with a progress bar)
0. **Welcome**: "Have you booked with us before?" Enter a phone number to load saved cars and details, or "I'm new" (no account needed).
1. **Service**: three big choices (detailing, coating, tinting) with one line each.
2. **Location**: Mobile (we come to you) or In-shop (drop-off); map with the shop address; hand-off choice (customer drops off or valet pick-up).
3. **Vehicle**: six picture tiles (coupe, sedan, 5-seat SUV, 6+ seat SUV, truck, van); year / make / model; add up to four vehicles.
4. **Package**: cards with picture, short description, duration, price, badges (Most popular, Most value, seasonal special).
5. **Add-ons**: optional checklist with "+$" prices ("+$60+" for variable), per-seat items; skippable.
6. **Day and time**: month calendar (today marked, limited days marked, unavailable greyed), then time-slot buttons; a running total (tax included) stays visible at the bottom.
7. **Your details**: name, phone, email, notes, "were you referred?" (referrer name and phone), summary of amount due now (30% deposit), balance on the day, "or pay cash on the day".
8. **Payment / confirmation**.
Good patterns: one question per screen, big tap targets, Back/Continue always in the same place, running total, no account needed, phone lookup for returning customers, validation message next to the field.

## How it maps to Dapper (what exists, what needs a decision)
- Exists: central catalogue with prices (main services by vehicle type, add-ons), scheduling engine with real availability, booking offers (price/deposit/terms gate), customer accounts, Square sandbox, invoices.
- Needs the owner's decision (do not assume): mobile service and valet (are they offered?), deposit rule (reference shows 30%; Dapper deposit is still unset), referral reward (no approved policy), vehicle-size tiles vs Dapper's own types (Hatchback, Sedan, SUV, Pickup, Van, Aircraft, Boat...), service groups (detailing / PPF / ceramic / other), whether the public site takes card deposits online.
- Plan: one shared wizard component used by the public site (customer), Workspace (staff, "walk-in") and Master (admin "New booking"), all calling the same booking API. Staff and admin variants skip marketing steps and can pick any time.

## Owner decisions (2026-10-24) and what is built
- Mobile and valet: both offered. 30% deposit for online bookings. Referral: ask who referred, record name and phone only (no reward). Vehicle types: cars (Hatchback, Sedan, SUV, Pickup, Van), 2 Wheeler, Aircraft S/M/L, Boat S/M/L.
- Slice 1 (source + Go tests + build; NOT yet clicked through in a signed-in browser): shared `BookingWizard.tsx` (7 steps: Service, Where, Vehicle, Package, Extras, Day & time, Details with price summary) used by Workspace walk-in and Master Bookings > New booking through `StaffBookingWizard.tsx`. New APIs: `GET /api/v1/employee/walk-in/availability`, `GET /api/v1/employee/walk-in/taxes`, `GET .../locations/{loc}/bookings/availability` (same engine as saving a booking). Master reserve now accepts `addonIds`; walk-in accepts `notes`. Vehicle, place (shop/mobile/valet + address) and referral are stored as lines in the booking notes (no new columns yet). Vehicle type and service group are read from the catalogue names ("Sedan - Express Interior Detail").
- Not built yet: the customer-facing public website version (needs a public booking API with phone verification for returning-customer lookup, rate limiting and Square card deposit), collecting the 30% deposit inside the staff flow, structured columns for vehicle/place/referral, up-to-4 vehicles, mobile/valet pricing rules.

## Slice 2: customer website booking (built 2026-10-24; migration 63; source + Go tests + build, NOT run end to end in a browser)
- Page `/booking` (public, no sign-in) shows the same wizard plus a Welcome step (phone check) and a Deposit step (30% by card or pay on the day). Public API under `/api/v1/public/booking/{site}`: `options`, `availability`, `square` (public card-form identifiers only), `verify/start`, `verify/check`, `book`. It only works for a **published public site** (`public_sites.published`); with none published every call answers 404 "not available".
- Server side: prices, tax and the 30% deposit are computed from the catalogue and franchise taxes, never from the browser. A booking holds the time (channel `online`, status pending), reuses the customer with the same email (details not overwritten), issues an invoice for the full total (uses a number from the issuer sequence), then charges the deposit through the existing Square invoice-payment path. If the deposit fails the invoice is voided and the time released. Repeating the same request returns the same booking. Unpriced services are not offered online. Per-visitor limits: 6 bookings / 10 min, 5 texts / hour, 240 reads / min.
- Identity: the website acts as principal `system:website` with a minimal grant list at the published location (created on first use; hidden from People & Access).
- Phone check: code stored hashed, 10 minutes, 5 tries, 3 texts per number per hour; saved name/email shown only after the right code. **No SMS provider is connected**, so `phoneVerification` is false and the pop-up skips the Welcome step.
- Square card deposit needs the Square binding saved in Master (Invoices & Payments > Card payments: sandbox, merchant MLCPSF5FV5NG6, location L66VH7B9XRWCR, credential reference `sandbox-default`) and the application ID matching that Square account.
- Tests: `TestPublicBookingEndToEnd` (isolated schema): publish gate, priced-only offers, availability, totals (13% tax example: 28,250 total, 8,475 deposit), idempotent repeat, double-booking refused, customer reuse, abort releases time, unpriced refused, phone code rules.
