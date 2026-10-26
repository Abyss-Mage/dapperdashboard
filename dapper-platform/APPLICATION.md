# Dapper application

Current direction: ADR-005, production-grade workflows and APIs shared with the future Dapper website. The application uses Go/PostgreSQL and Next.js. ERPNext/HRMS imports are reference material only. No WSL.

Native local prerequisites: project PostgreSQL on 15432, installed Go/Node, a built Next.js application, an explicit HTTPS certificate directory, and Windows-encrypted local database credentials. The local database role and certificate are development infrastructure, not qualified production hosting/security.

Build/check:

```powershell
pwsh -NoProfile -File scripts/test-postgres.ps1
npm --prefix dapper-platform/apps/web run build
```

First local startup uses a new, empty `dapper_application` database; it refuses an existing database and creates no organizations, customers, accounts or permissions:

```powershell
pwsh -NoProfile -File scripts/start-application.ps1 -SessionLifetime 1h -InitializeDatabase -CertificateDirectory C:\private\dapper-local-tls
```

Provide `certificate.pem` and `private-key.pem` in that private directory. The script never installs certificate trust. One hour is the currently explicit local session configuration; choose and approve production session policy separately.

Later startup:

```powershell
pwsh -NoProfile -File scripts/start-application.ps1 -SessionLifetime 1h
pwsh -NoProfile -File scripts/stop-application.ps1
```

Open `https://127.0.0.1:8443/workspace` for operations, `/account` for customers, `/booking` for the future website booking entry point. API routing belongs to the HTTPS gateway. The web process has no API rewrite/fallback to the retired JSON service. API connections remain local; external deployment requires approved ingress/origins/TLS and production hardening.

No default account exists. The authorized first administrator is `admin@dapperdash.ca`. A human operator creates it once, choosing the password privately in a Windows terminal from the project directory:

```powershell
pwsh -NoProfile -File scripts/initialize-administrator.ps1 -Email admin@dapperdash.ca -Organization "Dapper Detailing Pros"
```

The script prompts twice without displaying the password, sends it only over standard input to the trusted local tool, and persists an Argon2id hash. It refuses an application containing any existing organization or principal. One transaction creates the organization, account, explicit membership, structural/catalogue/access-management capabilities and audit. It creates no location, financial grants, customer links or sample data. There is no public first-admin endpoint. Staff/customer enrollment, recovery/MFA and complete role policy remain unfinished.

After sign-in, create actual franchises/locations and use Location access to assign exact capabilities to explicit organization members. Creating a location does not automatically grant operational rights. Do not invent locations or grant all capabilities merely to populate screens. Real customer imports require approved reconciliation/cutover.

Apply later reviewed migrations only with the application stopped:

```powershell
pwsh -NoProfile -File scripts/stop-application.ps1
pwsh -NoProfile -File scripts/migrate-application.ps1 -Apply
pwsh -NoProfile -File scripts/start-application.ps1 -SessionLifetime 1h
```

Migration maintenance first creates a Windows-user-encrypted backup and restores it into a separate temporary database. It compares all public table counts before applying reviewed migrations, preserves the encrypted backup and removes temporary plaintext files. This local restore/count check does not qualify row-content equality, off-site disaster recovery or production backup policy.

Website integration uses the same server-side customer ownership, catalogue and capacity authority as operations. Use same-origin secure sessions through approved ingress; separate-domain integration requires explicit origin/CSRF/session design. Never copy browser-calculated prices, availability, actor IDs or provider payment success into trusted records.

`reference/retired-demo` preserves obsolete demo code outside active application paths. Prior fixture databases are retained but not connected or relabelled as application data.

Member invitations are now a configured capability, not unrestricted signup. The API requires an explicit DAPPER_INVITATION_ENCRYPTION_KEY containing a base64-encoded32-byte key and the configured HTTPS client origin. Keep that key in an approved private secret store; never source/Obsidian/browser/log it. No key is automatically generated or configured by this implementation. Losing or replacing the key without a planned payload migration can make queued deliveries unreadable; key rotation/recovery and the actual email provider adapter remain unfinished.
Authorized access managers can queue/revoke explicitly expiring invitations once configured. Queued does not mean sent. No email worker/provider is connected, so do not issue real invitations or claim delivery acceptance yet. Enrollment links use a URL fragment cleared into page memory; no bearer token is returned to the administrator or stored in browser storage. Existing managed accounts must sign in in another tab to accept and cannot reset their passwords through the link. New account holders choose their own password. Acceptance gives org membership only; grant exact capabilities afterward. Customer ownership links, recovery and MFA remain separate work. Actual administrator initialization remains pending human password entry.

Customer portal ownership is explicitly provisioned after enrollment. An authorized operator needs both organization access.manage and exact-location customer_accounts.manage; neither is automatically added by location creation or enrollment. Select actual customer and enrolled member, review authorization and grant; no email matching occurs. Revoke existing links from the same scoped screen, including disabled/former members. Removing staff membership alone does not remove an independent customer ownership link. No real accounts or links were created during implementation. Lists are currently bounded100; complete search/pagination and full customer lifecycle acceptance remain open.

Staff with exact bookings.create and scheduling.reserve can atomically reserve configured service capacity and create a pending booking. The workspace form additionally needs customer/equipment read rights for its selectors. Select explicit resource/rule and timestamp including UTC offset; server calculates duration/buffers/units. Reuse retained request key on uncertain retries. No payment or customer confirmation is implied; the public /booking journey remains unfinished. Implicit creation no longer confirms a booking. Existing reservation cannot back multiple bookings or be moved/released inconsistently; dedicated atomic move/cancel workflows and approved eligibility/refund rules are still required.
Isolated browser suites: scripts/test-invoice-browser.ps1 -Suite Invoice|CustomerAccounts|BookingCapacity, followed by scripts/finish-invoice-browser.ps1. These use a disposable schema in the postgres test database and separate3007/8444 ports, never dapper_application. Finish now requires terminal successful Go package output (including deferred cleanup), not only a verified marker.
