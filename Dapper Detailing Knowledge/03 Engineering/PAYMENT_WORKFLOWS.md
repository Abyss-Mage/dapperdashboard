# Payment and invoice workflows
Required by SOW 8–9; payment/invoice runtime is not implemented. The demo does not process funds or present invented financial totals.

Square: support approved online deposits/full payments, supported in-person checkout, authorized refunds and reconciliation. Initiation is not success; only verified provider confirmation can settle a payment. Persist provider references, verify webhook signatures, deduplicate events/requests and reconcile failed/delayed results. Merchant routing, existing POS hardware/application, customer/order synchronization and supported terminal flow remain pending DEC-08 and account access.

Cash: authorized employee records method, exact amount, timestamp, actor, booking/invoice allocation and notes. Cash-to-Square synchronization is not required. Permission checks and an immutable audit trail are acceptance criteria, not current functionality.

Invoices: draft, reviewed finalization, unique approved issuer/number/date, customer/equipment details, itemized services/add-ons/discounts/taxes, total/paid/balance/status and relevant terms. Support PDF, scoped search/history, partial balances where required and audited corrections/refund/credit adjustments. Invoice, receipt and paid invoice must remain distinct.

Automatic delivery follows configurable completion and finalization rules. Never send a draft as a paid invoice or silently treat a job-status change as verified charges. Legal issuer/company mapping, tax/rounding, numbering, refund authority, payment/deposit and delivery triggers need approval before enabling mutations.

Financial persistence must use exact decimal/minor-unit handling with confirmed currency; exported BookingPress price strings are source references only. No Square SDK, live merchant tests, production payment acceptance or invoice-ledger claims yet. Provider credentials and account data must never enter this vault or source.

## Current real application (ADR-005)
The active finance API has PostgreSQL invoice/payment persistence, but approved issuer/tax/numbering and full provider flows remain incomplete. User-authorized methods are Square and cash. Manual RecordPayment currently accepts authorized cash only; square_card/square_terminal return a provider-verification conflict and cannot settle an invoice from an operator-entered reference. Unapproved bank-transfer/cheque methods are rejected. UI no longer prefills a sample service/price/tax percentage; reviewed tax amounts are explicit minor-unit entries. Full Square checkout/webhook/refund/reconciliation remains required; a disabled manual-card shortcut does not complete that scope.

## Cash retry integrity — 2026-10-23
Cash requires a bounded stable request key. Same scoped key + actor + invoice + exact amount/method/notes yields one receipt under concurrent retry; conflicting key reuse rejects. Distinct concurrent receipts serialize on invoice balance. Payment write/amount update/event commit together; audit failure rolls all back. A payments.record-only operator receives a minimal balance summary without internal invoice notes/history; no post-commit dependency on invoices.read. UI stores only random retry key and hashed payload identity in per-tab session storage, reusing uncertain retries until confirmed success. Native database and authenticated HTTP concurrency/conflict/authorization/audit tests pass. This does not complete issuer/tax/finalization/Square/reconciliation policy.

## Draft editing implementation plan — 2026-10-23
Proceeding with production invoice draft corrections: explicit invoices.manage, exact line/tax validation, atomic header/item/audit revision writes and optimistic invoice version. Finalization must reject a draft version changed since operator review. Issued/void documents cannot be edited; cash remains barred on drafts. No new tax/discount/legal policy approved. Full customer correction/credit/refund/PDF/email workflows remain separate. New migration31 planned; source/native/browser/deployed evidence will be recorded separately. One-time03:44 quota check succeeded (100% remaining) and was deleted as requested; no recurring checks recreated.
