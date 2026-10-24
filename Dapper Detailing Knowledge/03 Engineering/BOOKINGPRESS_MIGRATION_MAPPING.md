# BookingPress migration mapping & reconciliation specification

Deliverable: **DEL-22** | Launch Gate: **ACC-13**
Authority: SOW 13, Master Specification Section 19.

## 1. Overview & Provenance Rules
The legacy booking engine runs on WordPress / BookingPress. To ensure clean migration to Dapper Detailing's native Go / PostgreSQL platform, migration must adhere to strict provenance and reconciliation safeguards:
1. **Provenance Preservation**: Every migrated record retains its `source_system = 'bookingpress'`, `original_source_id`, `batch_id`, and source capture timestamp.
2. **Zero Database Pollution**: Dry runs operate strictly offline or in disposable schemas. No unapproved records are injected into the active `dapper_application` operational database.
3. **Exact Currency & Duration**: Prices are parsed as exact integers (cents) using half-up rounding. Durations are normalized to minutes (`m`, `h`, `d`). Anomaly flags (such as 1-minute placeholder durations) are explicitly tagged rather than silently mutated.
4. **Case-Insensitive Customer Deduplication**: Customer identity is bound by lowercase, trimmed email address.

---

## 2. Entity Mapping Matrix

| Legacy BookingPress Entity | Canonical Dapper Entity | Key Mappings & Transformations |
| :--- | :--- | :--- |
| `bookingpress_categories` | `catalogue_categories` (tags) | `bookingpress_category_id` -> `source_id`; `bookingpress_category_name` -> `name` |
| `bookingpress_services` | `services` | `bookingpress_service_id` -> `source_id`; `category_id` -> FK link; `service_price` -> `price_cents` (cents integer); `duration_val` + `unit` -> `duration_minutes`. 1-minute durations tagged `one_minute_duration_review`. |
| `bookingpress_extras` | `job_addons` | `bookingpress_extra_service_id` -> `source_id`; linked to service IDs; price and duration converted. |
| `bookingpress_customers` | `customers` | `customer_email` normalized to lowercase; `customer_phone`, `customer_name` mapped. Duplicate emails merge into single customer with multiple appointments. |
| `bookingpress_appointments` | `bookings` + `jobs` + `capacity_reservations` | Status mapping: `1 (Approved)` -> `confirmed`; `2 (Pending)` -> `pending`; `3 (Rejected)` -> `declined`; `4 (Cancelled)` -> `cancelled`. Datetimes normalized to UTC from `America/Toronto`. |
| `bookingpress_payment_logs` | `invoices` + `payments` | Amount converted to `cents`; status mapped (`Complete` -> `paid`, `Pending` -> `pending`). Square transaction references preserved where available. |

---

## 3. Discrepancy & Orphan Detection
The assessment engine (`scripts/assess_bookingpress_migration.py`) scans for:
- **Orphaned Services**: Services referencing category IDs that do not exist in `categories`.
- **Orphaned Extras**: Add-ons referencing service IDs that do not exist in `services`.
- **Orphaned Bookings**: Appointments referencing non-existent service IDs or missing customer emails.
- **Duration Anomalies**: Services with <= 1 minute or > 24 hour durations.

---

## 4. Dry Run & Assessment Results
Ran against current cached reference export (`dapper-platform/apps/api/data/bookingpress-reference.json` & `bookingpress-extras-reference.json`):
- **12 Categories** mapped.
- **79 Services** mapped.
- **228 Extras** mapped.
- **Provenance verified**: Source SHA256 checksums verified against manifest.
- **Reconciliation output**: `output/migration/bookingpress_migration_assessment.json`.
- **Active operational records created**: 0 (zero database pollution).
