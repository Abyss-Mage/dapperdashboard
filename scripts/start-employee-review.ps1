param([switch]$SkipBuild,[switch]$RefreshSource)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/employee-review'
$webRoot=Join-Path $runtimeRoot 'web'
$source=Join-Path $projectRoot 'dapper-platform/apps/web'
foreach($port in @(4176,8445)){$probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,$port);try{$probe.Start()}finally{$probe.Stop()}}
New-Item -ItemType Directory -Force -Path $webRoot|Out-Null
if(!$SkipBuild){
 if((Test-Path -LiteralPath (Join-Path $webRoot 'app')) -and !$RefreshSource){throw 'Explicit -RefreshSource is required to update the isolated snapshot with both review ports stopped.'}
 foreach($name in @('app','public','package.json','tsconfig.json','next.config.ts','next-env.d.ts')){Copy-Item -LiteralPath (Join-Path $source $name) -Destination $webRoot -Recurse -Force}
 if(!(Test-Path -LiteralPath (Join-Path $webRoot 'node_modules'))){New-Item -ItemType Junction -Path (Join-Path $webRoot 'node_modules') -Target (Join-Path $source 'node_modules')|Out-Null}
 Push-Location $webRoot
 try{& npm.cmd run build;if($LASTEXITCODE -ne 0){throw 'Isolated employee frontend build failed'}}finally{Pop-Location}
}
if(!(Test-Path -LiteralPath (Join-Path $webRoot '.next/BUILD_ID'))){throw 'Verified isolated frontend build required'}
foreach($name in @('ready','finish','verified')){$marker=Join-Path $runtimeRoot $name;if(Test-Path -LiteralPath $marker){Remove-Item -LiteralPath $marker}}
$keys=@('DAPPER_TEST_DATABASE_URL','DAPPER_EMPLOYEE_BROWSER_TEST','DAPPER_EMPLOYEE_BROWSER_ROOT','DAPPER_BROWSER_CERT','DAPPER_BROWSER_KEY','GOFLAGS','GOMAXPROCS')
$previous=@{};foreach($name in $keys){$previous[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
$records=@()
try{
 $credential=Import-Clixml -LiteralPath (Join-Path $projectRoot '.cache/postgresql/local-credential.clixml')
 $privatePassword=[Uri]::EscapeDataString($credential.GetNetworkCredential().Password)
 $env:DAPPER_TEST_DATABASE_URL="postgres://dapper_local:${privatePassword}@127.0.0.1:15432/postgres?sslmode=disable"
 $env:DAPPER_EMPLOYEE_BROWSER_TEST='1';$env:DAPPER_EMPLOYEE_BROWSER_ROOT=$runtimeRoot
 $env:DAPPER_BROWSER_CERT=Join-Path $projectRoot '.cache/application-runtime/tls/certificate.pem'
 $env:DAPPER_BROWSER_KEY=Join-Path $projectRoot '.cache/application-runtime/tls/private-key.pem'
 $env:GOFLAGS='-p=1';$env:GOMAXPROCS='2'
 $nodeExecutable=(Get-Command node.exe).Source
 $frontend=Start-Process -FilePath $nodeExecutable -ArgumentList 'node_modules/next/dist/bin/next start --hostname 127.0.0.1 --port 4176' -WorkingDirectory $webRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'web.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'web.stderr.log')
 $records+=@{id=$frontend.Id;started=$frontend.StartTime.ToUniversalTime().ToString('o');executable=$nodeExecutable;kind='web'}
 $goExecutable=(Get-Command go.exe).Source
 $backend=Start-Process -FilePath $goExecutable -ArgumentList 'test -count=1 -run ^TestEmployeeBrowserHarness$ -timeout 15m ./internal/workspace' -WorkingDirectory (Join-Path $projectRoot 'dapper-platform/apps/api') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'api.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'api.stderr.log')
 $records+=@{id=$backend.Id;started=$backend.StartTime.ToUniversalTime().ToString('o');executable=$goExecutable;kind='api'}
 $records|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $runtimeRoot 'processes.json')
 for($attempt=0;$attempt -lt 1800;$attempt++){
  $backend.Refresh();$frontend.Refresh();if($backend.HasExited -or $frontend.HasExited){throw 'Isolated employee fixture failed; inspect private scoped review logs'}
  if(Test-Path -LiteralPath (Join-Path $runtimeRoot 'ready')){Write-Output 'Synthetic employee review ready at https://127.0.0.1:8445/acceptance-session. No application database writes; use finish-employee-review.ps1 after browser acceptance.';return}
  Start-Sleep -Milliseconds 250
 }
 throw 'Isolated employee review did not become ready; inspect scoped logs'
} catch {
 New-Item -ItemType File -Force -Path (Join-Path $runtimeRoot 'finish')|Out-Null
 if($backend -and !$backend.HasExited){$null=$backend.WaitForExit(20000)}
 foreach($record in $records){
  $process=Get-Process -Id $record.id -ErrorAction SilentlyContinue
  if(!$process){continue}
  $expected=[DateTimeOffset]::Parse([string]$record.started).UtcDateTime
  if($process.Path -eq $record.executable -and $process.StartTime.ToUniversalTime().Ticks -eq $expected.Ticks){& taskkill.exe /PID $process.Id /T /F|Out-Null}
 }
 throw
} finally {
 foreach($name in $keys){[Environment]::SetEnvironmentVariable($name,$previous[$name],'Process')}
 $privatePassword=$null;$credential=$null
}
