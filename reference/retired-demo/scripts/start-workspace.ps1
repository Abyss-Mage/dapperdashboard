$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/workspace-runtime'
$settings=Get-Content -LiteralPath (Join-Path $runtimeRoot 'settings.json') -Raw | ConvertFrom-Json
if($settings.mode -ne 'synthetic-development-only' -or $settings.schema -notmatch '^dapper_workspace_demo_[a-z0-9_]{1,40}$'){throw 'Dedicated synthetic workspace settings required.'}
foreach($port in @(8121,8443)){$probe=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,$port);try{$probe.Start()}catch{throw "Workspace port $port is occupied; no process was stopped."}finally{$probe.Stop()}}
$databaseCredential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
$keys=@('DAPPER_DATABASE_URL','DAPPER_WORKSPACE','DAPPER_CLIENT_ORIGIN','DAPPER_PASSWORD_LOGIN','DAPPER_SESSION_LIFETIME','DAPPER_HTTPS_WORKSPACE','DAPPER_TLS_CERT','DAPPER_TLS_KEY')
$previous=@{};foreach($key in $keys){$previous[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
try{
 $databasePassword=[Uri]::EscapeDataString($databaseCredential.GetNetworkCredential().Password)
 $env:DAPPER_DATABASE_URL="postgres://dapper_local:${databasePassword}@127.0.0.1:15432/postgres?sslmode=disable&search_path=$($settings.schema)"
 $env:DAPPER_WORKSPACE='1';$env:DAPPER_CLIENT_ORIGIN='https://127.0.0.1:8443';$env:DAPPER_PASSWORD_LOGIN='1';$env:DAPPER_SESSION_LIFETIME=$settings.sessionLifetime
 $env:DAPPER_HTTPS_WORKSPACE='1';$env:DAPPER_TLS_CERT=Join-Path $runtimeRoot 'tls/certificate.pem';$env:DAPPER_TLS_KEY=Join-Path $runtimeRoot 'tls/private-key.pem'
 $api=Start-Process -FilePath (Join-Path $runtimeRoot 'workspace.exe') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'api.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'api.stderr.log')
 $gateway=Start-Process -FilePath (Join-Path $runtimeRoot 'gateway.exe') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'gateway.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'gateway.stderr.log')
 @($api,$gateway)|ForEach-Object{@{id=$_.Id;started=$_.StartTime.ToUniversalTime().ToString('o');executable=$_.Path}}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $runtimeRoot 'processes.json')
}finally{foreach($key in $keys){[Environment]::SetEnvironmentVariable($key,$previous[$key],'Process')};$databasePassword=$null}
$ready=$false
for($attempt=0;$attempt -lt 20;$attempt++){
 if($api.HasExited -or $gateway.HasExited){break}
 try{$health=Invoke-RestMethod 'http://127.0.0.1:8121/api/v1/health';if($health.mode -eq 'scoped-postgres-foundation' -and $health.identityProviderConfigured){$ready=$true;break}}catch{Start-Sleep -Milliseconds 250}
}
if(!$ready){throw 'Scoped workspace startup failed. Inspect the private runtime logs; no schema migrations were run.'}
Write-Output 'Synthetic scoped workspace listening at https://127.0.0.1:8443/workspace. Certificate trust and browser sign-in require user review; no trust settings changed.'
