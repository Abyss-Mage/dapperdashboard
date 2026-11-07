# Employee Workspace replacement — 2026-10-24

Status: CONFIRMED direct user product/design direction; implementation pending. Supersedes the broad ten-area Workspace UI and earlier preserve-layout restriction. Does not remove required business functionality from the overall project; administrative functionality belongs in Master.

## Confirmed scope

1. Employee authentication using existing verified identity/session authority.
2. Clock in/out, work/time tracking and ongoing location tracking while clocked in.
3. Submit next week's available days/time intervals to supervisor/admin. Availability submission is not an assigned or approved shift.
4. Add/book walk-in customers: name, email and phone required; BOTH email and phone verified before booking confirmation; choose permitted services and compatible add-ons; calendar date/time selection with authoritative capacity checks.
5. View prebooked customers in the assigned dealership/worksite.
6. Cash or Square Reader payment with immediate invoice generation. Reader support required on BOTH Android phone/tablet and iPhone/iPad.

Admin assigns employees to the dealership/worksite beforehand. Employees have NO franchise/location/dealership selector. The header shows assigned worksite read-only. Missing/ambiguous assignment blocks work with a useful contact-admin state; no auto-selection or guessed mapping of dealership account to franchise/location. Server resolves assignment and rejects foreign-scope selectors. Admin owns any reassignment and its effective time/history.

## Interface direction

Mode: Operate. Use Impeccable structure, Emil interaction precision and Apple clarity/touch ergonomics. Replace the previous long form stack; do not retain seventeen administrative panels merely behind a visual wrapper. Keep Dapper brand/red accents, readable type, solid accessible surfaces, high contrast, large touch targets and calm hierarchy. Subtle depth/materials only where useful; no animation blocking keyboard/repeated actions. Reduced-motion/transparency alternatives required.

Proposed employee navigation: Today | Bookings | Walk-in | My shifts. Payment is an action on the relevant booking/customer, with receipt/invoice history there. Desktop may use compact sidebar; phones use a thumb-accessible bottom bar. Profile/sign-out stays secondary.

- Today: assigned dealership label, clear clock status, clock-in/out action, ongoing tracking indicator, upcoming customers and pending-payment work relevant to the employee's authority.
- Bookings: day/week calendar and concise customer list, service/add-ons, verified contact state and booking/payment status; scoped customer detail.
- Walk-in: Customer -> verify email and phone -> services/add-ons -> available date/time -> review/book. Show authoritative amount and tax, preserve input and explain expired codes/conflicts.
- My shifts: personal attendance/history and next-week availability grid with multiple intervals, worksite timezone, submission status and revision history. Supervisor receives/reviews in Master; do not imply a scheduled shift from availability.
- Payment: authoritative amount, Cash/Reader, device connection/progress/failure/retry state, invoice preview/download and delivery status. Never present cancellation/authorization/timeout as payment success.

Remove from employee surface: organization/franchise/location CRUD, role/capability editing/assignment, issuer/tax setup, catalogue editing, resource-policy configuration, marketing/CMS and central reports. Preserve underlying APIs/history and Master workflows. No broad deletion of shared components because Master imports them.

## Implementation and acceptance constraints

- Verification challenges must bind to customer/contact/purpose, expire, limit attempts/resends and avoid account enumeration. Server evidence, not frontend toggles. Changes to a verified contact invalidate relevant verification. Provider credentials/templates still need configuration; no unsolicited real messages.
- On-shift GPS is approved as a feature, not permission to begin collection now. Specify notice/permission, authorized viewer, sampling/accuracy, retention, break behavior and offline/device loss. No off-shift collection. Denied permission, stale sample, lost permission and stopped tracking must be visible. Verify Android/iOS foreground/background/locked-device behavior before choosing web versus native/hybrid packaging; never promise reliable background tracking from a mock browser timer.
- Square mobile Reader integration must be verified against official current SDK/device/country requirements before platform choice. Reuse server merchant-bound payment authority; do not equate browser card checkout with physical Reader support. Hardware/provider/sandbox/live evidence stay separate. Cash is an audited authorized receipt, not provider card settlement.
- Invoice generation must use approved issuer, numbering and exact-money/tax rules. Invoice issuance and paid status are separate: generate as required by approved workflow and mark paid only after confirmed cash recording/provider settlement. Immediate invoice generation cannot bypass missing legal configuration or failure handling.
- Dealership assignment and merchant routing are related but not assumed identical. Define explicit ownership/mapping, preserve isolation and prevent employee reassignment from exposing prior worksite data.
- Test actual persisted booking/capacity, employee assignment/revocation, availability submission, verification expiry/replay, GPS lifecycle and payment timeout/duplicate recovery on both platform families. Use isolated synthetic fixtures, no customer operational-data pollution.

## Remaining decisions and next steps

GPS sampling/retention/viewer/notice and reliable device background behavior; dealership-to-location mapping and multiple-assignment resolution; next-week cutoff/timezone and availability edit rules; email/SMS credentials; reader integration/merchant/device evidence; approved issuer/tax rules. These remain open without inventing defaults. They do not prevent independent design/API-contract work after quota replenishes.

Next implementation owner must read/register exact files in PARALLEL_AGENT_COORDINATION before edits, coordinate with Antigravity Workspace, Codex Master/payment and website lane, and reserve shared build/migration windows. No WSL. Impeccable context engine failed earlier this session; fallback context/reference review applies, not an automated design scan claim. No application code changed for this brief. Last quota99%used/1%remaining; existing stop policy remains effective.

## User change — 2026-10-24: no OTP
CONFIRMED direct user: require valid email/mobile number format only. No OTP verification is required. Supersedes earlier both-email-and-phone verification requirement. Format validation does not establish contact ownership; never label these contacts as verified. Keep native GPS and Reader/provider gates unchanged. Walk-in API must atomically persist customer, capacity reservation, booking and assigned dealership link, with actor/payload-bound retries and no automatic payment/confirmed-policy claims.
