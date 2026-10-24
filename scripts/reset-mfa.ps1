# Emergency use by the machine owner only: removes authenticator-app MFA and recovery codes for ONE account so the
# person can sign in with their password and set MFA up again (administrators are forced to set it up at next sign-in).
# Without -Apply it only reports what it would do. Nothing else about the account changes.
param(
 [Parameter(Mandatory=$true)][string]$Email,
 [switch]$Apply
)
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$psql=Join-Path $root '.cache/postgresql/runtime/pgsql/bin/psql.exe'
$credential=Import-Clixml -LiteralPath (Join-Path $root '.cache/postgresql/local-credential.clixml')
$previous=$env:PGPASSWORD
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 $emailSql=$Email.Trim().ToLower().Replace("'","''")
 $found=& $psql -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -qAt -v ON_ERROR_STOP=1 -c "SELECT c.principal_id::text||' | MFA '||CASE WHEN m.enabled_at IS NULL THEN 'not enabled' ELSE 'enabled' END FROM password_credentials c LEFT JOIN member_mfa m ON m.principal_id=c.principal_id WHERE c.email='$emailSql'"
 if(!$found){throw 'No account with that email.'}
 Write-Output "Account: $found"
 if(!$Apply){Write-Output 'Preview only. Re-run with -Apply to remove this account''s MFA and recovery codes.';return}
 $sql="BEGIN; DELETE FROM member_mfa_recovery_codes WHERE principal_id=(SELECT principal_id FROM password_credentials WHERE email='$emailSql'); DELETE FROM member_mfa WHERE principal_id=(SELECT principal_id FROM password_credentials WHERE email='$emailSql'); INSERT INTO member_security_events(principal_id,action) SELECT principal_id,'mfa_disabled' FROM password_credentials WHERE email='$emailSql'; COMMIT;"
 $sql|& $psql -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -v ON_ERROR_STOP=1 -q -f -
 if($LASTEXITCODE -ne 0){throw 'Reset failed; nothing was changed.'}
 Write-Output 'MFA removed for that account. They will be asked to set it up again at next sign-in.'
}finally{$env:PGPASSWORD=$previous}
