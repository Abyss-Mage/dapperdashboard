---
name: dapper-tenant-security
description: Change or verify Dapper scope grants, ownership, sessions and tenant isolation without assuming client role policy.
---

# Dapper tenant security
The canonical RBAC_MATRIX.md and OPEN_DECISIONS.md distinguish proposed SOW roles from approved grants. Current PostgreSQL grants are exact principal/organization/location/action tuples. No central or franchise visibility is implicit.

Resolve the actor from a verified session, then authorize the selected scope. Path/body organization or location selectors never establish authority; reject actor injection. Verify list and mutation paths independently and use scoped ownership keys for related records.

Current session hashes, expiry, revocation and deactivation behavior live in internal/postgres. Re-enabling an account must not revive its old sessions. Keep JSON-demo evidence separate from scoped HTTP evidence.

Exercise two synthetic organizations, mismatched customer/vehicle links, revoked grants, inactive principals and unauthenticated requests. Extend coverage to files, reports, exports and finance as those paths are implemented. Source presence or hidden navigation is not isolation evidence.

RLS, production least-privilege roles, administrator provisioning and a live identity provider remain incomplete. This skill does not authorize account/grant changes on live systems or invent client permissions. No delegation or WSL.
