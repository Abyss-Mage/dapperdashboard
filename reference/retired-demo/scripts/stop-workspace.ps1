$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'process-identity.ps1')
$projectRoot=Split-Path $PSScriptRoot -Parent
$runtimeRoot=Join-Path $projectRoot '.cache/workspace-runtime'
$processFile=Join-Path $runtimeRoot 'processes.json'
if(!(Test-Path -LiteralPath $processFile)){return}
$records=Get-Content -LiteralPath $processFile -Raw|ConvertFrom-Json
foreach($record in $records){$process=Get-Process -Id $record.id -ErrorAction SilentlyContinue;if(!$process){continue};if(Test-DapperProcessIdentity -Process $process -Record $record){Stop-Process -Id $process.Id}else{throw 'Recorded workspace process identity changed; process was not stopped.'}}
Write-Output 'Recorded scoped workspace processes stopped.'
