# Sprint backlog
One implementer; sequential work. Every implementation ticket must include objective, SOW reference, affected modules, API/database changes, dependencies, security considerations, acceptance criteria, required tests and Definition of Done.

| Ticket | Deliverable | SOW | Dependencies / acceptance |
| --- | --- | --- | --- |
| DAP-001 | Vault, source inventory, repository comparison and pinned import | 1–25 | Source provenance and unchanged import verified |
| DAP-002 | Native Go/Next local demo and baseline acceptance | 14,15,17,18 | ADR-002; loopback runtime and synthetic records persist across restart; no WSL |
| DAP-003 | Dapper mapping/extension design and permissions | 4–7,14 | Client scope/issuer decisions; cross-franchise tests specified |
| DAP-004 | Customer-owned vehicles and service/package adaptation | 3,4 | Approved category/catalogue rules; generalized non-automobile fields |
| DAP-005 | Detailing booking/availability/job adaptation | 4,7 | Approved transitions/durations/capacity/deposits; concurrent capacity tests |
| DAP-006 | Employee parity configuration and minimal extensions | 7 | Source rules/consent available; onboarding/attendance parity demonstrated |
| DAP-007 | Invoice/payment configuration and Square sandbox | 8,9 | Issuer/tax/merchant approvals; idempotent financial reconciliation |
| DAP-008 | Calendar/email/marketing integration | 10–12 | Accounts and calendar/consent decisions; failure/retry evidence |
| DAP-009 | Legacy migration dry run | 13 | Actual exports; deterministic count and sample reconciliation |
| DAP-010 | Responsive QA, security, backup and demo | 14,16–18,22 | Real API/database/browser checks and explicit feature-status report |

## Seven-day allocation
Day 1: source selection, pinned import and baseline runtime readiness.
Day 2: stock workflow acceptance and gap mapping.
Day 3: approved organization/customer/catalogue adaptations.
Day 4: approved booking/job/employee slice.
Day 5: approved finance/provider proofs.
Day 6: authorization/concurrency/browser/recovery tests and fixes.
Day 7: local demo, feature matrix and handoff.
Blocked tickets do not authorize invented rules. Continue independent work; reduce claimed demo scope if decisions arrive late.

## Definition of Done
Persisted behavior verified, appropriate negative/failure tests pass, relevant browser flows checked, migration compatibility reviewed, canonical vault updated and test evidence linked. Upstream source presence and UI rendering alone do not satisfy completion.

