$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/invoice-browser-acceptance'
$records=Get-Content -LiteralPath (Join-Path $runtimeRoot 'processes.json') -Raw|ConvertFrom-Json
New-Item -ItemType File -Force -Path (Join-Path $runtimeRoot 'finish')|Out-Null
$goRecord=$records|Where-Object{$_.executable -like '*go.exe'}
$testProcess=Get-Process -Id $goRecord.id -ErrorAction SilentlyContinue
if($testProcess -and !$testProcess.WaitForExit(15000)){throw 'Browser test is still running; do not claim terminal acceptance or schema cleanup.'}
foreach($record in @($records|Where-Object{$_.executable -like '*node.exe'})){
 $process=Get-Process -Id $record.id -ErrorAction SilentlyContinue
 if(!$process){continue}
 $expected=if($record.started -is [DateTime]){$record.started.ToUniversalTime()}else{[DateTimeOffset]::Parse([string]$record.started).UtcDateTime}
 if($process.Path -ne $record.executable -or $process.StartTime.ToUniversalTime().Ticks -ne $expected.Ticks){throw 'Recorded acceptance process identity changed; refusing to stop it.'}
 & taskkill.exe /PID $process.Id /T /F | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Could not stop the recorded acceptance frontend tree.'}
}
if(!(Test-Path -LiteralPath (Join-Path $runtimeRoot 'verified'))){throw 'Acceptance did not verify persisted state. Inspect fixture logs; do not claim success.'}
$testOutput=@(Get-Content -LiteralPath (Join-Path $runtimeRoot 'api.stdout.log')|Where-Object{$_ -match '\S'})
if(!$testOutput.Count -or $testOutput[-1] -notmatch '^ok\s+dapper\.local/platform/internal/workspace\s' -or @($testOutput|Where-Object{$_ -match '^FAIL\b|^--- FAIL:'}).Count){throw 'Browser Go test did not finish successfully, including deferred schema cleanup; verified marker alone is insufficient.'}
Get-Content -LiteralPath (Join-Path $runtimeRoot 'verified')
Write-Output 'Isolated browser test completed; disposable schema cleaned by test and recorded frontend tree stopped.'
