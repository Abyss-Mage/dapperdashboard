# BookingPress Migration Execution & Cutover Engine

Date: **2026-10-23** | Deliverable: **DEL-22** | Launch Gate: **ACC-13**
Authority: Client-provided live BookingPress export (`c:\Users\apexp\Downloads\HPQThVYzbN.txt`).
Implementation script: [`scripts/migrate_bookingpress_to_postgres.py`](file:///C:/Users/apexp/Documents/ChatGPT/Dapper%20Detailing/scripts/migrate_bookingpress_to_postgres.py)

---

## 1. Executive Summary & Provenance Verification
The DEL-22 BookingPress cutover migration engine processes the client-provided live WordPress export, derives deterministic entity UUIDs, maps customers, vehicles, and appointments into canonical PostgreSQL schema, and validates all database constraints, types, foreign keys, and triggers.

- **Source File**: `c:\Users\apexp\Downloads\HPQThVYzbN.txt`
- **Cached Raw Capture**: `.cache/bookingpress-export/bookingpress_export_raw.json` (git-ignored)
- **Source SHA256**: `E7CA26002A4A8151113F4E599272E5FE03674142FFAE65B546C66F79C5D2C4FE`
- **Target Organization**: `9811de21-519e-4be1-ba2f-18fff3552e18` (*Dapper Detailing Pros*)
- **Target Location**: `8093cc38-90b4-43b1-ae1c-3c9054097748` (*dad*)
- **Zero Database Mutations**: Verified. Dry-run and verification transactions are rolled back; zero unapproved rows injected into production.

---

## 2. Reconciled Entity Mapping

| Entity | Scanned Count | Mapped Count | Transformation Safeguards |
| :--- | :--- | :--- | :--- |
| **Customers** | 453 | 453 (100%) | Deterministic UUIDv5 (`customer:{email}`). Names trimmed and checked ($2 \le \text{len} \le 100$). Lowercase unique email binding. Phone sanitized. |
| **Vehicles** | 453 | 453 (100%) | Deterministic primary vehicle UUIDv5 (`vehicle:{email}:primary`) bound to customer. Category `Standard`, description `Customer Vehicle`. |
| **Bookings** | 584 | 584 (100%) | Deterministic appointment UUIDv5 (`appointment:{id}`). Snapshot preservation of historical service names. America/Toronto EDT/EST converted to UTC. |
| **Invoices / Revenue** | 584 appts | CAD $172,385.15 paid | Preserved in appointment notes ledger: `[BookingPress #<id>] Price: CAD $<p> \| Paid: CAD $<paid> \| Gateway: <gw>`. |

---

## 3. Booking Lifecycle Distribution

- `completed`: **364** (BookingPress code 6)
- `confirmed`: **143** (BookingPress code 1)
- `cancelled`: **77** (BookingPress code 3 *Declined* + code 4 *Cancelled*)
- **Total**: **584** appointments

---

## 4. Constraint & Trigger Verification
Execution of `python scripts/migrate_bookingpress_to_postgres.py --verify-sql` verified:
1. **Foreign Key Integrity**:
   - `customers(organization_id, location_id, id)` verified against `locations`.
   - `vehicles(organization_id, location_id, customer_id, id)` verified against `customers`.
   - `bookings(organization_id, location_id, customer_id, id)` verified against `customers`.
   - `bookings(organization_id, location_id, customer_id, vehicle_id)` verified against `vehicles`.
2. **Triggers**:
   - `check_booking_capacity_binding` executes for every booking row. With `reservation_id = NULL`, the trigger gracefully succeeds for historical bookings without corrupting live capacity pools.
3. **Encoding & Special Characters**:
   - Client and server UTF-8 encoding verified with `PGCLIENTENCODING=UTF8`, handling curly quotes and international names cleanly.
4. **Transaction Rollback**:
   - Verification executed in an explicit `BEGIN; ... ROLLBACK;` block, proving 100% schema compliance while maintaining zero operational database mutations.

---

## 5. Artifact Manifest
- Engine: [`scripts/migrate_bookingpress_to_postgres.py`](file:///C:/Users/apexp/Documents/ChatGPT/Dapper%20Detailing/scripts/migrate_bookingpress_to_postgres.py)
- Unit Tests: [`scripts/test_bookingpress_migration.py`](file:///C:/Users/apexp/Documents/ChatGPT/Dapper%20Detailing/scripts/test_bookingpress_migration.py) (5/5 PASS)
- Dry-Run Report: [`output/migration/bookingpress_migration_dryrun.json`](file:///C:/Users/apexp/Documents/ChatGPT/Dapper%20Detailing/output/migration/bookingpress_migration_dryrun.json)
- Generated SQL: [`output/migration/bookingpress_migration_cutover.sql`](file:///C:/Users/apexp/Documents/ChatGPT/Dapper%20Detailing/output/migration/bookingpress_migration_cutover.sql) (1,505 statements)
