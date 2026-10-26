$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$statePath = Join-Path $projectRoot '.cache/demo-runtime/processes.json'
if (!(Test-Path -LiteralPath $statePath)) { Write-Output 'No recorded demo processes.'; return }
foreach ($record in @(Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json)) {
    $ownedProcess = Get-Process -Id $record.id -ErrorAction SilentlyContinue
    if (!$ownedProcess) { continue }
    $recordedStart = ([DateTime]$record.started).ToUniversalTime()
    if ($ownedProcess.Path -ne $record.executable -or $ownedProcess.StartTime.ToUniversalTime().Ticks -ne $recordedStart.Ticks) {
        Write-Warning "PID $($record.id) was reused; left it running."
        continue
    }
    Stop-Process -Id $ownedProcess.Id
}
Write-Output 'Recorded demo processes stopped.'
