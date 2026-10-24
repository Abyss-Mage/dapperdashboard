# ADR-003 — Explicitly configured OIDC identity adapter
Date: 2026-10-22. Status: PROPOSED technical implementation; client identity provider/deployment is not approved or configured.

The Go backend will reuse coreos/go-oidc and golang.org/x/oauth2 for signed ID-token verification and authorization-code PKCE. It will require exact issuer/client audience/expiry/nonce checks, a one-time browser-bound state flow and Secure HttpOnly cookies. It will not create users automatically or accept a client-selected actor as identity. Only an existing active issuer/subject mapping can establish a workspace session; location/action grants remain separate and explicit.

No live provider is selected, registered or contacted merely by adding the adapter. Default runtime remains disabled without complete private provider configuration. The synthetic JSON demo stays isolated. WordPress credentials and its administrator session are not platform identities.

Use short-lived bounded process-local login state initially; server restart fails closed. Durable/multi-instance state, production identity/MFA configuration, HTTPS reverse-proxy validation, least-privilege DB roles, rate-limit coverage and release review remain acceptance work. No production-ready claim from simulated provider tests.

Sources: [Go OIDC implementation](https://github.com/coreos/go-oidc), [Go OAuth2 PKCE API](https://pkg.go.dev/golang.org/x/oauth2#S256ChallengeOption). Keep dependency versions in go.mod/go.sum and preserve applicable license notices.
