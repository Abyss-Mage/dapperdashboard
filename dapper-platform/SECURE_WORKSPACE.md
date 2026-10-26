# Individual-session workspace (native Windows)

This is a synthetic development preview, separate from the shared JSON demonstration. It is not production-ready. No WSL, public bind, automatic certificate trust or real staff provisioning is performed.

- Shared demo: http://127.0.0.1:3006/
- Individual-session workspace: https://127.0.0.1:8443/workspace
- Scoped Go API: loopback 8121; native PostgreSQL: loopback 15432.

`setup-workspace.ps1 -SessionLifetime '1h'` explicitly builds native tools, creates a fresh `dapper_workspace_demo_...` schema, applies ten checksummed migrations, seeds only synthetic organization/location/account/service/work-order records, generates a local certificate and saves the generated test credential using Windows-user encryption under ignored `.cache/workspace-runtime`. It refuses an existing settings file or database schema. The explicit one-hour value is a synthetic test setting, not approved production session policy.

`start-workspace.ps1` starts only the scoped API and TLS gateway, records PID/start time/executable and requires matching migrated schema. `stop-workspace.ps1` stops only matching recorded processes. The frontend/demo has its own start/stop scripts. Keep unrelated port 3000 intact.

The gateway routes /api/v1 to the scoped API and frontend assets/pages to Next.js. The root redirects to /workspace. Password sign-in, scope discovery, versioned service changes, organization structure management and open-work completion are connected in this page. No shared-demo cookie fallback exists. Ordinary HTTP keeps credentials/sign-in disabled.

The local certificate is generated but not installed into the system trust store. Browser review is a user handoff; agents must not bypass browser certificate warnings. The HTTPS acceptance script validates the exact local certificate and hostname using a private trust context, not disabled TLS validation. Certificate/private key and generated credential stay ignored; never paste passwords, keys, session cookies or private connection data into chat/source/Obsidian.

Trusted operator command `cmd/account` provisions a credential only for an existing active principal. It requires DAPPER_ACCOUNT_PROVISION=1 and private database configuration, reads the password from bounded standard input and changes no grants. It is not public signup or an employee invitation/email workflow. Actual staff/owner onboarding, recovery delivery, MFA and least-privilege deployment remain required work.

Verification:
- `scripts/test-postgres.ps1`: real isolated-schema persistence, authorization, migration and HTTP tests.
- `scripts/verify-workspace-runtime.py`: accepts the synthetic credential only on stdin; performs actual HTTPS login/CRUD/completion/isolation/logout and writes sanitized evidence. No cookies/passwords are persisted by this verifier.
- `scripts/test-workspace-restore.ps1`: backs up only the dedicated synthetic schema, encrypts with Windows DPAPI, decrypts and verifies the digest, restores into a fresh disposable database, compares selected table counts, then removes the test database and plaintext files. Encrypted backup stays ignored. This is not off-site or production disaster-recovery qualification.

Current evidence: output/acceptance/scoped-runtime-verification.json and synthetic-restore-verification.json. Browser authenticated-flow acceptance is pending local certificate/user handoff. Do not label it passed from API tests alone.
