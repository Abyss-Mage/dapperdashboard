# Database schema
Updated 2026-10-22. This describes implemented migrations, not production acceptance.

The synthetic HTTP demo uses ignored `apps/api/demo-store.json` with customers, owned vehicles/equipment and illustrative bookings. It has no production identity, tenant boundary or financial ledger. Do not populate it with real records.

The separate PostgreSQL foundation is implemented under `apps/api/internal/postgres`. Explicit migration execution uses `cmd/database -migrate` with a private DAPPER_DATABASE_URL. Opening a pool or starting the demo never auto-migrates. Each version is transactional, serialized by advisory lock and protected by a SHA256 checksum.

| Version | Source | Actual tables / behavior |
| --- | --- | --- |
| 1 | schema.sql | organizations, locations, customers, vehicles, audit_events |
| 2 | authorization.sql | principals and explicit location_grants for customer actions |
| 3 | vehicle_grants.sql | explicit vehicle read/create action grants |
| 4 | sessions.sql | random-token hashes, principal links, expiry and revocation |
| 5 | session_invalidation.sql | account deactivation revokes sessions; re-enabling does not revive them |
| 6 | oidc_identities.sql | optional exact issuer/subject identity mapping |
| 7 | password_credentials.sql | managed Argon2id credentials; replacement/deletion revokes sessions |
| 8 | jobs.sql | scoped available/completed work orders and explicit job grants |
| 9 | services.sql | scoped main/add-on catalogue, optimistic versions and revision snapshots |
| 10 | franchises.sql | organization grants, franchises, scoped location parents and organization audit |
| 11 | job_service_link.sql | optional scoped catalogue main service link on jobs |
| 12 | job_addons.sql | server-resolved add-on snapshot array on jobs |
| 13 | reservations.sql | capacity resources, reservations, scheduling grants |
| 14 | reservation_release.sql | reservation cancellation and active index |
| 15 | scheduling_rules.sql | resource service combination duration rules and revisions |
| 16 | reservation_rules.sql | rule snapshots attached to reservations |
| 17 | reservation_history.sql | capacity reservation immutable history revisions |
| 18 | scheduling_read.sql | paginated scheduling.read capability |
| 19 | job_contributors.sql | scoped job contributors tracking and jobs.contribute capability |
| 20 | central_catalogue.sql | central_catalog_services, revisions, catalogue organization grants, jobs.reservation_id |
| 21 | attendance.sql | employee_attendance, employee_attendance_events, attendance.record/read/manage grants |
| 22 | bookings.sql | bookings, booking_events, jobs.booking_id, bookings.read/create/manage grants |
| 23 | invoices.sql | invoices, invoice_items, payments, invoice_events, integer cents math, invoices/payments grants |
| 24 | sop_documents.sql | sop_documents, sop_acknowledgments, SHA-256 integrity, sop.read/acknowledge/manage grants |

Locations reference an organization. Customer ownership includes organization and location. Vehicles reference the complete organization/location/customer key, preventing a vehicle from referencing another scope's customer. These technical organizations are not approved legal issuers or franchise accounting entities.

Customer/vehicle creation and actor/scope audit insertion share a transaction. Session issuance locks the active principal against concurrent deactivation. Session tokens are returned once; only SHA256 digests are stored. Location/action grants are checked per operation and are not inferred from role names or frontend navigation.

Implemented actions: customers.read, customers.create, vehicles.read, vehicles.create. Production identity verification, administrator provisioning, configurable role hierarchy, RLS, least-privilege database roles and finance/booking/employee tables remain incomplete. Audit events currently cover these customer/vehicle writes, not every SOW event.

Tests create uniquely named schemas and remove only their own synthetic fixtures. The HTTP integration harness explicitly passes its schema in the connection URL; changing pgx RuntimeParams does not rewrite Config.ConnString. A discovered harness error created empty local public tables; provenance/zero-row checks preceded removal, and corrected tests pass in isolated schemas. No legacy/production database was involved.


Job creation/completion and service/structure mutations lock their active principal plus exact grant until transaction completion. Completed jobs have one performer/timestamp and cannot be completed twice; audit failure rolls back the transition. Available jobs have no fixed employee assignment. This work-order model does not approve booking, payment, invoicing or supervisor QC transitions.

Services retain scoped parent links, archive state, current version and an immutable-by-application revision per edit. Parent kind/archive and dependent-child invariants are checked under locks. Stale version edits return a conflict. No source price/duration is treated as approved. Organization structure grants do not flow from location grants; existing locations keep a null franchise mapping until classified. These are tested technical organization boundaries, not approved legal-issuer mappings. Application API never updates/deletes revision records; production database privileges/retention policies remain outstanding.

Version11 job_service_link.sql stores optional scoped catalogue main identity. Version12 job_addons.sql stores server-resolved add-on ID/name snapshots in jobs.addons JSONB, default empty array for previous records; transaction validates active exact-scope main/child links and audits creation. Names survive later catalogue changes. No price/quantity/tax/duration inference. Both are locally deployed and verified with synthetic PostgreSQL/HTTPS/browser evidence.

Source migration13 reservations.sql adds scoped dated scheduling resources and technical capacity reservations, explicit scheduling.manage/reserve grants, optimistic move versions and occupied/request snapshots. Native isolated-schema concurrency/audit/ownership tests pass. Running synthetic workspace remains12 migrations; explicit backup/migration required before newer scoped executable startup.

Version14 reservation_release.sql adds cancellation actor/time and active-reservation index, with separate release capability. Versions13/14 now deployed to dedicated synthetic runtime after explicit backup/migration. Reservation create/move/release methods are native database-test verified; no scheduling HTTP route or approved service rules yet. Restore verifier inventories all current ordinary schema tables.

Source15 scheduling_rules.sql stores exact resource/main/sorted-add-on combinations with explicit timing/units, optimistic version and historical revisions. Native DB tests/vet pass. Not deployed: running workspace14. Rule-based reservation/booking binding and configuration API/editor remain incomplete.

Migration15 now explicitly deployed and restore-verified. Existing runtime accounts retain prior grants; no scheduling values seeded. Scope-wide booking acceptance still incomplete.

Deployed16 reservation_rules.sql retains applied scheduling rule/revision/snapshot with exact scope/resource and revision FKs. Existing records stay generic/null; configured reservation creation resolves server rule timing and preserves historical snapshot. No automatic real configuration/grants.

Deployed17 reservation_history.sql adds scoped capacity_reservation_revisions with versioned snapshots, actor/time and reservation FK. Future create/move/release all write history/audit atomically. No pre17 historical backfill fabricated. Native/HTTP and encrypted restore tests pass.

Deployed19 job_contributors.sql adds scoped job identity and job_contributors(job/principal unique,first timestamp) with explicit jobs.contribute. No historical contribution timestamps fabricated. Contribution/completion/roster writes audited transactionally.

Deployed 20 central_catalogue.sql adds organization-scoped `central_catalog_services` and `central_catalog_service_revisions` (optimistic version, immutable snapshots, main/add-on parent invariants), expands `organization_grants` with `'catalogue.read'` and `'catalogue.manage'`, and adds `jobs.reservation_id` foreign key referencing `capacity_reservations(organization_id, location_id, id)`.

Deployed 21 attendance.sql adds `employee_attendance` (clock-in, clock-out, break tracking, total work/break minutes, unique active shift index per employee per location) and `employee_attendance_events` (immutable punch events ledger with actor tracking), and expands grants with `'attendance.record'`, `'attendance.read'`, and `'attendance.manage'`. Verified row-for-row in encrypted backup restore drill.

Deployed 22 bookings.sql adds `bookings` (channel: online/internal/phone/walk_in/dealership, status: pending/awaiting_payment/confirmed/scheduled/in_progress/completed/cancelled), `booking_events` audit table, and `jobs.booking_id` foreign key. Expands grants with `'bookings.read'`, `'bookings.create'`, and `'bookings.manage'`.

Deployed 23 invoices.sql adds `invoices` (with integer cents math, currency 'CAD', subtotal/tax/total/paid/due, invariant `due_cents = total_cents - paid_cents`), `invoice_items` (itemized lines), `payments` (method: cash/square_card/square_terminal/bank_transfer/cheque, idempotency key, provider ref), and `invoice_events` status audit ledger. Expands grants with `'invoices.read'`, `'invoices.create'`, `'invoices.manage'`, `'payments.record'`, and `'payments.read'`.

Deployed 24 sop_documents.sql adds `sop_documents` (title, category: safety/chemical/procedure/onboarding, file name, SHA-256 fingerprint, version, page count), `sop_acknowledgments` (training track: orientation/assisted/independent/certified, hash matching, actor/employee non-repudiation log). Expands grants with `'sop.read'`, `'sop.acknowledge'`, and `'sop.manage'`. All 35 tables verified in encrypted backup/restore drill.

Version25 booking_vehicle_ownership.sql: vehicles_scoped_owner_key and bookings_vehicle_owner_fkey enforce exact organization/location/customer/equipment association. Optional vehicleId remains nullable. Existing migration checksums preserved; schema25 locally deployed/tested/restored with synthetic records only.

Version26 customer_accounts.sql: explicit customer_account_links with principal FK and scoped customer ownership FK; no backfill, grants or automatic linking. Tested and deployed only to synthetic runtime; verified enrollment remains incomplete.

Versions28–29: organization_memberships and explicit access.manage; access_grant_events retains each actual location capability grant/revoke with actor and target. First-admin trusted initializer is atomic and empty-application-only; no runtime bootstrap/public signup or data backfill. Applied locally after encrypted backup/isolated restore table-count verification; native tests pass.

Version31 invoice_draft_revisions.sql deployed locally after encrypted backup/isolated restore-count drill. Adds invoices.version initial1 and before-update trigger incrementing revisions; no historical edit events fabricated. invoice_draft_revisions scoped invoice FK, actor, positive consecutive from/to versions, unique invoice/to_version, before/after object snapshots and timestamp. Draft update holds parent row lock and exact grant/principal locks while replacing items/header and writing revision/event. Any audit failure rolls back all. Issued document immutability remains migration30; no old migration checksum edited.

Version32 member_invitations.sql locally deployed after encrypted backup/isolated restore-count verification. member_invitations stores scoped metadata, SHA-256 digest of random32-byte bearer, explicit expiry, creator and pending/accepted/revoked state with accepted principal/time consistency. Unique pending org/email and token digest. member_invitation_deliveries stores nonce-prefixed AES-256-GCM ciphertext, queued/sent/failed/cancelled and provider/time evidence required for sent; AEAD binds org/invitation. No live delivery worker is implemented. member_invitation_events retains created/revoked/accepted actor/time. Acceptance creates new credential/principal or reuses exact verified existing identity, inserts only org membership, consumes token and audits in one transaction; no grants/customer links/backfill. Old migrations unchanged.

Version33 customer_account_management.sql deployed after encrypted backup/isolated restore-count drill. Adds customer_accounts.manage to location capability allowlist only; grants none. Adds customer_account_events with scoped customer FK, target/actor, enabled/review assertion/time, constraint enabled implies reviewed. Existing customer_account_links schema/ownership unchanged; no auto-link/backfill. Grant/revoke link and event in same transaction; exact actor permissions and customer/target eligibility locked, per-tuple advisory serialization. Explicit ownership links remain independent from staff membership removal; active-account check protects portal, and administrators can revoke residual links after membership removal/deactivation.

Version34 booking_capacity_requests.sql locally deployed after encrypted backup/isolated restore/count check. Unique non-null bookings.reservation_id prevents duplicate backing; scoped booking identity FK supports actor/payload-bound retry receipts. Deferred booking/reservation constraint triggers validate service/add-on identity, actual start, exact occupied duration/buffers/units and active-booking non-cancelled reservation. Pre-existing linked bookings validated during migration; inconsistent/duplicate legacy records cause migration refusal, not automatic repair. New shared transaction helpers combine booking and capacity writes/history/audits; no old migrations edited or operational records seeded. Unreserved legacy staff records remain separate from reserved bookings; pricing/payment approval and complete lifecycle still open.

Migration35 booking_reschedule_requests locally deployed after encrypted backup/isolated restore matching table counts: scoped booking FK, actor, SHA256 request digest,16–128 requestkey, positive from_version and to_version=from+1, org/location/key uniqueness. Atomic reschedule receipts are separate from booking creation receipts; no operational data backfilled. Existing reservation resource/rule may change in one transaction with exact binding checks at commit. See latest checkpoint and verification artifact for tests/limits.

Schema36 booking_offers + booking_offer_revisions locally deployed. Offers require explicit rule revision/resource/category, exact CAD amounts, terms, notice/window/slot settings and publication reviewer; versioned atomic snapshot/audit. Normalized scoped rule/category uniqueness; quote_requires zero amounts, deposit<=subtotal+tax. No offer or approval backfill; current app has0offers/revisions. See latest checkpoint for native/HTTP/browser and deployment evidence.

Schema37 customer_booking_requests and customer_booking_request_events locally deployed after encrypted backup/isolated restore-count check. Frozen offer/config/rule/equipment snapshots, reviewed-information assertion, exact actor/payload key, scoped FKs/revision reference, append-only guards and history indexes. No actual requests/backfill. Future fulfillment/status/retention/redaction requires explicit additional workflow; current request is unconfirmed and never reserves/charges. See latest checkpoint for tests/limits.

Schema 38 scoped_quotations.sql locally deployed after encrypted backup/isolated restore-count verification. Expands location_grants_action_check with 'quotes.read', 'quotes.create', 'quotes.manage', 'quotes.issue' and organization_grants_action_check with 'quotes.read', 'quotes.manage'. Adds versioned `scoped_quotations` (draft, issued, accepted, declined, expired, superseded, fulfilled) attached to immutable customer booking requests, `scoped_quotation_items` with optional catalogue service snapshots, immutable `scoped_quotation_events` audit ledger, immutable `scoped_quotation_acceptances` capturing non-repudiation customer evidence (terms reviewed, SHA-256 canonical quote content digest, actor ID, unique idempotency key), and `quote_fulfillment_receipts` enforcing exactly one conversion receipt per booking request and per quotation. Adds `bookings.quotation_id` foreign key. Quoted totals are strictly server-computed in integer cents. Native PostgreSQL tests, optimistic edit/issue concurrency, sorted-lock concurrent conversion, capacity loss rollback and immutable trigger rejection verified.

## Migration41 — cumulative payment refund accounting (2026-10-23)
Registered after explicit parallel-agent allocation;1..40 unchanged. payments.refunded_cents is bounded0..amount_cents. Composite scoped amount key binds each payment_refund_adjustment to exact payment/captured cents; immutable adjustment records preserve previous/new cumulative totals and positive delta, unique payment/new total. Existing status=refunded rows fail migration until reviewed historical reconciliation; do not invent partial totals. Candidate compatibility/constraints/legacy-refusal tests now run against registered41 in disposable schemas.
Provider reconciliation locks invoice then payment, refuses ambiguous provider references, applies monotonic cumulative deltas and exact net invoice change atomically with adjustment/event/audit; duplicates/older totals no-op, stale failure cannot reverse capture. Partial refunds retain completed status with separate cumulative total, full refund uses refunded. Does not claim approved merchant binding/durable provider event inbox/external-charge recovery. Local deployment pending final native run at note creation.

## Public content43 and linked central services45 — 2026-10-24
43adds public_sites,service_publications,content_media,portfolio_images,content_events and the exact-location content.manage capability. Public metadata references operational catalog_services by scoped FK; names/prices are not duplicated. Media bytes are sanitized JPEG/PNG; publication/version/audit states persist. Applied43checksum was found in the shared application and restored exactly in source; this chat did not apply it.
45adds nullable explicit central_service_id scoped FK/unique location mapping and central revision propagation trigger, current local versions/revisions/audits, identity/dependency rejection, without changing historical job/invoice snapshots. Source/native tested in disposable schemas; not applied by website to the active database.42payment candidate unchanged;44role schema belongs to separate owner. New source needs coordinated44/45migration and runtime activation. No real data/grant seeding or guessed central mappings.
