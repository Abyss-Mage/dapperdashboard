# ADR-005 — Production application and website API
Date: 2026-10-22
Status: CONFIRMED direct user instruction. Supersedes ADR-002's demo-product direction, while retaining native Windows, unified Go/PostgreSQL, Next.js and no WSL.

The user requested production-grade implementation, removal of demo details and real workflows/APIs that will connect with the future Dapper Detailing website.

The active application must use verified accounts, explicit authorization and PostgreSQL persistence. Remove shared demo sessions, JSON persistence and fabricated dashboard records from active routes and launch paths. Retain isolated synthetic tests and historical evidence as engineering material, outside the active product. Do not relabel existing synthetic database records as real operations. Use a separate unseeded application database; do not import old fixtures or create default privileged users.

The website and operations interfaces use the same Go domain authority. Customer APIs enforce explicit ownership. Website integration must not expose internal staff APIs or trust browser-supplied identity, price, duration or scope. Provider operations require actual verified integrations, not manually fabricated success.

Production-focused development does not mean production acceptance has been achieved. Hosting/domain, least-privilege database roles, approved issuer/tax/booking policies, provider credentials, recovery/monitoring and full client acceptance remain unfinished. Retain these gates honestly without demo branding in customer/operator flows.

No legacy system modification or cutover is authorized. No external messages, real charges, destructive data cleanup, automatic account-linking or production grant provisioning is implied. One implementer; no WSL or delegation. Continue independently around missing approvals and stop/checkpoint at the existing usage threshold.
