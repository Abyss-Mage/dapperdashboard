# Pricing, duration, deposit and policy evidence (legacy BookingPress + public website)
Collected 2026-10-24 at the owner's request to answer the open questions in 04 Delivery/PRICING_DURATION_DEPOSIT_PLAN.md.
STATUS: CANDIDATE EVIDENCE ONLY. Nothing here is approved Dapper behaviour (DEC-04/05/06 remain PENDING_CLIENT_APPROVAL). Nothing was entered into the application. Sources: `dapper-platform/apps/api/data/bookingpress-reference.json` and `bookingpress-extras-reference.json` (captured 2026-10-21, status reference_only_pending_client_approval, currency "not_explicitly_confirmed"), `website-reference.json`, and live pages on https://dapperdetailing.ca/ read 2026-10-24 (home, contact-us, terms-conditions, refund-and-cancellation, book-appointment; the booking form itself did not load for the reader).

## Q1 Tax and currency
- Currency: no page states it for prices ("$"). The Refund & Cancellation page states the cancellation fee as "CAD $100", which suggests CAD. The legacy export marks currency unconfirmed.
- Tax (HST/GST), tax-inclusive vs exclusive, rounding: NOT stated on any source. Terms say website prices are estimates that can change with vehicle size, condition, contamination, extras, accessibility. Needs the owner's answer.
## Q2 Deposits and payment
- Deposit amount/percentage: NOT stated. Terms: payments go through Square; payment may be due at booking, before service or after completion "depending on the service and arrangement". Refund page: deposits, booking fees or advance payments MAY be non-refundable after cancellation (subject to law).
## Cancellation, reschedule, no-show, refunds (refund-and-cancellation page, section numbers as published)
- Customer cancellation of a confirmed booking: a CAD $100 fee MAY apply (3.1); cancellation must use approved channels (3.2); repeated cancellers may be refused future bookings (3.4).
- Reschedule: before the appointment may be accommodated without penalty, subject to availability (2.1). No notice window is stated.
- No-show: may be treated as a cancellation and may incur the same CAD $100 (4).
- Company-cancelled (weather, equipment, safety): reschedule free or full refund of prepaid amounts (5).
- Completed service is final; refunds only if paid but not performed, duplicate payment, company-cancelled and customer declines reschedule, or law requires (6). Report dissatisfaction within 24 hours (7). No refund where results are limited by vehicle condition (8). Contact the company before a chargeback (9). Liability capped at the amount paid for that service (10).
- Not stated: the time window for the fee, whether the fee applies to rescheduling, partial refund amounts, what "approved channels", "reasonable access" or "repeated" mean.
## Q3 Equipment categories (website navigation + legacy categories)
2 Wheeler; 4 Wheeler: Hatchback, Sedan, SUV, Pickup, Van; Ceramic Coating; PPF; Aircraft (Small/Medium/Large); Boat (Small/Medium/Large); also in the legacy export: Window Tint, "Others" (headlight restoration, claybar wax and polish, golf cart, small and big off roader, fabric guard).
## Q4 Resources, hours, location
- Hours (contact page): "8:00 AM – 9:00 PM", "Open 7 Days a Week" (no per-day breakdown). Candidate weekly hours Mon–Sun 08:00–21:00 in America/Toronto (legacy source timezone) — needs approval.
- Address: 660 The Kingsway, Peterborough, ON K9J 7C8; phone +1 (705) 977-6217; info@dapperdetailing.ca.
- Walk-ins welcome "based on availability"; mobile service ("We'll come to you") and at-shop both mentioned; no service area, radius, travel fee or arrival window stated. Number/identity of bays or crews and which services each can do: NOT stated anywhere.
## Q5 Who reviews/publishes prices: not in any source.

## Legacy catalogue (79 services, 12 categories, 228 extras). Price in dollars as exported; duration as exported
| Category | Services (price, duration) |
|---|---|
| Hatchback | Express Interior 79 (30m); VIP Interior 169 (3h); Luxury Full Body 229 (3h); Exterior Wash 75 (1h); Ultimate Protection 379 (6h) |
| Sedan | Express Interior 89 (30m); VIP Interior 189 (3h); Luxury Full Body 249 (3h); Exterior Wash 75 (1h); Ultimate Protection 429 (6h) |
| SUV | Express Interior 99 (30m); VIP Interior 189 (3h); Luxury Full Body 269 (3h); Exterior Wash 99 (1h); Ultimate Protection 449 (6h) |
| Pickup | Express Interior 99 (30m); VIP Interior 189 (3h); Luxury Full Body 269 (3h); Exterior Wash 99 (1h); Ultimate Protection 449 (6h) |
| Van | Express Interior 119 (30m); VIP Interior 209 (4h); Luxury Full Body 289 (5h); Exterior Wash 99 (1h); Ultimate Protection 449 (6h) |
| 2 Wheeler | 5 year Ceramic Coating 299 (2h) |
| Ceramic Coating | Shield 3-yr 699 (6h); Fortress 5-yr 999 (6h); 3 stage Paint Correction 1199 (6h); Armour up 1999 (6h) |
| PPF | Front: Hatchback/Sedan 1499, SUV/Pickup/Van 2199 (1 day); Full: Hatchback/Sedan 4999, SUV/Pickup/Van 5999 (2 days) |
| Window Tint | Single 59 (30m); Standard 4-window 219 (60m); Full 5-window premium 289 (90m) |
| Aircraft | Small: Interior 250 (3h), Exterior 450 (5h), Signature Full 625 (8h), Ceramic 800 (10h), Paint Correction 1000 (12h). Medium: Interior 450 (5h), Exterior 750 (8h), Full 1150 (12h), Ceramic 1200 (15h), Correction 1500 (18h). Large: Interior 1000 (8h), Exterior 1500 (12h), Full 2100 (18h), Ceramic 2000 (24h), Correction 2500 (24h) |
| Boat | Small: Interior 270 (2h), Exterior 395 (2h), Full 665 (4h), Ceramic 400 (4h), Correction 600 (5h). Medium: Interior 500 (3h), Exterior 550 (4h), Full 830 (7h), Ceramic 650 (6h), Correction 850 (8h). Large: Interior 600 (5h), Exterior 700 (6h), Full 960 (10h), Ceramic 1300 (10h), Correction 1500 (12h) |
| Others | Headlight Restoration 49 (exported as 1 minute, flagged); Claybar Wax and Polish 119 (1 minute, flagged); Fabric Guard 50 (1 minute, flagged); Golf Cart 70 (30m); Small off Roader 150 (30m); Big off Roader 180 (30m) |
Extras: 228 add-on rows attached to services, price range 25–799, ALL exported with missing/zero duration (flagged). Most common: Pet hairs 29/25 rows, Winter Salt Removal, Headlight Restoration, Claybar, Wax, Ceramic Coating, Engine Bay, Fabric Guard.

## Discrepancies to resolve before anything is approved
1. Home page "starting from" prices do not match the catalogue minimums: Sedan/SUV/Pickup "from $189" (cheapest catalogue item is the $75–99 Exterior Wash; $189 is the VIP Interior price); Ceramic Coating "from $499" vs catalogue minimum $699; Aircraft "from $249" vs $250.
2. Three services have a 1-minute duration placeholder (Headlight Restoration, Claybar Wax and Polish, Fabric Guard); every extra has no duration.
3. Boat Small Ceramic Coating (400) is cheaper than Small Exterior (395 vs 400 is close) but Medium Interior (500) vs Medium Exterior (550) ordering differs from Small/Large; confirm these are intended.
4. Contact page hours (08:00–21:00 daily) are the only hours source; walk-in and mobile availability against those hours is unstated.
5. Refund page says a fee "may" apply; the actual fee rule, window and whether it applies to reschedules are undecided.

## Addendum: owner's answers and the full BookingPress export (2026-10-24, later)
Owner answers: (1) Tax is added separately (prices are tax-exclusive) and Master needs a section to add and edit taxes per franchise. (2) Deposit: "don't know" -> stays unconfigured. (3) Resources: "no idea and no limit" -> no resource list yet; owner wants no booking limit. (4) Prices are reviewed/published by Super Admin and Owner (those two roles).
Source: owner-supplied WordPress BookingPress export (Downloads/4VZGECfO7b.txt, site dapperdetailing.ca, add-ons active: pro, coupons, service extras, tips, tax, invoice, conversion tracking) plus a screenshot of Manage Services. Customer rows and settings are NOT copied here. The export matches the earlier reference (79 services, 12 categories, 228 extras).
Findings from the full export:
- Visibility: 70 services show on the site; 9 are hidden (greyed in the screenshot): Headlight Restoration, Claybar Wax and Polish, Fabric Guard, Golf Cart, Small off Roader, Big off Roader, Single/Standard 4-/Full 5-Window Tint. The three 1-minute duration placeholders are all among the hidden ones.
- Tax: the invoice template shows an HST line at 13% added after subtotal and discount (and a tip line), i.e. tax-exclusive pricing, consistent with the owner's answer. The template also prints a business tax registration number (not copied here). Candidate only; owner approves rates per franchise in Master.
- Buffers: every service has 0 minutes before and after.
- Capacity per service (legacy "max capacity"): 4 for most car services, ceramic packages, tint and golf cart (33 services); 1 for all aircraft, boat and PPF services (40); 2 for 2-Wheeler ceramic, Headlight Restoration, Claybar Wax and Polish; 10 for the two off roaders; 3 for Fabric Guard. The owner said "no limit"; legacy shows these limits. Needs a decision.
- Notice windows to book, reschedule and cancel are set to "inherit" a global setting that is not in the export (so lead time and cancellation notice are still unknown).
- Hours: per-service work hours, days off and special days are empty; hours live in a global setting not in the export. Website contact page (08:00-21:00 daily) remains the only source.
- Extras: all 228 have duration 0 and quantity 1 (one allows 2); no per-service deposit settings exist in the export.
- Booking form fields: first/last name, email, phone, note, terms agreement, a "Service Location" radio (mobile vs shop) and a required address field for where the team should visit.
- Coupons: 18 historical coupons (percentage and fixed, e.g. 15-20%, $25, $200 off ceramic, 100% apology codes); most are expired. Promotion authority is undecided, so none are imported.
