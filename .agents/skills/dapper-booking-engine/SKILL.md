---
name: dapper-booking-engine
description: Implement Dapper scheduling, booking transitions and capacity checks using approved service rules.
---

# Dapper booking engine
Read canonical BOOKING_WORKFLOWS.md, the SOW section 4 and OPEN_DECISIONS.md. Source status names are requirements; transition edges, authority, deposits, cancellations and no-show policy still need approval.

Maintain one Go availability authority across customer/admin/employee interfaces. UI Calendar and Bookings are presentations of shared records, not independent schedulers. Calendar cards currently show synthetic starts; their height is not duration.

The BookingPress reference catalogue keeps raw durations and flags three one-minute source values. Do not silently repair those values, use them as approved slot lengths, or infer equipment capacity from category names.

For an implemented reservation primitive, prove overlapping concurrency, atomic rescheduling, resource/location ownership and timezone/DST behavior with database-backed tests. Keep quote-required services and payment-dependent confirmation gated until policy is approved.

Required intake channels include website, phone, walk-in, manual, dealership and fleet; a cash walk-in still needs a booking record. Preserve unresolved requirements in traceability rather than deleting them for the demo.
