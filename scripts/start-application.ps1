param(
 [Parameter(Mandatory=$true)][ValidatePattern('^([1-9]|1[0-9]|2[0-4])h$')][string]$SessionLifetime,
 [switch]$InitializeDatabase,
 [string]$CertificateDirectory
)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/application-runtime'
$apiRoot=Join-Path $projectRoot 'dapper-platform/apps/api'
$webRoot=Join-Path $projectRoot 'dapper-platform/apps/web'
$settingsPath=Join-Path $runtimeRoot 'settings.json'
$databaseName='dapper_application'
$binaryRoot=Join-Path $projectRoot '.cache/postgresql/runtime/pgsql/bin'
foreach($port in @(3006,8121,8443)){
 $probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,$port)
 try{$probe.Start()}finally{$probe.Stop()}
}
if(!(Test-Path -LiteralPath (Join-Path $webRoot '.next/BUILD_ID'))){throw 'Build the application frontend first.'}
if(!$InitializeDatabase -and !(Test-Path -LiteralPath $settingsPath)){throw 'First startup requires explicit -InitializeDatabase for a separate unseeded database.'}
if($InitializeDatabase -and (Test-Path -LiteralPath $settingsPath)){throw 'Application database is already initialized. Existing data will not be replaced.'}
New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
Push-Location $apiRoot
try{
 foreach($target in @('workspace','gateway','database')){
  & go.exe build -o (Join-Path $runtimeRoot "$target.exe") "./cmd/$target"
  if($LASTEXITCODE -ne 0){throw 'Application executable build failed.'}
 }
}finally{Pop-Location}
$credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
$keys=@('PGPASSWORD','DAPPER_DATABASE_URL','DAPPER_WORKSPACE','DAPPER_CLIENT_ORIGIN','DAPPER_PASSWORD_LOGIN','DAPPER_SESSION_LIFETIME','DAPPER_HTTPS_WORKSPACE','DAPPER_TLS_CERT','DAPPER_TLS_KEY','DAPPER_UPLOAD_DIR','DAPPER_SOP_DIR')
$previous=@{};foreach($key in $keys){$previous[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
$started=@()
try{
 $env:PGPASSWORD=$credential.GetNetworkCredential().Password
 $databasePassword=[Uri]::EscapeDataString($env:PGPASSWORD)
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${databasePassword}@127.0.0.1:15432/${databaseName}?sslmode=disable"
 if($InitializeDatabase){
  $exists=& (Join-Path $binaryRoot 'psql.exe') -h 127.0.0.1 -p 15432 -U dapper_local -d postgres -tAc "SELECT count(*) FROM pg_database WHERE datname='dapper_application'"
  if($LASTEXITCODE -ne 0 -or $exists.Trim() -ne '0'){throw 'Database already exists or database inventory failed; initialization refused.'}
  & (Join-Path $binaryRoot 'createdb.exe') -h 127.0.0.1 -p 15432 -U dapper_local $databaseName
  if($LASTEXITCODE -ne 0){throw 'Application database creation failed.'}
  & (Join-Path $runtimeRoot 'database.exe') -migrate
  if($LASTEXITCODE -ne 0){throw 'Application migration failed; no fixtures or accounts were created.'}
  @{mode='application';database=$databaseName;sessionLifetime=$SessionLifetime}|ConvertTo-Json|Set-Content -LiteralPath $settingsPath
 }else{
  $settings=Get-Content -LiteralPath $settingsPath -Raw|ConvertFrom-Json
  if($settings.mode -ne 'application' -or $settings.database -ne $databaseName){throw 'Application settings mismatch; refusing alternate database.'}
 }
 # Reuse the existing local certificate for the same host, without changing trust.
 $tlsRoot=Join-Path $runtimeRoot 'tls'
 if(!(Test-Path -LiteralPath (Join-Path $tlsRoot 'private-key.pem'))){
  New-Item -ItemType Directory -Force -Path $tlsRoot|Out-Null
  if(!$CertificateDirectory){throw 'Provide -CertificateDirectory with existing certificate.pem/private-key.pem; certificate trust is never changed automatically.'}
  foreach($file in @('certificate.pem','private-key.pem')){Copy-Item -LiteralPath (Join-Path $CertificateDirectory $file) -Destination (Join-Path $tlsRoot $file)}
 }
 $env:PGPASSWORD=$null
 $env:DAPPER_WORKSPACE='1';$env:DAPPER_CLIENT_ORIGIN='https://127.0.0.1:8443';$env:DAPPER_PASSWORD_LOGIN='1';$env:DAPPER_SESSION_LIFETIME=$SessionLifetime
 $env:DAPPER_HTTPS_WORKSPACE='1';$env:DAPPER_TLS_CERT=Join-Path $tlsRoot 'certificate.pem';$env:DAPPER_TLS_KEY=Join-Path $tlsRoot 'private-key.pem'
 $env:DAPPER_UPLOAD_DIR=Join-Path $runtimeRoot 'uploads'
 $env:DAPPER_SOP_DIR=Join-Path $projectRoot 'dapper-platform/apps/api/data/sops'
 $webExecutable=(Get-Command node.exe).Source
 $apiExecutable=Join-Path $runtimeRoot 'workspace.exe'
 $gatewayExecutable=Join-Path $runtimeRoot 'gateway.exe'
 # Optional Square sandbox credential: decrypted only for the API process, never written anywhere.
 # Server-held keys, generated once and protected for this Windows user: authenticator-secret encryption (MFA) and
 # invitation payload encryption. Never written to source, the vault or logs. Passed to the API process only.
 $keysPath=Join-Path $projectRoot '.cache/secrets/app-keys.clixml'
 if(!(Test-Path -LiteralPath $keysPath)){
  New-Item -ItemType Directory -Force -Path (Split-Path $keysPath)|Out-Null
  $makeKey={$b=[byte[]]::new(32);[Security.Cryptography.RandomNumberGenerator]::Fill($b);ConvertTo-SecureString ([Convert]::ToBase64String($b)) -AsPlainText -Force}
  [pscustomobject]@{mfa=(& $makeKey);invitation=(& $makeKey)}|Export-Clixml -LiteralPath $keysPath
 }
 $appKeys=Import-Clixml -LiteralPath $keysPath
 $env:DAPPER_MFA_KEY=[Net.NetworkCredential]::new('',$appKeys.mfa).Password
 $env:DAPPER_INVITATION_ENCRYPTION_KEY=[Net.NetworkCredential]::new('',$appKeys.invitation).Password
 # Optional outgoing mail (created by scripts/set-smtp-config.ps1). Without it invitations stay queued and nothing is sent.
 $smtpPath=Join-Path $projectRoot '.cache/secrets/smtp.clixml'
 if(Test-Path -LiteralPath $smtpPath){$smtp=Import-Clixml -LiteralPath $smtpPath;$env:DAPPER_SMTP_HOST=$smtp.host;$env:DAPPER_SMTP_PORT=[string]$smtp.port;$env:DAPPER_SMTP_USERNAME=$smtp.username;$env:DAPPER_SMTP_PASSWORD=[Net.NetworkCredential]::new('',$smtp.password).Password;$env:DAPPER_SMTP_FROM_ADDRESS=$smtp.fromAddress;$env:DAPPER_SMTP_FROM_NAME=$smtp.fromName}
 $squareConfig=Join-Path $projectRoot '.cache/square-sandbox/config.clixml'
 if(Test-Path -LiteralPath $squareConfig){$squareSaved=Import-Clixml -LiteralPath $squareConfig;$env:DAPPER_SQUARE_SANDBOX_TOKEN=[Net.NetworkCredential]::new('',$squareSaved.token).Password;if($squareSaved.PSObject.Properties['applicationId'] -and $squareSaved.applicationId){$env:DAPPER_SQUARE_SANDBOX_APPLICATION_ID=$squareSaved.applicationId}}
 $api=Start-Process -FilePath $apiExecutable -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'api.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'api.stderr.log')
 $env:DAPPER_SQUARE_SANDBOX_TOKEN=$null;$env:DAPPER_SQUARE_SANDBOX_APPLICATION_ID=$null
 foreach($secretName in 'DAPPER_MFA_KEY','DAPPER_INVITATION_ENCRYPTION_KEY','DAPPER_SMTP_HOST','DAPPER_SMTP_PORT','DAPPER_SMTP_USERNAME','DAPPER_SMTP_PASSWORD','DAPPER_SMTP_FROM_ADDRESS','DAPPER_SMTP_FROM_NAME'){[Environment]::SetEnvironmentVariable($secretName,$null,'Process')}
 $started+=@{process=$api;executable=$apiExecutable}
 $web=Start-Process -FilePath $webExecutable -ArgumentList 'node_modules/next/dist/bin/next start --hostname 127.0.0.1 --port 3006' -WorkingDirectory $webRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'web.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'web.stderr.log')
 $started+=@{process=$web;executable=$webExecutable}
 $gateway=Start-Process -FilePath $gatewayExecutable -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'gateway.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'gateway.stderr.log')
 $started+=@{process=$gateway;executable=$gatewayExecutable}
 $started|ForEach-Object{@{id=$_.process.Id;started=$_.process.StartTime.ToUniversalTime().ToString('o');executable=$_.executable}}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $runtimeRoot 'processes.json')
 $ready=$false
 for($attempt=0;$attempt -lt 30;$attempt++){
  if(@($started|Where-Object {$_.process.HasExited}).Count){break}
  try{$health=Invoke-RestMethod 'http://127.0.0.1:8121/api/v1/health';$page=Invoke-WebRequest 'http://127.0.0.1:3006/account' -UseBasicParsing;if($health.identityProviderConfigured -and $page.StatusCode -eq 200){$ready=$true;break}}catch{Start-Sleep -Milliseconds 250}
 }
 if(!$ready){throw 'Application startup failed. Inspect application-runtime logs.'}
 Write-Output 'Dapper application ready at https://127.0.0.1:8443/workspace. Separate PostgreSQL database; no fixture data, default accounts or provider operations.'
}catch{
 foreach($entry in $started){if(!$entry.process.HasExited){Stop-Process -Id $entry.process.Id}}
 throw
}finally{
 foreach($key in $keys){[Environment]::SetEnvironmentVariable($key,$previous[$key],'Process')}
 $databasePassword=$null;$credential=$null
}
