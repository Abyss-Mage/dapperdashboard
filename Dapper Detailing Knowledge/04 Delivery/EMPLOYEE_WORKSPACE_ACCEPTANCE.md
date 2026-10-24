# Employee Workspace implementation and acceptance — 2026-10-24

Direct user scope: dedicated employee interface; admin-assigned dealership; no scope picker; six workflows. Latest changes supersede OTP: email/mobile format validation only. Full booking calendar and GSAP React motion are requested. Codex owns this lane; no delegation or WSL.

## Implemented increment

- Employee authentication reuses existing managed sessions, login/logout, same-origin mutations and active-principal checks.
- Migration46 adds explicit employee worksite, next-week availability, booking-dealership links and immutable audit events. No operational seeds or grants. Assignment requires org access.manage, existing active membership, exact dealership/location and explicit timezone. Optimistic versions and active-shift reassignment checks apply.
- Employee attendance uses existing audited clock/break methods. Personal attendance only. Worksite authority is resolved by server, never caller selectors.
- Availability persists next-Monday intervals in explicit worksite timezone, revisions and supervisor inbox in Master. It is not a confirmed shift or payroll approval.
- Walk-in validates required contact syntax server-side and client-side, normalizes international phone formatting, and uses no OTP. Customer, capacity reservation, pending booking, dealership link, audits and actor/payload-bound retry receipt commit together. Exact configured service/add-on rules only; conflicts/stale rules/audit failures do not leave partial operational records. Repeated/nonexistent DST civil times are rejected.
- Calendar loads selected month/week/day date ranges from assigned-dealership API (42-day maximum, 500+truncation notice), with date/period navigation and booking details. No fake appointments. Legacy organization management endpoints reject assigned non-admin employees to prevent bypassing dealership isolation.
- Cash recording uses explicit receipt confirmation, existing issued invoices, exact cents and idempotency. Issuer/customer/item snapshots remain visible after receipt; invoice totals/tax/paid/balance and scoped print styles included. No card payment fallback or fake settlement.
- Workspace becomes Today / Bookings / Walk-in / My shifts. Master retains shared admin components and adds employee assignment plus availability review. GSAP useGSAP/matchMedia/revert cleanup adds brief heading/feedback motion; reduced-motion disables it.

## Test evidence

Full native Go/PostgreSQL suite: all nine tested packages PASS, including atomic employee walk-in/calendar persistence. Final focused TestEmployee rerun after management-endpoint restriction: PASS (postgres3.328s, workspace3.169s). Final Go vet PASS after the guard. Final webpack/TypeScript build with GSAP PASS for all11routes. Pinned HTTPS ingress7checks and Workspace/Master assets PASS; build FfztZLxs066PkE_-c71ql. Six helper tests PASS: timezone midnight, DST-independent date arithmetic, full calendar ranges, contact formats and availability validation.

Isolated browser fixture uses postgres database with disposable schema and separate review cookie, ports4176/8445. Never uses dapper_application or changes the real user session. Browser acceptance:

`PASS synthetic isolated employee closedShift1 availability1 cashReceipt1 invoiceDue0 walkinCapacity1 dealershipLink1; no OTP/provider/GPS operation`

Earlier runs additionally exercised start/end break, availability interval save and reload. Calendar month/week/day navigation and appointment-to-invoice flow observed; a newly saved format-only walk-in appeared at the selected dealership date/time. Desktop/mobile screenshots in output/acceptance. Mobile page width375 at viewport390 showed no document overflow; month grid has internal horizontal scrolling. Fixtures cleaned up with guarded scripts. Browser connection interruptions and native date-input automation required reconnection/setValue; no fabricated test result.

## Design finish review (inline role substitution; no delegation)

Code-led build: no approved comp or quality-bar image was supplied, and context engine unavailable. No automated design-detector result is claimed. Screenshots named above are the evidence packet. DESIGN.md describes shipped source, not a proposed alternative.

Disposition: fix (complete six-workflow scope).

Persistence: PRODUCT.md and scoped DESIGN.md present; direction contract retained. No shipping raster provenance missing because no decorative rasters ship.

Fidelity: assigned masthead, four destinations, personal shift desk, chronological customers and mobile navigation match the narrowed employee brief. Full calendar and format-only intake adapt later explicit user requests. GPS/Reader are visibly unavailable; automatic invoice issuance missing. No comp-fidelity certification is possible without a comp.

Ceiling: restraint and direct task access suit the operational surface. Real hardware/background/provider behavior is unverified. Week/day are agenda views, not time-duration grids; no drag scheduling.

Material fixes: finish native tracking lifecycle/privacy configuration and device acceptance; integrate/test Android/iOS physical Reader with approved merchant mapping; finish automatic invoice issuance using approved issuer/tax/pricing rules. Verify print/PDF, dark theme and broader accessibility before production acceptance.

Keep: no employee scope picker, no automatic grants, honest provider states, exact-money receipts and dealership boundaries.

| Before | After | Why |
| --- | --- | --- |
| Broad operational/admin console | Four employee destinations | Staff only need their assigned worksite tasks |
| Local agenda cache for a date | Scoped selected-range month/week/day calendar | Other months need authoritative records |
| OTP blocker | Validated email/mobile with no ownership claim | Latest explicit user instruction |
| Invoice detail disappeared after cash | Existing snapshot merged with receipt totals | Receipt must retain issuer/customer/items |
| Legacy management APIs could bypass dealer filtering | Assigned non-admin management-route guard | UI filtering alone is not isolation |

## Still required

Native on-shift GPS collection/background behavior and approved notice/viewer/sampling/retention/break/offline rules; no location collection is active. Physical Square Reader native integration/merchant hardware credentials/device acceptance; no live/provider charge performed. Instant invoice creation needs approved issuer/tax/price configuration and employee workflow authority. Existing bookings require explicit dealership attribution; no historical mapping inferred. Worksite revocation UI, scheduling approvals, calendar drag/reschedule, printer/PDF and physical mobile acceptance remain separate unfinished work. No production-ready percentage claimed.
