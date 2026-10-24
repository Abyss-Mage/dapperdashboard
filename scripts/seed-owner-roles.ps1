# Machine-owner script. Creates the three owner-level roles the project owner requested and gives ONE account the Super Admin role:
#   Super Admin  - every capability, organization-wide (full access)
#   Owner        - every capability, organization-wide (full access)
#   Franchise Owner - day-to-day capabilities, assigned per franchise (partial: only the franchises it is assigned to;
#                     no organization settings, access/roles management, central catalogue/documents or invoice issuer setup)
# These are the project owner's requested definitions, not a client-approved role template.
# Without -Apply it only reports what it would do (the change is rolled back). Re-running is safe: existing roles/assignments are kept.
param(
 [Parameter(Mandatory=$true)][string]$Email,
 [switch]$Apply
)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$psql=Join-Path $root '.cache/postgresql/runtime/pgsql/bin/psql.exe'
$credential=Import-Clixml -LiteralPath (Join-Path $root '.cache/postgresql/local-credential.clixml')
$ending= if($Apply){'COMMIT;'}else{'ROLLBACK;'}
$sql=@'
BEGIN;
CREATE TEMP TABLE _t ON COMMIT DROP AS
 SELECT c.principal_id AS pid, m.organization_id AS org
 FROM password_credentials c JOIN principals p ON p.id=c.principal_id AND p.active
 JOIN organization_memberships m ON m.principal_id=c.principal_id
 WHERE c.email=lower(:'email');
DO $$ BEGIN IF (SELECT count(*) FROM _t)<>1 THEN RAISE EXCEPTION 'Expected exactly one active organization membership for that email'; END IF; END $$;
CREATE TEMP TABLE _caps ON COMMIT DROP AS SELECT unnest(ARRAY[
 'customers.read','customers.create','customer_accounts.manage','vehicles.read','vehicles.create',
 'jobs.read','jobs.create','jobs.complete','jobs.contribute',
 'catalogue.read','catalogue.manage','content.manage',
 'scheduling.manage','scheduling.reserve','scheduling.release','scheduling.read',
 'attendance.record','attendance.read','attendance.manage',
 'bookings.read','bookings.create','bookings.manage',
 'invoices.read','invoices.create','invoices.issue','invoices.manage',
 'payments.record','payments.read',
 'sop.read','sop.acknowledge','sop.manage',
 'quotes.read','quotes.create','quotes.manage','quotes.issue',
 'dealership.read','dealership.manage','marketing.read','marketing.manage',
 'structure.read','structure.manage','invoices.configure','access.manage','roles.manage','roles.assign']) AS action;
CREATE TEMP TABLE _defs ON COMMIT DROP AS
 SELECT 'Super Admin' AS name, 'Full access to everything in the organization.' AS descr, array(SELECT action FROM _caps ORDER BY action) AS caps
 UNION ALL SELECT 'Owner', 'Full access to everything in the organization.', array(SELECT action FROM _caps ORDER BY action)
 UNION ALL SELECT 'Franchise Owner', 'Runs the franchises it is assigned to: bookings, customers, work, staff time, invoices, quotes and marketing. No organization-wide settings, people/roles management, central catalogue or documents.',
   array(SELECT action FROM _caps WHERE action NOT IN ('content.manage','sop.manage','structure.read','structure.manage','invoices.configure','access.manage','roles.manage','roles.assign') ORDER BY action);
WITH new_roles AS (
  INSERT INTO roles(organization_id,name,description) SELECT t.org,d.name,d.descr FROM _t t CROSS JOIN _defs d
  ON CONFLICT (organization_id,name) DO NOTHING RETURNING id,name,organization_id),
 caps AS (INSERT INTO role_capabilities(role_id,action) SELECT n.id,unnest(d.caps) FROM new_roles n JOIN _defs d ON d.name=n.name RETURNING role_id)
INSERT INTO role_definition_events(organization_id,actor_id,role_id,role_name,action,version,capabilities)
 SELECT n.organization_id,(SELECT pid FROM _t),n.id,n.name,'created',1,d.caps FROM new_roles n JOIN _defs d ON d.name=n.name;
WITH a AS (
  INSERT INTO member_role_assignments(organization_id,principal_id,role_id,scope_type,scope_id,assigned_by)
  SELECT t.org,t.pid,r.id,'organization',t.org,t.pid FROM _t t JOIN roles r ON r.organization_id=t.org AND r.name='Super Admin'
  ON CONFLICT DO NOTHING RETURNING organization_id,principal_id,role_id,scope_type,scope_id)
INSERT INTO role_assignment_events(organization_id,actor_id,principal_id,role_id,scope_type,scope_id,action,version)
 SELECT organization_id,principal_id,principal_id,role_id,scope_type,scope_id,'assigned',1 FROM a;
SELECT r.name AS role, (SELECT count(*) FROM role_capabilities rc WHERE rc.role_id=r.id) AS capabilities,
       (SELECT string_agg(coalesce(c.email,'?')||' ('||a.scope_type||')', ', ') FROM member_role_assignments a LEFT JOIN password_credentials c ON c.principal_id=a.principal_id WHERE a.role_id=r.id) AS assigned_to
FROM roles r JOIN _t t ON t.org=r.organization_id WHERE r.name IN ('Super Admin','Owner','Franchise Owner') ORDER BY r.name;
'@ + "`n$ending`n"
$previous=$env:PGPASSWORD
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 $sql | & $psql -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -v ON_ERROR_STOP=1 -v "email=$($Email.Trim())" -q -f -
 if($LASTEXITCODE -ne 0){throw 'Nothing was changed.'}
 if($Apply){Write-Output 'Applied.'}else{Write-Output 'Preview only (rolled back). Re-run with -Apply to make the change.'}
}finally{$env:PGPASSWORD=$previous}
