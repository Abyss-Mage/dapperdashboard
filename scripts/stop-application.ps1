$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'process-identity.ps1')
$projectRoot=Split-Path $PSScriptRoot -Parent
$recordsPath=Join-Path $projectRoot '.cache/application-runtime/processes.json'
if(!(Test-Path -LiteralPath $recordsPath)){return}
foreach($record in @(Get-Content -LiteralPath $recordsPath -Raw|ConvertFrom-Json)){
 $process=Get-Process -Id $record.id -ErrorAction SilentlyContinue
 if(!$process -or $process.HasExited){continue}
 if(!(Test-DapperProcessIdentity -Process $process -Record $record)){
   if($process.HasExited){continue}
   throw 'Application process identity changed; unrelated process preserved.'
 }
 try {
   Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
 } catch {}
}
Remove-Item -LiteralPath $recordsPath -ErrorAction SilentlyContinue
Write-Output 'Recorded Dapper application processes stopped.'
