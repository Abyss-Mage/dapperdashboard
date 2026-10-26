param([switch]$Enable,[switch]$Disable)
$ErrorActionPreference='Stop'
if($Enable.IsPresent -eq $Disable.IsPresent){throw 'Choose exactly one explicit synthetic preview action: -Enable or -Disable.'}
$projectRoot=Split-Path $PSScriptRoot -Parent
$settings=Get-Content -Raw -LiteralPath (Join-Path $projectRoot '.cache/workspace-runtime/settings.json')|ConvertFrom-Json
if($settings.mode -ne 'synthetic-development-only' -or $settings.schema -notmatch '^dapper_workspace_demo_[a-z0-9_]{1,40}$'){throw 'Dedicated synthetic workspace required.'}
foreach($id in @($settings.principalId,$settings.organizationId,$settings.locationId)){if($id -notmatch '^[a-fA-F0-9]{8}(-[a-fA-F0-9]{4}){3}-[a-fA-F0-9]{12}$'){throw 'Invalid synthetic selector.'}}
$sql=@'
BEGIN;
SET LOCAL search_path TO "@schema@";
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM principals p JOIN location_grants g ON g.principal_id=p.id
 JOIN organizations o ON o.id=g.organization_id JOIN locations l ON l.organization_id=o.id AND l.id=g.location_id
 WHERE p.id='@actor@' AND p.external_subject='synthetic:workspace-operator' AND p.active
 AND o.id='@org@' AND o.name='Synthetic Dapper Operations' AND l.id='@location@'
 AND l.name='Synthetic Training Location' AND g.action='customers.read') THEN
 RAISE EXCEPTION 'Exact synthetic fixture identity and scope required'; END IF;
END $$;
@mutation@
INSERT INTO audit_events(organization_id,location_id,actor_id,action,entity_id)
VALUES('@org@','@location@','@actor@','@action@','@actor@');
COMMIT;
'@
if($Enable){$mutation="INSERT INTO location_grants(principal_id,organization_id,location_id,action) VALUES('@actor@','@org@','@location@','scheduling.manage') ON CONFLICT DO NOTHING;";$action='synthetic.scheduling_preview.enabled'}else{$mutation="DELETE FROM location_grants WHERE principal_id='@actor@' AND organization_id='@org@' AND location_id='@location@' AND action='scheduling.manage';";$action='synthetic.scheduling_preview.disabled'}
$sql=$sql.Replace('@mutation@',$mutation).Replace('@schema@',$settings.schema).Replace('@actor@',$settings.principalId).Replace('@org@',$settings.organizationId).Replace('@location@',$settings.locationId).Replace('@action@',$action)
$credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
$previousPassword=$env:PGPASSWORD
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 $sql|& (Join-Path $projectRoot '.cache/postgresql/runtime/pgsql/bin/psql.exe') -X -q --host=127.0.0.1 --port=15432 --username=dapper_local --dbname=postgres -v ON_ERROR_STOP=1 --file=-
 if($LASTEXITCODE-ne 0){throw 'Synthetic scheduling permission change failed.'}
}finally{$env:PGPASSWORD=$previousPassword;$credential=$null;$sql=$null}
Write-Output "Synthetic scheduling editor permission updated ($action). No reserve/release permission or production role was granted."
