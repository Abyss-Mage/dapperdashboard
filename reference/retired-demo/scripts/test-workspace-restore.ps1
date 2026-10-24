$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/workspace-runtime'
$settings=Get-Content -LiteralPath (Join-Path $runtimeRoot 'settings.json') -Raw|ConvertFrom-Json
$sourceSchema=$settings.schema
if($settings.mode -ne 'synthetic-development-only' -or $sourceSchema -notmatch '^dapper_workspace_demo_[a-z0-9_]{1,40}$'){throw 'Only the dedicated synthetic schema can be exercised.'}
$binaryRoot=Join-Path $projectRoot '.cache/postgresql/runtime/pgsql/bin'
$backupRoot=Join-Path $runtimeRoot 'backups'
New-Item -ItemType Directory -Force -Path $backupRoot|Out-Null
$runId=[DateTime]::UtcNow.Ticks.ToString()
$plainPath=Join-Path $backupRoot "$runId-source.dump"
$restorePath=Join-Path $backupRoot "$runId-restore.dump"
$encryptedPath=Join-Path $backupRoot "$runId.dump.dpapi"
$restoreDatabase='dapper_restore_test_'+$runId
if($restoreDatabase -notmatch '^dapper_restore_test_[0-9]+$'){throw 'Invalid isolated restore database name.'}
$created=$false
$previousPassword=$env:PGPASSWORD
$credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 # Discover every ordinary table in this validated, dedicated synthetic schema;
 # newly migrated tables must not silently escape the restore count check.
 $tables=@(& (Join-Path $binaryRoot 'psql.exe') -X --host=127.0.0.1 --port=15432 --username=dapper_local --dbname=postgres -A -t -v ON_ERROR_STOP=1 -c "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname='$sourceSchema' ORDER BY tablename")
 if($LASTEXITCODE -ne 0 -or $tables.Count -eq 0){throw 'Synthetic source table inventory unavailable.'}
 foreach($table in $tables){if($table -notmatch '^[a-z][a-z0-9_]*$'){throw 'Unexpected synthetic table identifier.'}}
 $countExpressions=$tables|ForEach-Object{'(SELECT count(*) FROM "'+$sourceSchema+'"."'+$_+'")'}
 $countSQL='SELECT '+($countExpressions -join ',')
 & (Join-Path $binaryRoot 'pg_dump.exe') --host=127.0.0.1 --port=15432 --username=dapper_local --dbname=postgres --schema=$sourceSchema --format=custom --file=$plainPath
 if($LASTEXITCODE -ne 0){throw 'Synthetic schema backup failed.'}
 $sourceCounts=& (Join-Path $binaryRoot 'psql.exe') -X --host=127.0.0.1 --port=15432 --username=dapper_local --dbname=postgres -A -t -v ON_ERROR_STOP=1 -c $countSQL
 if($LASTEXITCODE -ne 0){throw 'Source verification counts unavailable.'}
 $plainBytes=[IO.File]::ReadAllBytes($plainPath)
 $sourceDigest=(Get-FileHash -LiteralPath $plainPath -Algorithm SHA256).Hash
 $encrypted=[Security.Cryptography.ProtectedData]::Protect($plainBytes,$null,[Security.Cryptography.DataProtectionScope]::CurrentUser)
 [IO.File]::WriteAllBytes($encryptedPath,$encrypted)
 [Array]::Clear($plainBytes,0,$plainBytes.Length)
 Remove-Item -LiteralPath $plainPath
 $restoredBytes=[Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes($encryptedPath),$null,[Security.Cryptography.DataProtectionScope]::CurrentUser)
 [IO.File]::WriteAllBytes($restorePath,$restoredBytes)
 [Array]::Clear($restoredBytes,0,$restoredBytes.Length)
 if((Get-FileHash -LiteralPath $restorePath -Algorithm SHA256).Hash -ne $sourceDigest){throw 'Encrypted backup round-trip digest mismatch.'}
 & (Join-Path $binaryRoot 'createdb.exe') --host=127.0.0.1 --port=15432 --username=dapper_local --maintenance-db=postgres $restoreDatabase
 if($LASTEXITCODE -ne 0){throw 'Could not create fresh restore test database.'};$created=$true
 & (Join-Path $binaryRoot 'pg_restore.exe') --host=127.0.0.1 --port=15432 --username=dapper_local --dbname=$restoreDatabase --no-owner --no-privileges --exit-on-error $restorePath
 if($LASTEXITCODE -ne 0){throw 'Restore into isolated test database failed.'}
 $restoreCounts=& (Join-Path $binaryRoot 'psql.exe') -X --host=127.0.0.1 --port=15432 --username=dapper_local --dbname=$restoreDatabase -A -t -v ON_ERROR_STOP=1 -c $countSQL
 if($LASTEXITCODE -ne 0 -or $sourceCounts -ne $restoreCounts){throw 'Restored row counts differ from the synthetic source.'}
 @{verifiedAt=[DateTime]::UtcNow.ToString('o');mode='synthetic-development-only';result='pass';tables=$tables;rowCounts=$restoreCounts;backupEncryption='Windows DPAPI CurrentUser';encryptedBackupSha256=(Get-FileHash -LiteralPath $encryptedPath -Algorithm SHA256).Hash;scope='one dedicated synthetic schema';productionRestoreVerified=$false}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $projectRoot 'output/acceptance/synthetic-restore-verification.json')
 Write-Output 'PASS: native encrypted synthetic backup restored into a fresh isolated database; all selected table counts match. Production/off-site recovery is not qualified.'
}finally{
 if($created){& (Join-Path $binaryRoot 'dropdb.exe') --host=127.0.0.1 --port=15432 --username=dapper_local --maintenance-db=postgres $restoreDatabase;if($LASTEXITCODE -ne 0){Write-Warning 'Fresh restore test database could not be removed; inspect the synthetic test database.'}}
 foreach($temporary in @($plainPath,$restorePath)){$resolved=[IO.Path]::GetFullPath($temporary);$expectedRoot=[IO.Path]::GetFullPath($backupRoot)+[IO.Path]::DirectorySeparatorChar;if(!$resolved.StartsWith($expectedRoot,[StringComparison]::OrdinalIgnoreCase)){throw 'Temporary backup path escaped its workspace.'};if(Test-Path -LiteralPath $resolved){Remove-Item -LiteralPath $resolved}}
 $env:PGPASSWORD=$previousPassword;$credential=$null
}
