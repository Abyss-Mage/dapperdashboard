$ErrorActionPreference='Stop'
$runtimeRoot=Join-Path (Split-Path $PSScriptRoot -Parent) '.cache/employee-review'
$records=Get-Content -LiteralPath (Join-Path $runtimeRoot 'processes.json') -Raw|ConvertFrom-Json
New-Item -ItemType File -Force -Path (Join-Path $runtimeRoot 'finish')|Out-Null
$api=$records|Where-Object{$_.kind -eq 'api'}
$test=Get-Process -Id $api.id -ErrorAction SilentlyContinue
if($test -and !$test.WaitForExit(20000)){throw 'Isolated employee persistence verification is still running'}
foreach($record in @($records|Where-Object{$_.kind -eq 'web'})){
 $process=Get-Process -Id $record.id -ErrorAction SilentlyContinue
 if(!$process){continue}
 $expected=if($record.started -is [DateTime]){$record.started.ToUniversalTime()}else{[DateTimeOffset]::Parse([string]$record.started).UtcDateTime}
 if($process.Path -ne $record.executable -or $process.StartTime.ToUniversalTime().Ticks -ne $expected.Ticks){throw 'Review process identity changed; refusing to stop'}
 & taskkill.exe /PID $process.Id /T /F|Out-Null
 if($LASTEXITCODE -ne 0){throw 'Could not stop isolated employee frontend'}
}
if(!(Test-Path -LiteralPath (Join-Path $runtimeRoot 'verified'))){throw 'Employee browser persistence acceptance did not pass; inspect isolated logs'}
$lines=@(Get-Content -LiteralPath (Join-Path $runtimeRoot 'api.stdout.log')|Where-Object{$_ -match '\S'})
if(!$lines.Count -or $lines[-1] -notmatch '^ok\s+dapper\.local/platform/internal/workspace\s'){throw 'Go fixture did not report terminal test success'}
Get-Content -LiteralPath (Join-Path $runtimeRoot 'verified')
Write-Output 'Isolated employee fixture terminated and disposable schema cleanup completed.'
