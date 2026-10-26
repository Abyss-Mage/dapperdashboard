$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'process-identity.ps1')
$instant=[DateTimeOffset]::Parse('2026-10-21T23:55:26.9863771Z')
$process=[PSCustomObject]@{Id=123;Path='C:\Synthetic\workspace.exe';StartTime=$instant.LocalDateTime}
foreach($timestamp in @($instant.ToString('o'),$instant.UtcDateTime)) {
 $record=[PSCustomObject]@{id=123;executable=$process.Path;started=$timestamp}
 if(!(Test-DapperProcessIdentity $process $record)){throw 'Equivalent recorded instant rejected.'}
}
foreach($record in @(
 [PSCustomObject]@{id=124;executable=$process.Path;started=$instant.ToString('o')},
 [PSCustomObject]@{id=123;executable='C:\Other\workspace.exe';started=$instant.ToString('o')},
 [PSCustomObject]@{id=123;executable=$process.Path;started=$instant.AddTicks(1).ToString('o')},
 [PSCustomObject]@{id=123;executable=$process.Path;started='invalid'}
)) {if(Test-DapperProcessIdentity $process $record){throw 'Mismatched process identity accepted.'}}
Write-Output 'PASS: string/DateTime timestamps normalize; PID/path/start mismatches remain denied.'
