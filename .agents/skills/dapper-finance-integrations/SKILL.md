---
name: dapper-finance-integrations
description: Implement approved Dapper Square, cash, invoice and reconciliation workflows with exact money and provider evidence.
---

# Dapper finance integrations
Read canonical PAYMENT_WORKFLOWS.md, INTEGRATION_PLAN.md and open issuer/tax/merchant decisions. Square, cash and custom invoice requirements are not replaced by imported ERPNext source or a different payment provider.

Source price strings are reference data, not an approved currency/tax/deposit rule. Use exact financial representations once currency and rounding are confirmed. Do not turn a job-status change into verified charges or a paid invoice.

Separate initiation, verified payment confirmation, reconciliation and customer documents. Persist provider references, authenticate webhooks, make duplicate requests/events idempotent and test delayed/failure/refund paths using approved sandbox access.

Cash needs actor, time, amount, method, booking/invoice link and audit; Square cash sync is not required. Invoice, receipt and paid invoice are distinct. Finalization, correction and delivery triggers need approved authority.

Report source/static/local-sandbox/live-provider evidence separately. Never perform a financial transaction or expand live account access merely because this skill is loaded. Keep credentials out of source, logs and Obsidian.
