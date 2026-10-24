param([ValidateSet('Invoice','CustomerAccounts','BookingCapacity','BookingCancellation','BookingReschedule','BookingOffers','CustomerAvailability','CustomerRequest','OperationalJourney')][string]$Suite='Invoice')
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/invoice-browser-acceptance'
$webRoot=Join-Path $projectRoot 'dapper-platform/tests/invoice-browser'
$apiRoot=Join-Path $projectRoot 'dapper-platform/apps/api'
foreach($port in @(3007,8444)){$probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,$port);try{$probe.Start()}finally{$probe.Stop()}}
New-Item -ItemType Directory -Force -Path $runtimeRoot|Out-Null
foreach($name in @('ready','finish','verified')){$marker=Join-Path $runtimeRoot $name;if(Test-Path -LiteralPath $marker){Remove-Item -LiteralPath $marker}}
$modules=Join-Path $webRoot 'node_modules'
if(!(Test-Path -LiteralPath $modules)){New-Item -ItemType Junction -Path $modules -Target (Join-Path $projectRoot 'dapper-platform/apps/web/node_modules')|Out-Null}
$keys=@('DAPPER_TEST_DATABASE_URL','DAPPER_INVOICE_BROWSER_TEST','DAPPER_BROWSER_ROOT','DAPPER_BROWSER_CERT','DAPPER_BROWSER_KEY','DAPPER_BROWSER_SUITE')
$previous=@{};foreach($name in $keys){$previous[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
$started=@()
try {
 $credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
 $privatePassword=[Uri]::EscapeDataString($credential.GetNetworkCredential().Password)
 $env:DAPPER_TEST_DATABASE_URL="postgres://dapper_local:${privatePassword}@127.0.0.1:15432/postgres?sslmode=disable"
 $env:DAPPER_INVOICE_BROWSER_TEST='1';$env:DAPPER_BROWSER_ROOT=$runtimeRoot
 $env:DAPPER_BROWSER_SUITE=switch($Suite){'CustomerAccounts'{'customer-accounts'}'BookingCapacity'{'booking-capacity'}'BookingCancellation'{'booking-cancellation'}'BookingReschedule'{'booking-reschedule'}'BookingOffers'{'booking-offers'}'CustomerAvailability'{'customer-availability'}'CustomerRequest'{'customer-request'}'OperationalJourney'{'operational-journey'}default{'invoice'}}
 $env:DAPPER_BROWSER_CERT=Join-Path $projectRoot '.cache/application-runtime/tls/certificate.pem'
 $env:DAPPER_BROWSER_KEY=Join-Path $projectRoot '.cache/application-runtime/tls/private-key.pem'
 $goExecutable=(Get-Command go.exe).Source
 $backend=Start-Process -FilePath $goExecutable -ArgumentList 'test -count=1 -run ^TestInvoiceBrowserHarness$ -timeout 16m ./internal/workspace' -WorkingDirectory $apiRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'api.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'api.stderr.log')
 $started+=@{process=$backend;executable=$goExecutable}
 $nodeExecutable=(Get-Command node.exe).Source
 $frontend=Start-Process -FilePath $nodeExecutable -ArgumentList 'node_modules/next/dist/bin/next dev --hostname 127.0.0.1 --port 3007' -WorkingDirectory $webRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'web.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'web.stderr.log')
 $started+=@{process=$frontend;executable=$nodeExecutable}
 $started|ForEach-Object{@{id=$_.process.Id;started=$_.process.StartTime.ToUniversalTime().ToString('o');executable=$_.executable}}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $runtimeRoot 'processes.json')
 for($attempt=0;$attempt -lt 100;$attempt++){
  if(@($started|Where-Object{$_.process.HasExited}).Count){throw 'Isolated fixture exited; inspect scoped acceptance logs.'}
  if(Test-Path -LiteralPath (Join-Path $runtimeRoot 'ready')){Write-Output 'Isolated browser fixture ready: https://127.0.0.1:8444/acceptance-session. Synthetic schema only, no application database writes. Signal .cache/invoice-browser-acceptance/finish to verify persisted state and clean schema.';return}
  Start-Sleep -Milliseconds 250
 }
 throw 'Isolated browser fixture did not become ready.'
} catch {
 New-Item -ItemType File -Force -Path (Join-Path $runtimeRoot 'finish')|Out-Null
 foreach($entry in $started){if(!$entry.process.HasExited -and $entry.process.Id -eq $frontend.Id){Stop-Process -Id $entry.process.Id}}
 throw
} finally {
 foreach($name in $keys){[Environment]::SetEnvironmentVariable($name,$previous[$name],'Process')}
 $privatePassword=$null;$credential=$null
}
