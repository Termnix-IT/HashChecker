$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../powershell/HashChecker.Common.ps1')
$repo = Split-Path -Parent $PSScriptRoot
$fixture = Join-Path $repo 'testdata/ok-sha256'
$result = Invoke-HashVerification -Algorithm SHA256 -Workspace $fixture
if ($result.ExitCode -ne 0 -or $result.Status -ne 'match') { throw 'SHA256 match failed' }
$md5 = Invoke-HashVerification -Algorithm MD5 -Workspace (Join-Path $repo 'testdata/ok-md5')
if ($md5.ExitCode -ne 0) { throw 'MD5 match failed' }
$mismatch = Invoke-HashVerification -Algorithm MD5 -Workspace (Join-Path $repo 'testdata/mismatch')
if ($mismatch.ExitCode -ne 1) { throw 'Mismatch failed' }
$invalid = Invoke-HashVerification -Algorithm SHA256 -Workspace (Join-Path $repo 'testdata/invalid-hash')
if ($invalid.ExitCode -ne 4) { throw 'Invalid input failed' }
$multiple = Invoke-HashVerification -Algorithm MD5 -Workspace (Join-Path $repo 'testdata/multiple-targets')
if ($multiple.ExitCode -ne 2) { throw 'Multiple targets failed' }
$conflict = Invoke-HashVerification -Algorithm SHA256 -Workspace $fixture -HashFile 'vendor_hash.txt' -ExpectedHash '0'
if ($conflict.ExitCode -ne 2) { throw 'Conflicting inputs failed' }
$temporary = Join-Path $repo ('bin/ps-test-' + [Guid]::NewGuid().ToString('N'))
$null = New-Item -Path $temporary -ItemType Directory -Force
try {
    $name = ([char]0x65E5).ToString() + ' file[1].bin'
    $target = Join-Path $temporary $name
    [IO.File]::WriteAllBytes($target, [byte[]](1, 2, 3))
    $expected = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash
    $direct = Invoke-HashVerification -Algorithm SHA256 -Workspace $temporary -ExpectedHash $expected
    if ($direct.ExitCode -ne 0 -or $direct.Path -ne $name) { throw 'Direct input and literal path failed' }
    $csv = Join-Path $temporary 'result.csv'
    Export-HashVerificationResult $direct $csv
    $row = Import-Csv -LiteralPath $csv -Encoding UTF8
    if ($row.Path -ne $name -or $row.Status -ne 'match') { throw 'CSV round trip failed' }
    $original = [IO.File]::ReadAllText($csv)
    $refused = $false
    try { Export-HashVerificationResult $direct $csv } catch { $refused = $true }
    if (-not $refused -or [IO.File]::ReadAllText($csv) -ne $original) { throw 'CSV overwrite protection failed' }
    $again = Invoke-HashVerification -Algorithm SHA256 -Workspace $temporary -ExpectedHash $expected
    if ($again.ExitCode -ne 0) { throw 'CSV discovery exclusion failed' }
    $formula = New-HashVerificationResult SHA256 '=1+1' '=2+2' '' 'error' 4 'invalid'
    $formulaCsv = Join-Path $temporary 'formula.csv'
    Export-HashVerificationResult $formula $formulaCsv
    $formulaRow = Import-Csv -LiteralPath $formulaCsv -Encoding UTF8
    if ($formulaRow.Path -ne "'=1+1" -or $formulaRow.ExpectedHash -ne "'=2+2" -or $formula.Path -ne '=1+1') {
        throw 'CSV formula protection failed'
    }
    $object = & (Join-Path $repo 'powershell/Verify-SHA256Hash.ps1') -Workspace $temporary -ExpectedHash $expected -PassThru
    if ($LASTEXITCODE -ne 0 -or $object.Status -ne 'match') { throw 'Wrapper object output failed' }
    Write-Host 'PowerShell checks passed (match, mismatch, invalid, discovery, literal path, direct, CSV, wrapper).'
} finally {
    $resolved = [IO.Path]::GetFullPath($temporary)
    $allowed = [IO.Path]::GetFullPath((Join-Path $repo 'bin')) + [IO.Path]::DirectorySeparatorChar
    if (-not $resolved.StartsWith($allowed, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe test cleanup path' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
