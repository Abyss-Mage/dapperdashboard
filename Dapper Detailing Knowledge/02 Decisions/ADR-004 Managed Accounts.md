# ADR-004 — Dapper-managed email/password accounts
Date: 2026-10-22. Status: CONFIRMED direct user answer to the sign-in decision.

The user selected email/password accounts managed by Dapper. Implement native Go account verification using an established Argon2id password-hash helper and the existing PostgreSQL hashed-session lifecycle. OIDC remains optional and disabled; it is not a required provider dependency.

No automatic public signup, administrator grants or real staff accounts are inferred from this decision. Begin with explicit synthetic fixtures and a protected account-provisioning boundary. Keep location/action grants separate from credentials, use Secure HttpOnly session cookies under HTTPS, protect cookie writes and throttle authentication attempts. Password reset/invitation delivery depends on the configured email adapter; MFA capability and production security/acceptance remain required work.

Passwords, recovery/invitation/session tokens and private connection data must not be printed or enter source/Obsidian. Real staff provisioning and approved role policy remain separate from the synthetic local demonstration.
