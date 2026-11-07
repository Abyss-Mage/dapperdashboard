# Domain glossary — read before touching franchise, dealership or worksite code
Updated 2026-10-24 from direct user clarification (supersedes the earlier version of this note). These terms are **different things**; do not merge them.

| Term | Meaning | Where it lives |
| --- | --- | --- |
| **Organization** | The company (Dapper Detailing Pros). Top-level tenant. | `organizations` |
| **Franchise** | **One site/business unit of the company, identified by a unique code** (for example `PTBO` = Main Office, 660 The Kingsway, Peterborough). Has a name and an optional structured address (street, city, province/state, postal code, country). Staff, bookings, scheduling, invoices, grants and GPS settings belong to a franchise. There is **no separate "location" concept** in the product. | physical row in `locations` (+ `code`, `address_*` columns); 1:1 shadow row in `franchises` |
| **Dealership** | A **partner car dealer the company works with**, added by an administrator in **Master > B2B Fleet Accounts** (account type `dealership`). Bookings can be linked to a dealership; employees post key-tag photos for a dealership. It is **not** a franchise or a site. | `dealership_accounts` where `account_type='dealership'` |
| Fleet / commercial account | Other B2B account types in the same table. Never offered as dealerships. The old internal "Main Office" account (type `commercial`) is a leftover of the previous worksite hack and is no longer used. | `dealership_accounts` |
| **Employee assignment** | The administrator assigns an employee to a **franchise** (required) with a timezone. A partner dealership is an optional "focus" (nullable). | `employee_worksite_assignments` (`dealership_id` nullable since migration 52) |

## How the franchise-as-site model is implemented (migration 53 + `franchises.go`)
- The word "location" is retired from the UI and the structure API. `POST/PUT/DELETE /api/v1/organizations/{org}/franchises` create/edit/delete a franchise; the `locations` create/edit/delete routes no longer exist. `GET /structure` returns `franchises[]` (each with `id` = **site id**, `franchiseId` = internal grouping id, `code`, `name`, `address`) plus a read-only `locations[]` compatibility list used by AccessManager.
- Internally the site row stays in `locations` because every module is keyed on `(organization_id, location_id)`. Renaming that physically would touch every table; do it only as part of a deliberate rebuild.
- The `franchises` table is kept as a **1:1 shadow** per site so franchise-scoped role assignments (`member_role_assignments.franchise_id`) keep working. The API creates both rows together and `UpdateFranchise` adopts older sites that had no grouping row. The database does NOT enforce one-site-per-franchise (older role tests rely on multi-site franchises); the API does for new franchises.
- Codes: uppercase, 2–12 chars `[A-Z0-9-]`, unique per organization (partial unique index; nullable only so pre-existing rows could be back-filled). The application requires a code for every new franchise.
- Deleting a franchise is blocked by customers, jobs, bookings, invoices, attendance, photo posts, employee assignments/availability/booking links or franchise-scoped role assignments. It removes the site's own configuration (GPS settings, location grants, grant history) and keeps a `franchise.deleted` entry in `organization_audit_events`.

## Booking visibility for employees
Employees see bookings that an administrator linked (`employee_booking_worksites`). A link with no dealership means "everyone assigned to that franchise"; a link with a dealership is visible only to employees whose assignment names the same dealership. Walk-ins inherit the creating employee's dealership focus (or none).

## Rules that follow
1. Dealership pickers list only active accounts of type `dealership`; the server rejects anything else (tags). Customer-car posts link to a **booking**; their dealership comes from the booking link and may be empty.
2. UI copy says "franchise", never "location", for sites. Code/API ids still use `locationId` (= the franchise's site id).
3. Adding a dealership is a Master action; the Workspace tells employees to ask an administrator when none exist.

## Remaining debt
- `AccessManager` still offers both "Location" and "Franchise" scopes for role assignment; for new franchises they are equivalent. Collapse in the Master rebuild.
- Many internal names (`location_id`, `Scope.LocationID`, routes `/locations/{location}`) keep the old word. Behaviour is unaffected.
- A partner dealership belongs to one franchise (`dealership_accounts.location_id`); serving several franchises needs one account per franchise until reworked.
