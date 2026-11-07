# Live BookingPress Plugin Export Assessment

Date: **2026-10-23** | Deliverable: **DEL-22** | Launch Gate: **ACC-13**
Authority: Client-provided live BookingPress export (`c:\Users\apexp\Downloads\HPQThVYzbN.txt`).

---

## 1. Provenance & Integrity Verification
- **Raw Export SHA256**: `E7CA26002A4A8151113F4E599272E5FE03674142FFAE65B546C66F79C5D2C4FE`
- **File Size**: 19,451,569 bytes (~19.5 MB)
- **Source Site**: `https://dapperdetailing.ca`
- **Export Keys**: `export_key: 3_1792764764`, `export_site_key: 4_1781083616`
- **Active WordPress Add-ons Confirmed**:
  - `bookingpress_lite`, `bookingpress_pro`
  - `coupons_module`
  - `service_extra_module`
  - `tip_addon`
  - `tax_addon`
  - `invoice_addon`
  - `conversion_tracking_addon`
- **Raw Capture Storage**: Securely stored in `.cache/bookingpress-export/bookingpress_export_raw.json` (git-ignored, strictly local). Zero customer PII or credentials are committed to source or Obsidian.

---

## 2. Entity Inventory & Reconciliation Summary

| Entity Type | Scanned Count | Valid / Mappable | Issues / Observations |
| :--- | :--- | :--- | :--- |
| **Categories** | 12 | 12 (100%) | Exact match with previously verified reference catalogue. |
| **Services** | 79 | 79 (100%) | Exact match with previously verified reference catalogue. |
| **Extra Services (Add-ons)** | 228 | 228 (100%) | Linked to service IDs; exact match with extras reference. |
| **Customers** | 453 | 453 (100%) | 100% have valid email addresses; 100% unique (no duplicates). |
| **Appointments** | 584 | 584 (100%) | 100% link to valid customers in the export. 479 link to active services; 105 link to historical retired services with full snapshot preservation. |
| **Coupons** | 18 | 18 (100%) | Promotional discount codes with fixed/percentage rules. |
| **Notification Templates** | 32 | 32 (100%) | Email & SMS notification trigger definitions. |

---

## 3. Financial & Operational Metrics
- **Currency**: **100% CAD** across all 584 appointments and 79 services (resolves prior open uncertainty on operational currency).
- **Date Range**:
  - Earliest appointment: **2024-06-21**
  - Latest scheduled appointment: **2026-10-29**
- **Lifecycle Status Breakdown**:
  - `completed` (code 6): **364** appointments
  - `confirmed` (code 1): **143** appointments
  - `declined` (code 3): **76** appointments
  - `cancelled` (code 4): **1** appointment
- **Financial Reconciliation**:
  - **Total Historical Paid**: `CAD $172,385.15` (17,238,515 cents)
  - **Total Historical Due**: `CAD $16,886.62` (1,688,662 cents)
  - **Total Historical Volume**: `CAD $172,623.68` (17,262,368 cents)
- **Payment Method Distribution**:
  - `on-site` (in-person terminal / cash): **486**
  - `manual` (staff entry): **58**
  - `square` (online card payment): **38**
  - `unspecified` (`-`): **2**

---

## 4. Key Architectural Discoveries & Cutover Safeguards
1. **Historical Service Snapshots**:
   105 historical appointments reference earlier service IDs (e.g. ID 9 "SUV - VIP Interior Detail", ID 6 "Sedan - VIP Interior Detail") that were subsequently replaced or consolidated in the WordPress admin. The export preserves `bookingpress_service_name`, duration, and price on the appointment record itself. Dapper's job model naturally supports this via snapshot preservation (`service_name`, `description`, and item lines).
2. **Customer Identity Binding**:
   All 453 customer records have unique, well-formed emails. In Dapper, these will map directly to scoped `customers` records, linked by lowercase email address.
3. **Square Reference Linking**:
   The 38 online Square transactions contain provider payment tokens that can be correlated with Square merchant transaction logs.

---

## 5. Safeguard Verification
- **Zero Database Pollution**: Verified. The assessment was performed entirely in Python dry-run memory; zero records were written to `dapper_application`.
- **Zero PII Exposure**: Verified. The generated report (`output/migration/bookingpress_live_export_assessment.json`) contains only aggregated metrics, counts, and anonymized issue codes.
- **Repeatability**: Can be re-executed deterministically at any time via `python scripts/assess_bookingpress_migration.py`.
