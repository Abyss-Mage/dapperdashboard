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

## Second tester round (2026-10-31, later): coupons
- Coupons (migration 75, owner dashboard > Money > Coupons; `marketing.read` to see, `marketing.manage` to change): date range or never expires, title, code (generate button), percentage or fixed amount, optional single customer, optional services, times allowed (0 = unlimited), on/off. A used coupon cannot be deleted, only switched off.
- Applying: staff type a code in the appointment's Payment panel (draft invoices only); customers type it in the online booking form. An invalid code (unknown, expired, wrong customer, wrong service, used up) stops the booking before any time is held. Tax is worked out on the reduced amount. The invoice stores `discount_cents` and `coupon_code`; its total is subtotal minus discount plus tax, and the printed invoice shows a Discount line.
- Not done: coupons are per franchise (not organization-wide); the staff and employee booking wizards have no coupon box (staff use the appointment panel); expiry dates use the America/Toronto calendar day.

## Reports (second list, item 18)

- New Reports tab (Every day group), shown to anyone with bookings, payments or customers read access.
- Three reports: Appointments, Revenue (net payments, per day), Customers. Filters: date range with quick ranges, service, status. Range limit about two years, 5,000 rows.
- API: `GET /api/v1/organizations/{organization}/locations/{location}/reports/{kind}?from&to&tz&service&status`. Access is checked per report (appointments need bookings.read, revenue needs payments.read and invoices.read, customers need customers.read and bookings.read).
- Download CSV is built in the browser (formula-looking cells are prefixed so spreadsheets show them as text). Download PDF opens a print page; the user chooses Save as PDF.
- Evidence: Go test TestReports and the full suite pass against a real isolated PostgreSQL; web build passes; deployed to production 2026-10-31 (backup `dapper-before-reports-2026-10-31-1643.dump`). The screen has not been visually checked by me.

## SOP add and remove (second list, item 2)

- SOP & Compliance (owner dashboard and staff workspace) now has "Add document" for anyone with `sop.manage`: title, category, PDF up to 10 MB. The server checks it is a PDF, stores the original bytes under a checksum-based name, and publishes it to employees. Each card has Open PDF and Remove.
- Remove hides the document from employees; the record and file stay so earlier acknowledgments keep their evidence. Adding the same PDF again restores it.
- Routes: `POST .../sops?title&category` (raw PDF body), `DELETE .../sops/{id}`, `GET .../sops/{id}/file`. When `DAPPER_SOP_DIR` is unset, files go in a `sops` folder beside the uploads folder (on the server `/opt/dapper/app/.dapper/sops`).
- Evidence: Go test TestSOPManage and the full suite pass on real PostgreSQL; web build passes; deployed 2026-10-31. Screen not visually checked by me.

## Custom scripts (second list, item 5)

- New Settings > Custom scripts page: any number (up to 50) of snippets, each with a name, placement (page head or end of body), the code pasted as the provider gives it, optional page paths, and an on/off switch. Edit, turn off and delete are available. Works like WPCode in WordPress.
- Only roles with `access.manage` (Super Admin, Owner) can see or change it. Website tracking (GTM, Meta Pixel, Mailchimp) is unchanged and still takes identifiers only.
- Migration 76 `custom_scripts`. API: `GET/POST .../organizations/{org}/custom-scripts`, `PUT/DELETE .../{script}`; the public tracking response now includes the active snippets. The site rebuilds script tags so they run, once per page load, on matching paths.
- Security note, shown on the page: the public site and the dashboard share one address, so a malicious snippet could act as a signed-in person viewing the website. This is reduced once the website has its own domain separate from the dashboard.
- Evidence: Go test TestCustomScripts and the full suite pass; web build passes; deployed 2026-10-31 with backup `dapper-before-scripts-*.dump`. A snippet was not run in a browser by me.

## Quotes layout (second list, item 3)

- New quotation form: client name, phone and email now sit on one row; booking request and valid-until on the next. The tax field is a narrow box beside the totals instead of stretching across the form, and the line-item columns are narrower. Web-only change, deployed 2026-10-31; not visually checked by me.

## Dealership tag gallery (second list, item 14)

- Dealership tags now show only the posted photos, as a grid of small previews. Clicking one opens it full size with its details (dealership, who posted, time, GPS, notes, retention) and a strip to flip between the post's other photos.
- Logging: previews come from a new `.../employee-uploads/photos/{photo}/thumbnail` route (same access, downscaled, not logged). Opening the full photo is logged in the viewing log as before. Customer-car uploads keep the old "open photos" list.
- Evidence: Go test TestShrinkJPEG and the full suite pass; web build passes; deployed 2026-10-31. Not visually checked by me and not tried with real photos.

## Appointments page like BookingPress, and status editable for every booking (third round)

- The Appointments page now follows the WordPress "Manage Appointments" layout: title with green Add New and Share URL (copies the public /booking link); filter row (date range, customer name, service, status); second row (appointment ID, search, Reset, Apply, Export); table with checkbox, ID, Date, Customer, Service, Duration, Status dropdown, Payment and Created date; expandable rows (+/−); footer with "Showing N out of M", Per page and pager. Export downloads the shown (or ticked) rows as CSV.
- Status dropdown is now available on every appointment, including Completed and Cancelled ones. New `CorrectBookingStatus` (same `POST .../bookings/{id}/status` route): normal forward moves are unchanged; completed to cancelled releases the time slot; reopening a cancelled booking detaches its released slot (it stays on the books without holding capacity, move it to hold a new time); every change is written to the booking history as a staff correction. Employees keep the stricter rules.
- Evidence: full Go suite passes on real PostgreSQL (booking capacity test extended with reopen, complete, uncomplete and re-cancel); web build passes. Not visually checked by me.

## Fourth round: UI glitches, reschedule, backdating

- Checkboxes and radio buttons were stretched to the full width of their row by a global style (coupon form, others); they are now fixed 20px everywhere. The +/- buttons inside number fields lost their round pill style; plain tables (activity log, staff hours) no longer get a white header; the availability "Add time / Copy to ..." buttons have spacing.
- "Complete, money still owed" removed: with money owing, completing needs Cash or E-transfer received; a booking with nothing owed still has a plain Complete.
- Reschedule: a booking that holds a calendar slot but has no matching service timing rule could not be moved ("No matching active service timing rules"). It can now be moved by hand: it gives its slot back, keeps the new time and shows in history. `MoveUnreservedBooking` handles reserved bookings (test extended).
- Backdating: administrators with bookings.manage can step back to earlier months and pick days that already passed in New booking (`past=1` on the availability route, honoured only with bookings.manage).
- Not done: true per-service slot capacity (Opening hours still has the shop-wide "Slots for each time"). Needs a scheduling engine change, see the chat.

## Slots per service

- Each service rule now has its own `slots` (migration 77): how many bookings of that service can run at the same time. When one is booked or offered, only bookings of the same service count against that limit; other services no longer use it up. Existing rules kept their old number (calendar slots divided by the units the service used).
- Changed in all four places that count existing bookings: making a booking or moving one, staff and public availability, and the customer portal offers and requests. A reservation without a service rule still uses the shared calendar capacity as before.
- Screens: Services > Slots (central catalogue services and franchise extras) edits "Slots (bookings of this service at the same time)". Opening hours no longer has the shop-wide slots field once a calendar exists.
- Evidence: TestServiceSlots (service A with 1 slot refuses a second booking at the same time, service B with 2 slots is not blocked by A and refuses a third) and the full suite pass on real PostgreSQL; web build passes; deployed 2026-11-01 with backup `dapper-before-slots-2026-11-01-0621.dump`. Checked on the live site that Services > Slots shows the per-service number (2 for the first service).

## First franchise, Super Admin and website images (fifth round)

- Setup wizard has a new "Franchise" step after the owner: name, short code, street, city, province or state, postal code, country, optional phone. Install creates the franchise, a published website entry for it (slug is the lowercase code), the built-in **Super Admin** role (every capability the server knows) and assigns it to the owner for the whole organization. Code: `ProvisionFirstFranchise`, `install/server.go`, `install/wizard/index.html`. The CLI `initialize-admin` still creates only the owner account.
- Dashboard: Settings > Website images (needs content.manage): add several JPEG or PNG photos (up to 6 MB), a permission tick before anything is published, edit titles, change section, show or hide, move up or down, delete. A franchise without a website entry gets a one-click "Set up the website entry".
- Website: the Portfolio page reads `GET /api/v1/public/portfolio` (published photos of published franchise sites, in the dashboard order). When at least one is published it replaces the built-in photos in the hero mosaic, the sideways strip and the viewer; with none it keeps the built-in ones. Migration 78 stores each photo's width and height so frames keep their shape while loading.
- New routes: `DELETE .../content/portfolio/{image}`, `PUT .../content/portfolio-order`.
- Evidence: TestFirstFranchiseSetup, TestPortfolioManage, the install request tests and the full suite pass on real PostgreSQL; web build passes; deployed 2026-11-01 with backup `dapper-before-images-2026-11-01-0745.dump`. On the live site: upload, publish, the Portfolio page switching to the uploaded photo, and delete were all exercised with a test picture, now deleted. The setup wizard's new step was not run in a browser (it needs an empty database); only its server side is tested.

## Email & SMS menu (replaces the Mailchimp menu)

- The dashboard's "Mailchimp" entry is replaced by **Email & SMS** (Money group; only roles with `access.manage`, so Super Admin and Owner). The Mailchimp popup script address still lives in Settings > Website tracking.
- Email (SMTP): server, port, username, password, From address and name. Saved settings go into use at once and replace the server's own `DAPPER_SMTP_*` settings until removed. Send-test-email button. With this, invitations and password-reset emails need no server file edit.
- Text messages (Twilio): Account SID, Auth Token, and a sending number (+E.164) or Messaging Service SID. Check connection (asks Twilio about the account, sends and costs nothing) and send-test-text. Customer texts (reminders, confirmations) are not switched on; this stores and tests the settings.
- The SMTP password and the Twilio Auth Token are sealed with the server key in table `messaging_settings` (migration 79) and never sent back to a browser; a blank box keeps the saved one. Every save and removal is audited. Test sends are limited to one every 8 seconds per person.
- Code: `email.Live` (settings that change while running), `twilio` package (REST client), `postgres/messaging_settings.go`, `workspace/messaging.go`, `owner/MessagingSettings.tsx`.
- Evidence: tests for the Twilio client against a fake Twilio server, the live email sender, and the settings store (permissions, sealing, keep-password, validation) plus the full suite pass; web build passes; deployed 2026-11-01 with backup `dapper-before-messaging-2026-11-01-0808.dump`. The page was checked on the live site; no real SMTP server or Twilio account was available to me, so a real email send and a real Twilio call have not been run.

## Employee role, Employees page, layout slips (sixth round)

- **Employee role** (migration 80, also created at setup): a built-in role "Employee" with the 21 capabilities the employee workspace uses (clock in/out, calendar, walk-ins with invoices and payments, uploads, SOP). Owners can change it under Team > Roles.
- **Employees page** lists only people holding the Employee role. Owners and managers are managed under Team. The invitation form now starts with the Employee role selected.
- **Workspace only:** `GET /api/v1/account/home` says whether someone's only role is Employee (and they hold no organization-wide grant). The dashboard sends such people to `/workspace`. What they may do inside the workspace is whatever the Employee role grants; the redirect is not the only barrier, but the dashboard APIs they could still call are limited by that role.
- **Double background:** panels that bring their own glass stage (Customer cars, Dealership tags, GPS and Square panels) no longer draw a second background behind the card inside the dashboard. Rows of fields and a button (Website images, Email & SMS) share one bottom line.
- Evidence: TestFirstFranchiseSetup extended (Employee role, workspace-only for employee-only, administrators and people with no role); full suite and web build pass; deployed 2026-11-01 with backup `dapper-before-employee-role-2026-11-01-0846.dump`. Checked live: Employees page empty with guidance text, Customer cars and Website images layouts, roles list shows Employee with 21 capabilities. Redirect for an Employee-only account was not exercised end to end (no such account exists yet).

## Seventh round: tester list from Discord

- **Invoice business details** (was "Invoice issuers / Location issuer / Create issuer"): one plain form (legal name, tax number, address, number prefix, first number). Saving switches it on for every franchise, so nothing is linked by hand. Root cause of "This action is not granted": invoice and structure settings checked only direct organization grants, ignoring organization-wide roles such as Super Admin and Owner. Both now accept a direct grant or an organization-wide role (`financeConfigurationGrant`, `structureGrant`). Until the details exist, collecting a payment at job completion refuses with a message pointing here.
- **Notes**: status changes no longer write "Operator status update" into a booking's own notes (they stay in the booking history). Migration 81 removes the leftovers already stored.
- **Reschedule conflict**: the new time is now picked from the times the calendar can offer on the chosen day (past days included for people who manage bookings), instead of typing any time. The conflict message says what happened.
- **Quotations**: tax is a percent of the subtotal (13 to start); Download PDF opens the quotation in the invoice look with business details, valid-until, note and terms; Settings > Invoice template has a Quotations section (title and terms). Quotations show the client's name instead of internal ids.
- **Coupons**: services are chosen in a popup with search (every service when nothing is ticked).
- **Services**: tick several to bulk edit price (set, raise or lower by percent) and time, or archive them together.
- **Backdating**: month and year pickers in New booking for administrators; opening hours are applied to past days, so any past date inside normal hours can be booked. Entering a past time needs bookings.manage.
- **Slots screen** explains what a calendar is when a franchise has more than one.
- Not done, needs a decision: add-ons as one record shared by several main services (today one row per main service, kept in step by name); and the Staff hours "font fix" (no specific fault could be identified).
- Evidence: full Go suite and web build pass on real PostgreSQL; deployed 2026-11-02 with backup `dapper-before-fixes-2026-11-01-1939.dump`. Not yet checked on screen: the browser window used for checks was closed and its replacement stops at the certificate warning for the server address.
