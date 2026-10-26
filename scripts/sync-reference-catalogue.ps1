$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$source = Get-Content (Join-Path $projectRoot 'Dapper Detailing Knowledge/99 Sources/website-reference.json') -Raw | ConvertFrom-Json
if ($source.approval_status -ne 'reference_only_pending_client_approval') { throw 'Unexpected approval state; review the catalogue import.' }
$pages = @($source.pages | ForEach-Object {
    $sourceUri = [Uri]$_.url
    if ($sourceUri.Scheme -ne 'https' -or $sourceUri.Host -ne 'dapperdetailing.ca') { throw 'Unexpected source host.' }
    [ordered]@{ name = $sourceUri.AbsolutePath.Trim('/').Replace('-', ' '); source = $_.url; sourceSha256 = $_.sha256; publishedPriceLabels = @($_.published_price_labels) }
})
$reference = [ordered]@{ approvalStatus = $source.approval_status; currency = $source.currency; retrievedAt = $source.retrieved_at; transactionEnabled = $false; pages = $pages }
$target = Join-Path $projectRoot 'dapper-platform/apps/api/data/website-reference.json'
[IO.File]::WriteAllText($target, ($reference | ConvertTo-Json -Depth 8) + "`n", [Text.UTF8Encoding]::new($false))
Write-Output "Synced $($pages.Count) reference pages. Transactional pricing remains disabled."
