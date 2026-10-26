$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$apiRoot = Join-Path $projectRoot 'dapper-platform/apps/api'
$webRoot = Join-Path $projectRoot 'dapper-platform/apps/web'
$runtimeRoot = Join-Path $projectRoot '.cache/demo-runtime'
New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
foreach ($port in @(8120, 3006)) {
    $portProbe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $port)
    try { $portProbe.Start() } catch {
        throw "Port $port is already occupied. Stop the existing demo before starting another."
    } finally { $portProbe.Stop() }
}
$apiExecutable = Join-Path $projectRoot '.cache/dapper-api.exe'
if (!(Test-Path -LiteralPath $apiExecutable)) { throw 'Build the API into .cache/dapper-api.exe first.' }
if (!(Test-Path -LiteralPath (Join-Path $webRoot '.next/BUILD_ID'))) { throw 'Run npm run build in apps/web first.' }
$nodeExecutable = (Get-Command node.exe).Source
$previousDemoMode = $env:DAPPER_DEMO
try {
    $env:DAPPER_DEMO = '1'
    $apiProcess = Start-Process -FilePath $apiExecutable -WorkingDirectory $apiRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'api.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'api.stderr.log')
    $webProcess = Start-Process -FilePath $nodeExecutable -ArgumentList 'node_modules/next/dist/bin/next start --hostname 127.0.0.1 --port 3006' -WorkingDirectory $webRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $runtimeRoot 'web.stdout.log') -RedirectStandardError (Join-Path $runtimeRoot 'web.stderr.log')
    @($apiProcess, $webProcess) | ForEach-Object { @{ id = $_.Id; started = $_.StartTime.ToUniversalTime().ToString('o'); executable = $_.Path } } | ConvertTo-Json | Set-Content (Join-Path $runtimeRoot 'processes.json')
} finally { $env:DAPPER_DEMO = $previousDemoMode }
$ready = $false
for ($attempt = 0; $attempt -lt 20; $attempt++) {
    if ($apiProcess.HasExited -or $webProcess.HasExited) { break }
    try {
        $health = Invoke-RestMethod 'http://127.0.0.1:8120/api/v1/health'
        $page = Invoke-WebRequest 'http://127.0.0.1:3006/'
        if ($health.mode -eq 'local-demo' -and $page.StatusCode -eq 200) { $ready = $true; break }
    } catch { Start-Sleep -Milliseconds 500 }
}
if (!$ready) {
    foreach ($startedProcess in @($apiProcess, $webProcess)) { if (!$startedProcess.HasExited) { Stop-Process -Id $startedProcess.Id } }
    throw 'Demo startup failed. Inspect .cache/demo-runtime logs.'
}
Write-Output 'Synthetic demo ready at http://127.0.0.1:3006. Logs: .cache/demo-runtime'
