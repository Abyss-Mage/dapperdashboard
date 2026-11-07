# Master dashboard simplification and owner requests (2026-10-25)

Owner direction (2026-10-25): stop the 3D home page for now (owner will supply car videos and garage images); focus on the dashboard. Items and what was done:

## Booking wizard (staff "New booking", walk-in and website booking share it)
- Categories: Detailing, Ceramic coating, Paint protection film, Window tinting. Shield, Fortress, 3 Stage Paint Correction and Armour Up are Ceramic coating packages.
- Vehicle order as on the website: 2 Wheeler, Hatchback, Sedan, SUV, Pickup, Van, Aircraft S/M/L, Boat S/M/L. Packages without a vehicle in the name (ceramic packages, tint) apply to the car types.
- Tax shows its percentage ("Tax (13%)"); the "Deposit due now (30%)" lines were removed from the details summary (the website's deposit step itself is unchanged).
- Mobile and email share one row, half each.
- Window tint added to the central catalogue and Main Office using the owner's legacy BookingPress prices and times: Single window $59 / 30 min, Standard 4-window $219 / 60 min, Full 5-window premium $289 / 90 min (bays resource, 1 unit).

## Dashboard structure
- Every section is its own page with its own address (`/owner#customers`, `#staff-hours`, …), grouped Every day / Team / Money / Settings; pages a person cannot use are hidden.
- Removed from Master: Invoice issuers page, Customer booking offers ("Packages"), the scheduling resources/service-combination editor, the setup checklist, B2B fleet queue, popup form builder and consent ledger. "Phone calendar" became the **Link Calendar** button on Bookings.
- "Work orders completed" → "Bookings completed" (overview and numbers pages). "Documents" → "SOP & Compliance" everywhere.
- Dark mode: the coloured background glows are gone.

## New or changed pages
- **Bookings**: month / week / day calendar (same as the staff workspace), franchise time zone.
- **Customers**: short list; clicking a name opens their vehicles, bookings, jobs and invoices. Every customer can be deleted: everything removable is deleted; if issued invoices, payments or signed records exist those stay (tax records) and the customer is removed from all lists (`customers.removed_at`). One customer per email and per phone number at a franchise (database unique indexes); walk-ins and website bookings reuse the existing customer.
- **Staff hours**: live map of clocked-in staff (OpenStreetMap tiles, attribution shown) and time worked per person by day / week / month with the site's date filters. No clock-in buttons here.
- **Staff availability**: approve or reject each week's submission; a changed submission goes back to waiting.
- **Team → Invitations**: choose the role and franchise before sending; assigned on acceptance by the inviter.
- **Services**: central catalogue on top; changes (name, price, archive) apply to every franchise, and new central services switch on everywhere automatically. Below it, only the franchise's own extra services.
- **Opening hours**: one set of hours per franchise, saved to all its bays behind the scenes.
- **Invoice template**: optional owner-written HTML + CSS with `{{placeholders}}` (escaped values), shown and printed in a sandboxed frame with no scripts; scripts/frames/plug-ins are refused by the server.
- **Card payments**: each franchise connects its own Square account (Square OAuth code flow, scopes PAYMENTS_WRITE, PAYMENTS_READ, MERCHANT_PROFILE_READ), then picks the Square location; Square confirms it (active, CAD, card processing). Tokens are stored encrypted with the server key, renewed every ≤ 7 days (Square guidance), checked every 6 hours. Needs `scripts/set-square-app-secret.ps1` and the redirect URL `https://127.0.0.1:8443/api/v1/integrations/square/oauth/callback` registered in the Square Developer Dashboard. **Not yet tested against Square** (no app secret on this machine).
- **Dealerships**: add / edit / switch off dealerships (each is also a customer). **Invoice Ninja**: owner saves the https address and API token (checked, stored encrypted); issued dealership invoices get a "Send to Invoice Ninja" button (creates or reuses the client, copies lines, sent once). **Not yet tested against a real Invoice Ninja account.**
- **Mailchimp** is the only marketing page; no prefilled server prefix.

## Data
- Test/demo records deleted on 2026-10-25 (customers incl. ZZ tests, bookings, holds, jobs, 2 paid test invoices and payments, attendance, test dealership, requests). Kept: organization, franchise, catalogue and prices, taxes, hours, rules, roles, Super Admin and the website system account, SOP documents, invoice template and issuer. Encrypted-at-rest backup taken first (`.cache/backups`, local only).
- Migration 64 (`customer_unique_availability_review.sql`) applied after the migration script's encrypted backup and restore check.

## Evidence
- Source + static: Go build/vet, TypeScript check, production web build.
- Runtime tests: full Go/PostgreSQL suite passed (customer delete, duplicates, records, invitations, central auto-link).
- Live checks in the running app: pages load; duplicate email/phone refused; customer delete works; wizard categories and vehicle order; dark mode plain.
- Not live-verified: Square OAuth connection, Invoice Ninja sending, staff map with real GPS (no staff on shift).

## Dropdowns (2026-10-25, later)
- Every native `<select>` in the web app (about 85 in 32 files) is now `app/GlassSelect.tsx`, a drop-in with the same `<option>` / `<optgroup>` children and `onChange(e.target.value)`. GSAP (core + `useGSAP`, `quickTo`, `utils.mapRange/clamp`): menu scales out from the button (bottom sheet under 640 px), options stagger in, one highlight pill glides between options, options near the pointer swell like the macOS dock, the chosen label slides in, the chevron turns. Keyboard: arrows, Home/End, Enter/Space, Esc, Tab, type-ahead; required fields still validate; motion off for prefers-reduced-motion.
- Checked live on desktop width (open, arrow, Enter, Esc). Not yet checked: phone bottom sheet, grouped menus, very long lists, real touch.
- `scripts/test-workspace-components.mjs` tests F05, F08, F11 fail on the committed code too (they check older wording); not caused by this change.

## Motion system (2026-10-25, later)
- `app/motion/MotionSystem.tsx` (mounted once in the root layout): GSAP entrance for cards, tiles, rows, tables, choices and panels added to any page (outermost element only, short stagger, transform + opacity only, props cleared afterwards); items below the fold wait and rise as they near the screen (IntersectionObserver, at most 250 waiting, print-safe). Content that was already on screen at load is not touched; nothing runs for prefers-reduced-motion.
- `app/motion/AnimatedNumber.tsx`: KPI tiles (overview and section numbers) and the booking wizard totals count up and re-count when the value changes.
- Master: a glowing bar glides beside the active sidebar page; each page slides in from the direction you move and the heading text replaces itself; customer drawer slides in.
- Booking wizard: each step slides in from the side you are heading towards; progress segments glow.
- Charts: lines draw, areas fade in, columns grow, bar lists fill, last-point dot pops (CSS keyframes, with pathLength so no measuring).
- Global CSS (`globals.css`): button lift / press / sheen, card lift, input focus glow, dialog and alert and status entrances, light/dark colour crossfade (class `dd-theme-fade` set by the theme toggle).
- Checked in the running app: sidebar bar, page slide, card fades mid-flight, wizard step slide, no element left invisible, no horizontal overflow. Frame rate in the capture tool is low, so timing feel was not judged by eye; phone sizes and the staff workspace motion beyond the entrance fade are not yet reviewed.

## Native controls replaced (2026-10-25, later)
- `app/Pickers.tsx`: `DatePicker`, `TimePicker`, `DateTimePicker`, `ColorPicker` replace every browser date, time, datetime-local and colour input (20 places, 15 files). Same props as the inputs (value, onChange(e.target.value), min, max, required, name, aria-label). Date: month grid with a gliding selected-day pill, month/year switcher, Today / Clear, full keyboard (arrows, Page Up/Down, Home/End). Time: hour / minute (5-minute steps) / AM-PM wheels with gliding pills, "End of day" where a franchise closes at midnight. Colour: swatches plus hex. Phones get a bottom sheet.
- `workspace/ConfirmDialog.tsx` redrawn as a glass, theme-aware, animated dialog; `app/confirm.tsx` (`askConfirm`) replaces the four browser "Are you sure?" boxes (void invoice, issue quotation, delete popup form, discard invoice edits).
- `app/motion/Tooltips.tsx`: glass tooltips replace the browser's title tooltips (shown after a short pause, instantly on keyboard focus; no touch).
- Global CSS: custom checkboxes and radios, a real toggle for switch rows, no number spinners, glass file button, thin scrollbars, turning chevron on expanders, quiet textarea grip, brand text selection, autofill colours.
- Services: the central catalogue and each service's add-ons can be folded (remembered per device); very tall lists fade instead of resizing.
- Not done: the public marketing pages under `main website` (separate static site, its own forms).

## Number steppers and the phone check (2026-10-25, later)
- `app/NumberField.tsx` replaces every `<input type="number">` (20 places in 12 files): a real number input with − / + buttons, hold to repeat (faster the longer you hold), min / max / step respected, arrow keys and typing still work, a small spring on each change.
- Checked on a 390 x 844 touch viewport and a 1100 px desktop viewport, light and dark, using a temporary page that showed every control (deleted afterwards): dropdown, date, time, date-and-time, colour, steppers, switch, checkbox, radio, file button, text area, expander, confirm dialog, tooltip. Phones get bottom sheets flush to the bottom edge; confirm buttons stack. Found and fixed: a global dialog animation fighting the sheets (they stuck at 96 % size), checkboxes stretched to 44 px tall by an old input rule, the older dark-mode input rule painting over checked boxes, toggle tracks and the stepper / hex boxes.
- Not checked: real-device touch (only emulated), and screen readers.
