# Dashboard and workspace tester feedback: batches A and B (2026-10-31)

Source: the tester's 29-point list. Evidence level: **source present, TypeScript and Go tests pass, deployed to the production server. No logged-in visual check and no live-provider evidence.**

## Batch A: catalogue and services
- One line per service (name, Main/Add-on, price, time), 17 to 18 px, no small text. Time to complete takes minutes, hours or days (stored as minutes, 7-day maximum unchanged).
- Duplicate action on a service.
- Add-on offered with several main services: the form ticks main services and creates one add-on row per service (same name and price). Edits apply to every row with the same name. The database still allows one parent per add-on; booking and franchise copies depend on that.
- Per-service slot settings ("Slots" button, franchise services): calendar, slot length, gap before and after, bookings at the same time. These are the existing scheduling rules (rule with no add-ons). Days and hours come from the calendar's weekly hours; a service needing its own hours needs its own calendar.
- Employee franchise assignment no longer shows timezone or partner dealership (timezone defaults to America/Toronto for new assignments).
- Dashboard CSS font sizes scaled about 1.3x (a page zoom would misplace popovers).

## Batch B: bookings, appointments, invoices
- New bookings default to `confirmed`, shown as "Approved". Open bookings can go directly to Completed or Cancelled. Status filter: All, Approved, Completed, Cancelled.
- Appointment shows its invoice: "Dues pending $X" or "Paid $X, Pending $0.00".
- Mark completed opens a dialog: "Cash received" issues a draft invoice if needed, records the remaining amount as cash (idempotency key `complete-<booking>`) and completes; or "Complete, money still owed". Only cash is recorded (the backend accepts cash directly; card goes through Square). E-transfer is not added: payment methods are a business decision.
- Employee rule: an employee assigned to a franchise without `scheduling.manage` may only mark walk-in bookings completed or cancelled (API `POST /api/v1/employee/bookings/{id}/status`, plus the staff status route). Employee roles therefore need `bookings.manage`.
- Backdated bookings: the staff "+ New appointment" form has no date limit; the capacity wizard still offers only open future slots.
- Invoice layout redesigned (red accent, paid or dues banner, Pending total). Invoice opens inside the appointment with print or save as PDF.
- The staff workspace no longer has the separate Invoices list. The owner dashboard Invoices tab remains.

## Not done yet
- Batches C, D, E (customers, dealerships page, quotation client name, account area, staff hours copy, calendar sync, GTM, Meta Pixel and Mailchimp scripts, franchise vs company invoice template differences).
- Reschedule appears only for bookings holding a calendar reservation.
- Website enquiry inbox paused.

## Batch C: customers, dealerships, quotations (migration 66)
- Customers list: filters (name/phone/email search, booked date range, service, booking status), larger phone and email, booking count, address. Customers that back a dealership account are hidden from the list (they live on the Dealerships tab).
- Address: `bookings.service_address` (staff booking form, mobile jobs) and `customers.address` (remembered, editable). Online and wizard bookings do not capture an address yet.
- Quotations: `request_id` and `vehicle_id` are optional; staff quote a client by name (optional phone and email) and the customer is found or added. Deposit field removed (new quotes save 0). Quotes without a booking request cannot use the existing fulfil-to-booking step.

## Batch D: accounts and staff hours (migration 67)
- Two-factor sign-in is optional for everyone, administrators included.
- Signed-in users can change email and password (current password required; every session ends afterwards).
- Emailed password reset: `/reset-password`, `POST /api/v1/public/auth/password-reset/request|confirm`, one-hour single-use tokens (SHA-256 stored), rate limited. **The email is only sent when DAPPER_SMTP_* is configured; on the server it is not, so no reset email is delivered yet** (a log line says so).
- People list: a member with roles but no employee franchise assignment shows "All franchises" or "Selected franchises" and is no longer "No franchise yet".
- Employee availability: the employee picks the week (this week up to 12 weeks ahead, Mondays) and can copy a day's hours to Mon to Fri or every day.
- Customer account area: edit name, phone, address; view and print invoices; change email and password; vehicles and bookings were already there.

## Open
- Overview "Access needed" (item 27) was not reproduced: tile gating follows role capabilities. Needs the account and the tile that shows it.
- Batch E: calendar sync, GTM, Meta Pixel, Mailchimp scripts, franchise vs company invoice differences.

## Batch E: integrations (migrations 68 to 70)
- Website tracking (owner dashboard > Settings > Website tracking, needs `content.manage`): Google Tag Manager container, Meta Pixel ID and the Mailchimp popup script address. Only identifiers are stored (format-checked; the Mailchimp address must be https on chimpstatic.com or list-manage.com). The public site's `Tracking` component reads `GET /api/v1/public/tracking` and loads only what is set. Nothing is configured yet. No cookie-consent banner exists; whether one is needed for these tags is a decision for the owner.
- Calendar subscribe links (Apple, Google, Outlook): private tokened ICS links per franchise (owner dashboard Calendar tab, needs `bookings.read`) and per employee (their account page). Only the token digest is stored; the link is shown once and can be turned off. Cancelled bookings are left out; summary is service and customer name, location is the service address. The older session-protected "Link calendar" download cannot work in a calendar app.
- Per-franchise invoice template: a franchise can have its own template (wording, colour, notes, footer, layout); without one it prints with the organization's. Editor has a "which template" switch. Logo per franchise is not done (no logo upload exists); quotations do not use this template (not changed).

## Follow-up: gaps closed (migrations 71 and 72)
- Website enquiry inbox: the public contact form posts to `POST /api/v1/public/enquiries` (honeypot field, 8 per hour per connection); administrators with `marketing.read` see an Enquiries tab (new, read, archived; `marketing.manage` to change). The enquiry belongs to the organization with the most franchises, because this installation serves one website. If the inbox is unreachable the form falls back to the email draft.
- Reschedule for bookings without a calendar reservation: `POST .../bookings/{id}/move` changes the time and writes it to the booking history (employees without `scheduling.manage` are refused).
- E-transfer: recorded as the existing `bank_transfer` payment method (no provider reference). Offered next to Cash in the Mark completed dialog.
- Service address on every booking path: the shared booking wizard (public, staff, employee) now sends `serviceAddress`; it is kept on the booking and remembered on the customer.
- Per-franchise logo: PNG, JPEG or WebP up to 512 KB, type decided from the bytes (SVG refused), needs `invoices.configure`; shown on that franchise's invoices.
- Still not done: quotations have no printable document, so there is no quotation template to differ by franchise. Overview "Access needed" for the Super Admin is not reproduced: on the server the Super Admin has all five overview permissions at organization scope.

## Overview "Access needed" on the server: cause and fix
- Cause: the dashboard sends the browser's time zone to the charts API. A browser in India reports the legacy name `Asia/Calcutta`; Go accepts it but the server's PostgreSQL (no legacy tz files) rejects it, so the charts call failed, and the overview then labelled the Bookings, Payments and Bookings-completed tiles "Access needed" even for the Super Admin (the permissions were fine).
- Fix: legacy zone names are mapped to current ones (`timezone_names.go`) before they reach the database, and a charts failure that is not a permission refusal now says "Unavailable / Use Refresh" instead of "Access needed". Verified by calling the charts and insights code on the server for the Super Admin with `Asia/Calcutta` (all succeed).
