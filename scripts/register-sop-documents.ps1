# Publishes the five supplied SOP / onboarding PDFs to ONE organization so employees can read and acknowledge them.
# Publishing a document makes it company policy for those employees, so this is an administrator decision:
# without -Apply it only checks the files against their expected SHA-256 and prints what it would register.
param(
 [Parameter(Mandatory=$true)][ValidatePattern('^[0-9a-fA-F-]{36}$')][string]$OrganizationId,
 [switch]$Apply
)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$sopRoot=Join-Path $root 'dapper-platform/apps/api/data/sops'
$documents=@(
 @{title='Employee Onboarding & Safety Policy';category='onboarding';file='onboarding_policy.pdf';pages=6;sha='3bd0ccbf8a3834d9e5abdccb3911f98c6d3f4fe07f7848eb428a5de1e7ba2999'},
 @{title='Interior Detail SOP';category='procedure';file='interior_detail_sop.pdf';pages=7;sha='db5732a72dba4bfcb5455a846c30467a2c06910abf4e601a7d90efc9d377d587'},
 @{title='Full Body Detail SOP';category='procedure';file='full_body_detail_sop.pdf';pages=6;sha='dbe16870f4a2d6ffe3e0169ed25b65484cf7edd4e099e0d27e38a8fe5d4fd70d'},
 @{title='Ceramic Coating SOP';category='procedure';file='ceramic_coating_sop.pdf';pages=5;sha='776a28a2e444c8fbb2850279a4488570968d0cac111488b399e04e93374102e5'},
 @{title='Add-on Services SOP';category='procedure';file='addon_services_sop.pdf';pages=6;sha='5208b6c0bbfcf9ba9df1efa5e40646a04e048915f1c84bd678fe26d739a9074b'}
)
foreach($d in $documents){
 $path=Join-Path $sopRoot $d.file
 if(!(Test-Path -LiteralPath $path)){throw "Missing file: $($d.file)"}
 $actual=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLower()
 if($actual -ne $d.sha){throw "Checksum mismatch for $($d.file); refusing to register."}
 Write-Output "OK  $($d.title)  ($($d.file), $($d.pages) pages)"
}
if(!$Apply){Write-Output 'Preview only. Re-run with -Apply to register these documents for the organization.';return}
$binaryRoot=Join-Path $root '.cache/postgresql/runtime/pgsql/bin'
$credential=Import-Clixml -LiteralPath (Join-Path $root '.cache/postgresql/local-credential.clixml')
$previous=$env:PGPASSWORD
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 $exists=& (Join-Path $binaryRoot 'psql.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -qAt -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM organizations WHERE id='$OrganizationId'"
 if($LASTEXITCODE -ne 0 -or $exists.Trim() -ne '1'){throw 'Organization not found; nothing registered.'}
 $sql=($documents|ForEach-Object{"INSERT INTO sop_documents(organization_id,title,category,file_name,sha256_hash,version,page_count) VALUES('$OrganizationId','$($_.title.Replace("'","''"))','$($_.category)','$($_.file)','$($_.sha)',1,$($_.pages)) ON CONFLICT (organization_id,file_name,version) DO NOTHING;"}) -join "`n"
 $sql|& (Join-Path $binaryRoot 'psql.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -v ON_ERROR_STOP=1 -1 -f -
 if($LASTEXITCODE -ne 0){throw 'Registration failed; nothing was committed.'}
 Write-Output 'Registered. Employees will see these documents under Home > Documents once the application is running with DAPPER_SOP_DIR (start-application.ps1 sets it).'
}finally{$env:PGPASSWORD=$previous}
