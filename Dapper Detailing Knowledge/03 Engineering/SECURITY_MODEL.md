# Security model
Current demo evidence and proposed production controls are separate.

The Go demo binds to loopback, rejects non-loopback Host values, requires DAPPER_DEMO=1, uses a random HttpOnly SameSiteStrict cookie, checks the exact write Origin and accepts bounded strict JSON. Local synthetic JSON is not production storage. The shared demo cookie does not establish a real user, role or tenant boundary.

Before real users/data: implement production identity/session lifecycle, per-request organization and ownership authorization, PostgreSQL constraints, audit records, protected files, backup restoration and provider-secret management. Test two synthetic franchises across list/detail/search/export/report/attachment/mutation paths. Client visibility/legal issuer decisions remain pending; a selector never substitutes for authorization.

Private ID documents, signatures, photos and GPS need restricted downloads, upload validation and approved retention. GPS stays disabled pending collection policy. Credentials and real exports must not enter source or Obsidian.

WordPress exporter: authenticated administrator with manage_options; GET-only routes, bounded pagination, explicit catalogue fields, public-only pages/media and cross-origin browser rejection. Isolated PHP tests are not live WordPress/application-password/site-schema evidence.

Square requires verified signatures, idempotent request/event handling and reconciliation. Production release needs real authorization, concurrency, security and restore evidence. Imported upstream tests are reference material, not Dapper acceptance.

## Invitation work in progress — 2026-10-23
Building explicit organization-member invitations behind access.manage, with chosen bounded expiry, hashed single-use bearer tokens and encrypted delivery payloads. No public unrestricted signup, default permissions, password reset or email-based customer linking. Existing managed identity must prove its existing session before accepting a membership invitation; invitation token cannot replace its password. New identity chooses its own password. Email delivery remains a separately configured provider adapter; queued must never mean sent. Invitation service must require an explicit application encryption key and HTTPS enrollment origin. No real invitation, account or external message is being created during implementation. Source/native/HTTP/browser/deployment evidence will be recorded separately.

## Customer ownership provisioning plan — 2026-10-23
Implement explicit reviewed customer-account link/revoke with BOTH organization access.manage and a new exact-location customer_accounts.manage capability. No capability is auto-granted. Target enrollment must be an active managed account and explicit member of the same organization; no email inference or public self-link. Customer and account selectors rechecked server-side. Grant requires explicit ownership-review assertion and audit; revoke must work on previously linked inactive/non-member identities to remove residual portal access. Keep arbitrary business-role/franchise policies pending. This is controlled access provisioning, not independent identity-proof/legal verification or customer invitation delivery acceptance.

Schema33 reviewed ownership management is now source/native/HTTP/isolated-browser verified. access.manage alone cannot link; customer_accounts.manage alone cannot link. Managed active target membership and explicit review required for grant; no email inference or default grants. Revocation must not be prevented by target membership/credential removal. Review assertion records operator authorization, not independent legal identity-proof evidence. Portal links and staff membership have distinct lifecycles; membership deletion does not silently rewrite customer ownership. Full role/policy/client approval and real-account acceptance remain open.
