$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$databaseRoot = Join-Path $projectRoot '.cache/postgresql'
$binaryRoot = Join-Path $databaseRoot 'runtime/pgsql/bin'
$dataRoot = Join-Path $databaseRoot 'data'
$credentialPath = Join-Path $databaseRoot 'local-credential.clixml'
if (!(Test-Path (Join-Path $binaryRoot 'initdb.exe'))) { throw 'Unpack the official PostgreSQL Windows archive under .cache/postgresql/runtime first.' }
if (!(Test-Path (Join-Path $dataRoot 'PG_VERSION'))) {
    if (Test-Path $dataRoot) { throw 'An incomplete data directory exists. Inspect it before initialization.' }
    $secretBytes = [byte[]]::new(32)
    [Security.Cryptography.RandomNumberGenerator]::Fill($secretBytes)
    $secretText = [Convert]::ToBase64String($secretBytes)
    $credential = [PSCredential]::new('dapper_local', (ConvertTo-SecureString $secretText -AsPlainText -Force))
    $credential | Export-Clixml -LiteralPath $credentialPath
    $passwordPath = Join-Path $databaseRoot 'init-password.tmp'
    try {
        [IO.File]::WriteAllText($passwordPath, $secretText, [Text.UTF8Encoding]::new($false))
        & (Join-Path $binaryRoot 'initdb.exe') -D $dataRoot -U dapper_local --auth=scram-sha-256 --encoding=UTF8 --locale=C --pwfile=$passwordPath > (Join-Path $databaseRoot 'init.log') 2>&1
        if ($LASTEXITCODE -ne 0) { throw 'Database initialization failed. Inspect .cache/postgresql/init.log.' }
    } finally {
        $secretText = $null
        if (Test-Path -LiteralPath $passwordPath) { Remove-Item -LiteralPath $passwordPath }
    }
}
$control = Join-Path $binaryRoot 'pg_ctl.exe'
& $control status -D $dataRoot > $null 2>&1
if ($LASTEXITCODE -eq 0) { Write-Output 'Project PostgreSQL is already running.'; return }
$probe = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,15432)
try { $probe.Start() } finally { $probe.Stop() }
$arguments = "start -w -D `"$dataRoot`" -l `"$(Join-Path $databaseRoot 'server.log')`" -o `"-h 127.0.0.1 -p 15432`""
$process = Start-Process -FilePath $control -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $databaseRoot 'start.log') -RedirectStandardError (Join-Path $databaseRoot 'start-error.log')
# Wait for pg_ctl itself, not its intentionally long-lived postgres descendants.
$process.WaitForExit()
if ($process.ExitCode -ne 0) { throw 'PostgreSQL failed to start. Inspect .cache/postgresql logs.' }
Write-Output 'Native project PostgreSQL ready on 127.0.0.1:15432; SCRAM authentication; no system service.'
