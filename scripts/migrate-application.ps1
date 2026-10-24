param([Parameter(Mandatory=$true)][switch]$Apply)
$ErrorActionPreference='Stop'
if(!$Apply){throw 'Explicit -Apply is required.'}
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/application-runtime'
$settings=Get-Content -LiteralPath (Join-Path $runtimeRoot 'settings.json') -Raw|ConvertFrom-Json
if($settings.mode -ne 'application' -or $settings.database -ne 'dapper_application'){throw 'Application database selector mismatch.'}
$probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,8121)
try{$probe.Start()}catch{throw 'Stop the recorded application before applying a reviewed migration.'}finally{$probe.Stop()}
$binaryRoot=Join-Path $projectRoot '.cache/postgresql/runtime/pgsql/bin'
$backupRoot=Join-Path $runtimeRoot 'backups'
New-Item -ItemType Directory -Force -Path $backupRoot|Out-Null
$runId=[DateTime]::UtcNow.Ticks.ToString()
$plain=Join-Path $backupRoot "$runId-source.dump"
$restoredDump=Join-Path $backupRoot "$runId-restored.dump"
$encrypted=Join-Path $backupRoot "$runId.dump.dpapi"
$restoreDatabase='dapper_application_restore_'+$runId
if($restoreDatabase -notmatch '^dapper_application_restore_[0-9]+$'){throw 'Invalid isolated restore selector.'}
$created=$false
$credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
$previousPassword=$env:PGPASSWORD;$previousDatabase=$env:DAPPER_DATABASE_URL
$countSQL=@'
CREATE FUNCTION pg_temp.application_counts() RETURNS jsonb LANGUAGE plpgsql AS $$
DECLARE item record; row_count bigint; result jsonb := '{}'::jsonb;
BEGIN
 FOR item IN SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename LOOP
  EXECUTE format('SELECT count(*) FROM public.%I',item.tablename) INTO row_count;
  result := result || jsonb_build_object(item.tablename,row_count);
 END LOOP;
 RETURN result;
END $$;
SELECT pg_temp.application_counts();
'@
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 $sourceCounts=& (Join-Path $binaryRoot 'psql.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -qAt -v ON_ERROR_STOP=1 -c $countSQL
 if($LASTEXITCODE -ne 0){throw 'Source inventory failed.'}
 & (Join-Path $binaryRoot 'pg_dump.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d dapper_application -Fc -f $plain
 if($LASTEXITCODE -ne 0){throw 'Pre-migration backup failed.'}
 $protected=[Security.Cryptography.ProtectedData]::Protect([IO.File]::ReadAllBytes($plain),$null,[Security.Cryptography.DataProtectionScope]::CurrentUser)
 [IO.File]::WriteAllBytes($encrypted,$protected)
 $decrypted=[Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes($encrypted),$null,[Security.Cryptography.DataProtectionScope]::CurrentUser)
 [IO.File]::WriteAllBytes($restoredDump,$decrypted)
 [Array]::Clear($decrypted,0,$decrypted.Length);$decrypted=$null
 & (Join-Path $binaryRoot 'createdb.exe') -h 127.0.0.1 -p 15432 -U dapper_local $restoreDatabase
 if($LASTEXITCODE -ne 0){throw 'Isolated restore database creation failed.'};$created=$true
 & (Join-Path $binaryRoot 'pg_restore.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d $restoreDatabase --exit-on-error $restoredDump
 if($LASTEXITCODE -ne 0){throw 'Encrypted backup restore failed; application migrations not applied.'}
 $restoreCounts=& (Join-Path $binaryRoot 'psql.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d $restoreDatabase -qAt -v ON_ERROR_STOP=1 -c $countSQL
 if($LASTEXITCODE -ne 0 -or ($sourceCounts -join '') -ne ($restoreCounts -join '')){throw 'Restored table counts differ; application migrations not applied.'}
 Push-Location (Join-Path $projectRoot 'dapper-platform/apps/api')
 try{& go.exe build -o (Join-Path $runtimeRoot 'database.exe') ./cmd/database;if($LASTEXITCODE -ne 0){throw 'Migration executable build failed'}}finally{Pop-Location}
 $password=[Uri]::EscapeDataString($env:PGPASSWORD)
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${password}@127.0.0.1:15432/dapper_application?sslmode=disable"
 & (Join-Path $runtimeRoot 'database.exe') -migrate
 if($LASTEXITCODE -ne 0){throw 'Reviewed migration failed; encrypted backup preserved. No automatic rollback attempted.'}
 @{timestamp=[DateTime]::UtcNow.ToString('o');database='dapper_application';backup=[IO.Path]::GetFileName($encrypted);restore='PASS';verification='all public table counts; no row-content equality claim';productionOffSiteRecovery='NOT_QUALIFIED'}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $projectRoot 'output/acceptance/application-migration-verification.json')
 Write-Output 'Reviewed migrations applied after an encrypted backup and isolated restore with all table counts matching. Application is stopped; restart explicitly.'
}finally{
 if($created){& (Join-Path $binaryRoot 'dropdb.exe') -h 127.0.0.1 -p 15432 -U dapper_local $restoreDatabase;if($LASTEXITCODE -ne 0){Write-Warning 'Isolated restore database cleanup failed; inspect before retrying.'}}
 foreach($path in @($plain,$restoredDump)){if(Test-Path -LiteralPath $path){Remove-Item -LiteralPath $path}}
 $env:PGPASSWORD=$previousPassword;$env:DAPPER_DATABASE_URL=$previousDatabase
 $password=$null;$credential=$null;$decrypted=$null
}
