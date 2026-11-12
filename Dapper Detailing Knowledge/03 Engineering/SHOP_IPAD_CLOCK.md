# Shop iPad: clock and shared dashboard (error404 #15, #20)

Status: **built and deployed 2026-11-11** (migrations 112 `shop_clock.sql`, 115 `shop_login.sql`).

## Decisions (owner, 2026-11-11)
- #15: staff clock in, take breaks and clock out on one iPad in the shop; personal clock-in on phones and location tracking are removed; the owner sets each person's PIN.
- #20 (replaces #15's open no-login page): **one iPad per franchise**, signed in with a **separate shop username and password per franchise** that the owner sets (it opens only the worker list, never a dashboard). The iPad lists the franchise's workers; a worker taps their name and types a **6-digit PIN**. There is a single iPad, so **everything is shown**: the PIN opens that worker's own employee dashboard on the iPad, with Clock in / Start break / End break / Clock out on a bar at the top.

## How it works
- Owner: Staff hours → Shop iPad: set the shop login (username 3–40 lowercase characters, password 10+), copy the `/shop` link, set or remove 6-digit PINs. Changing or removing the shop login signs every iPad out.
- iPad: `/shop` → sign in with the shop login (cookie `__Host-dapper_shop`, 90 days) → tap name → PIN → the worker's normal dashboard session (30 minutes, marked with the shop's franchise) and the shop bar. **Done**, 2 minutes without a tap, or reopening `/shop` ends the worker session. The old `/shop-clock/{org}/{loc}` link redirects to `/shop`; the old public no-login API is removed.
- Limits: 10 wrong shop passwords per visitor, 5 wrong PINs per worker and 30 per iPad, each per 15 minutes (in-process limits).
- **Owners and administrators (access.manage) and anyone with two-factor turned on cannot open their dashboard with a PIN**; they use their own login. This is a safety rule added during the build, not an owner request.
- PINs set before #20 were 4 digits and were cleared by migration 115; the owner sets new 6-digit PINs.

## Evidence
- HTTP test `TestScopedHTTPBoundaryWithPostgres`: PIN must be 6 digits; shop login saved (password never returned); worker list refused without the shop sign-in; wrong shop password refused; PIN unlock gives a working dashboard session; status, clock in, break, end break, clock out; a normal session cannot use the shop bar; Done ends the worker session; an owner's PIN is refused; 5 wrong PINs lock; changing the shop password signs the iPad out.
- The `/shop` name list, PIN pad and the shop bar over `/workspace` were checked in the local preview with synthetic data.
- **Not verified:** on a real iPad (Safari cookies, Home Screen app), and with real staff on production.

## Third round (owner, 2026-11-12)
- The shop clock is the workspace's own **Clock in / out** page, the first page for everyone who signs in (own login: the franchise they are assigned to; on their own login there is no "Open my dashboard").
- The shop username signs in on the normal workspace form. The shop login opens the **whole workspace** as a hidden per-franchise **"Shop iPad"** account (`system:shop:<location>`, the Employee role's permissions at that franchise minus clocking itself; never in staff or PIN lists). Its workspace session (24 h max) renews while the shop login (90 days) lasts. Removing the shop login switches the account off.
- **Availability** on the Shop account: name + PIN opens that worker's own availability week.
- Migration 117: a team member's phone is optional (error404 #41).
- Evidence: HTTP test (shop sign-in opens the Shop workspace with booking access; Shop account absent from the PIN list; wrong PIN refused on availability). Not verified on a real iPad.
- 2026-11-13 (error404 #48): the PIN list is the franchise's team by role, like the Employees page: a role that records time assigned to this location or its franchise. Organization-wide roles and old direct grants no longer list someone.
