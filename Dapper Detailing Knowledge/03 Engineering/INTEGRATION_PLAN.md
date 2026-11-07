# Integration plan
## Square
Proposed Go adapter and persisted financial allocation; imported ERPNext allocation models are reference knowledge only. Sandbox first. Store provider references and events, verify signatures, deduplicate webhooks and checkout requests, reconcile actual confirmed results. Merchant routing/POS hardware/refunds require client decisions. No Stripe/Polar substitution.

## Calendar
Evaluate scoped ICS subscription feeds and CalDAV against approved accounts, sharing and update latency. Create/update/cancel the same event identity. Direct bidirectional editing remains excluded without approval. Existing in-app calendar is not Apple synchronization.

## Email
Implement a durable Go outbox/templates/notification adapter; Frappe is reference knowledge, not the runtime. Configure transactional sender and scoped routing; test retries, delivery-state visibility and invoice-finalization triggers. A local capture inbox is not external-delivery evidence.

## Mailchimp and popups
Add consent-specific subscription adapter and configurable form settings. Do not infer marketing permission from bookings. Track duplicates, status, errors, selected pages, delay, repeat limits and suppression after successful submission.

## Migration
Use repeatable imports with source IDs, batch hashes, dry-run mode, row errors, count reconciliation and future-booking verification. Source exports and timezone/payment mappings remain blocked. Do not load real records into demonstrations without authorization.

