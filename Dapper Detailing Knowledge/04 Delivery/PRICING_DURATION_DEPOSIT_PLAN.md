# Plan: service prices, durations and deposits (2026-10-24)
Status: PLAN ONLY, nothing built. Blocked on approved values (DEC-04 tax, DEC-05 prices/durations/resources, DEC-06 deposits and cancellation). No value is invented or taken from legacy WordPress/BookingPress defaults.

## 1. What already exists (do not rebuild)
| Need | Where it lives today | Notes |
|---|---|---|
| Service names, main/add-on tree, central vs franchise | catalog_services, central services; Master > Locations & Setup > Services | Names only, no price column by design |
| Duration, buffers, resource units per service combination | service_scheduling_rules (+ revisions), SchedulingEditor in Bookings | One rule per resource + main service + exact add-on set; versioned; reservations snapshot the rule |
| Price, tax, deposit, terms, cancellation terms, lead time, advance window, slot size, published | booking_offers (+ revisions), BookingOfferManager in Bookings | One offer per rule + equipment category; mode fixed or quote_required; deposit cannot exceed total; publishing requires reviewed_by/reviewed_at |
| Quote deposit | scoped_quotations.deposit_cents | Per quotation |
| Invoice lines | invoice_items entered by staff | Unit prices typed per invoice; no catalogue price lookup |
So "price + duration + deposit" is already modelled as rule (time) + offer (money + terms). The work is a guided way to enter many of them correctly, not a new schema.

## 2. What you must give me (one table, one row per sellable combination)
Resource (bay/crew) | main service | add-ons (exact set, or none) | equipment category (car, SUV, truck, boat...) | duration minutes | buffer before/after | resource units | mode (fixed price or quote required) | subtotal CAD | tax (see below) | deposit (see below) | customer terms text | cancellation terms text | earliest lead time | how far ahead customers may book | slot step minutes | publish yes/no.
Plus three policy answers: (a) tax: rate(s) and rounding rule, and whether prices are entered tax-inclusive or tax-exclusive (DEC-04); (b) deposit: fixed amount, percentage of total, or none, and per service or global (DEC-06); (c) cancellation/reschedule/no-show wording (DEC-06).
Legacy numbers that disagree with the new ones are NOT used; list which source wins when they differ.

## 3. Build steps (in order, each testable)
1. Tax and deposit configuration (needs DEC-04/06). Today offers store tax_cents and deposit_cents as amounts the caller computes. Add an approved, versioned per-franchise tax setting (rate, rounding) and deposit rule (fixed/percent) and have the server compute tax and deposit from the subtotal, so staff never hand-calculate. Migration 57, audited, behind structure.manage or invoices.configure. Without it, keep entering cents manually.
2. Price & duration sheet in Master (Locations & Setup > new "Pricing" view). One row per rule+category combining the rule and the offer: edit duration/buffers/units and price/tax/deposit/terms together, showing the computed total and deposit. Under the hood it calls the existing rule and offer endpoints (no new money logic), saves as draft, and shows validation per row (deposit <= total, duration > 0, add-on belongs to main, resource has hours).
3. Bulk import with dry run: paste or upload the table (CSV), the server validates every row and shows what would be created or revised, nothing is saved until you confirm; rows referencing unknown services/resources are listed, never auto-created.
4. Review and publish gate: a separate "Review" step records reviewer and time (existing reviewed_by/at); only reviewed rows can be published, and publishing is what makes an offer bookable. Revisions are kept; existing bookings keep the snapshot they were made with.
5. Invoice and quote integration: when staff add a catalogue service to an invoice or quote, pre-fill the line from the published offer (still editable, shows the source revision). Optional; invoices stay manual until you approve it.
6. Customer-facing check: booking page shows only published offers with hours, duration and the deposit due; end-to-end test with a Square sandbox deposit (still requires your sandbox run).

## 4. Guardrails
- Every number comes from you; the UI starts empty and the checklist in Locations & Setup gains a "Pricing reviewed" step that stays To do until published offers exist.
- Money is integer CAD cents; one currency; tax computed server-side once DEC-04 is approved.
- Changes create revisions, never overwrite history; audit events for save, review, publish.
- Deposits do not create payment capture by themselves; Square sandbox verification is a separate gate.
- Not legal advice: cancellation and terms text is yours; I do not draft policy.

## 5. Effort and order of work once values arrive
Step 1 (config + migration + tests) -> 2 (sheet UI) -> 3 (import) -> 4 (review gate UI) -> 5 -> 6. Steps 2-4 can start before DEC-04/06 if you accept entering tax and deposit in cents first, but I would rather wait for the tax rule so totals are right from day one.

## 6. Open questions for you
1. Prices tax-inclusive or tax-exclusive? Which province tax (e.g. HST) and rounding rule?
2. Deposit: fixed amount, percentage, or none; same for all services?
3. Equipment categories to price separately (list them).
4. Which resources (bays/crews) exist and which services each can do?
5. Who is allowed to review/publish prices (role or person)?
