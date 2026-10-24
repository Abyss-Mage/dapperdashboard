# RBAC and authorization matrix
SOW 5.3 roles are proposed and require approval. No live franchise role assignment is inferred from these descriptions.

| SOW role | Proposed scope from SOW | Current runtime status |
| --- | --- | --- |
| Super Administrator | Explicit platform-wide authorized administration | No production role or global bypass implemented |
| Franchise Administrator | Assigned franchise operations and authorized records | Visibility/authority pending DEC-01/02 |
| Location Manager | Assigned location operations | Policy and workflow actions pending |
| Employee / Detailer | Available work pool in authorized locations; permitted attendance | No fixed assignments per direct user change; portal parity pending |
| Finance / Accounting | Authorized financial documents and reports | Issuer/tax/finance grants pending |
| Customer | Own profile, vehicles, bookings and invoices | Production portal ownership not implemented |

Implemented PostgreSQL authorization is deny-by-default action grants on exact organization/location tuples. customers.read, customers.create, vehicles.read and vehicles.create are independent. An active principal plus a valid session is necessary but not sufficient. Session actor is resolved server-side; path/body/header selectors never create a grant. Grants are rechecked for each operation. Deactivation revokes existing sessions even if the account is later re-enabled.

Real database and HTTP tests cover cross-organization selection, mismatched owner links, unknown actions, revoked grants, disabled principals, malformed/expired/revoked sessions and logout. The shared-cookie JSON demo does not provide this production isolation and is labeled synthetic.

Pending: approved central/franchise visibility rules, customer ownership routes, employee assignment grants, financial/report/file/export checks, administrator grant-management authorization, RLS and least-privilege DB roles. No role matrix is marked approved or fully implemented.


New tested actions: jobs.read/create/complete and catalogue.read/manage on exact location scopes; structure.read/manage on exact organization scopes. These are explicit capabilities, not role-name bypasses. User confirmed Owner service editing; actual owner provisioning and central/franchise override rules remain to be configured. Revoked/foreign grants are denied over repository and HTTP routes. Mutation grant/principal locks serialize revocation/deactivation with job/catalogue/structure writes.

Customer portal foundation (schema26, source/native tests): account links are explicit principal/customer tuples with exact org/location ownership FK, never inferred by email or staff grants. Session resolves actor; membership locks protect reads and own-equipment writes. No account enrollment HTTP route or automatic runtime links. Production enrollment/verification remains incomplete.

Schema29 access administration: active principal + explicit organization membership + access.manage required. Target membership and exact organization/location ownership checked. No automatic role inheritance, financial rights, fixture account adoption or global bypass. First administrator receives structure.read/manage, catalogue.read/manage and access.manage only; actual location capabilities require deliberate grant. Trusted human setup for admin@dapperdash.ca is prepared, not executed. Native/HTTP grant/revoke/rollback/isolation tests pass; real-account browser acceptance pending.

## Website publishing capabilities — 2026-10-24
Direct user confirms permission-controlled shared services and portfolio management across website/Workspace/Master. Actual grants remain deliberate: organization catalogue.manage edits central services; organization catalogue.read plus exact-location catalogue.manage enables an explicit linked local service; location catalogue.manage publishes public descriptions of those same IDs; location content.manage configures the public site and manages/uploads/publishes/unpublishes portfolio images. No role-name/email bypass or automatic capability assignment.
Revoked/foreign direct grants and private media access were tested negatively.44role-only mutation/transaction/central-grant support remains the separate role owner's integration task; role-only publishing acceptance is not claimed. See PUBLIC_WEBSITE_PARITY. Public read APIs return only explicitly published content and never customer/staff records.
