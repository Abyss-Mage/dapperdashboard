function Test-DapperProcessIdentity {
    param($Process, $Record)
    try {
        if (!$Process -or !$Record -or !$Record.executable -or !$Record.started) { return $false }
        if ($Process.Id -ne $Record.id -or $Process.Path -ne $Record.executable) { return $false }
        # ConvertFrom-Json may produce either a string or a DateTime depending
        # on PowerShell version. Compare normalized instants, never display text.
        $expected = [DateTimeOffset]$Record.started
        return $Process.StartTime.ToUniversalTime().Ticks -eq $expected.UtcDateTime.Ticks
    } catch { return $false }
}
