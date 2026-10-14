# Windows PowerShell 5.1-compatible verification and result export.

function New-HashVerificationResult {
    param([string]$Algorithm, [string]$Path, [string]$ExpectedHash, [string]$ActualHash,
          [string]$Status, [int]$ExitCode, [string]$ErrorMessage)
    [pscustomobject][ordered]@{
        Algorithm = $Algorithm.ToLowerInvariant()
        Path = $Path
        Status = $Status
        ExpectedHash = $ExpectedHash
        ActualHash = $ActualHash
        ExitCode = $ExitCode
        ErrorMessage = $ErrorMessage
    }
}

function Resolve-HashCheckerPath {
    param([string]$Path, [string]$Workspace)
    if ([IO.Path]::IsPathRooted($Path)) { return $Path }
    return Join-Path -Path $Workspace -ChildPath $Path
}

function Invoke-HashVerification {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][ValidateSet('MD5', 'SHA256')][string]$Algorithm,
        [Parameter(Mandatory = $true)][string]$Workspace,
        [string]$HashFile,
        [string]$TargetFile,
        [AllowEmptyString()][string]$ExpectedHash
    )
    $targetName = ''
    $expected = ''
    $actual = ''
    $failureCode = 2
    $failureMessage = '入力ファイルの指定を確認してください。'
    try {
        if ($PSBoundParameters.ContainsKey('ExpectedHash') -and $HashFile) { throw '入力競合' }
        $directory = Get-Item -LiteralPath $Workspace -ErrorAction Stop
        if (-not $directory.PSIsContainer) { throw '作業フォルダを指定してください。' }
        $workspacePath = $directory.FullName
        if ($TargetFile) {
            $target = Get-Item -LiteralPath (Resolve-HashCheckerPath $TargetFile $workspacePath) -ErrorAction Stop
        } else {
            $excluded = @('.ps1', '.bat', '.cmd', '.txt', '.log', '.csv')
            $targets = @(Get-ChildItem -LiteralPath $workspacePath -File -ErrorAction Stop |
                Where-Object { $excluded -notcontains $_.Extension.ToLowerInvariant() })
            if ($targets.Count -ne 1) { throw '確認対象ファイルを1つ配置してください。' }
            $target = $targets[0]
        }
        if ($target.PSIsContainer) { throw '対象には通常ファイルを指定してください。' }
        $targetName = $target.Name
        if ($PSBoundParameters.ContainsKey('ExpectedHash')) {
            $expected = $ExpectedHash.Trim()
        } else {
            if ($HashFile) {
                $hashPath = Resolve-HashCheckerPath $HashFile $workspacePath
            } else {
                $hashFiles = @(Get-ChildItem -LiteralPath $workspacePath -File -Filter '*.txt' -ErrorAction Stop)
                if ($hashFiles.Count -ne 1) { throw 'ハッシュファイルを1つ配置してください。' }
                $hashPath = $hashFiles[0].FullName
            }
            $failureCode = 3
            $failureMessage = 'ハッシュファイルを読み取れませんでした。'
            $encoding = New-Object System.Text.UTF8Encoding($false, $true)
            $raw = [IO.File]::ReadAllText($hashPath, $encoding)
            $failureCode = 4
            $failureMessage = 'ハッシュ値のみを1行で記載してください。'
            $lines = @($raw -split '\r?\n' | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' })
            if ($lines.Count -ne 1) { throw $failureMessage }
            $expected = $lines[0]
        }
        $failureCode = 4
        $failureMessage = '期待ハッシュ値の文字数または16進数形式が不正です。'
        $length = if ($Algorithm -eq 'MD5') { 32 } else { 64 }
        if ($expected.Length -ne $length -or $expected -cnotmatch '^[0-9a-fA-F]+$') { throw $failureMessage }
        $expected = $expected.ToLowerInvariant()
        $failureCode = 5
        $failureMessage = '確認対象ファイルのハッシュ値を算出できませんでした。'
        $actual = (Get-FileHash -LiteralPath $target.FullName -Algorithm $Algorithm -ErrorAction Stop).Hash.ToLowerInvariant()
        if ($expected -eq $actual) {
            return New-HashVerificationResult $Algorithm $targetName $expected $actual 'match' 0 ''
        }
        return New-HashVerificationResult $Algorithm $targetName $expected $actual 'mismatch' 1 ''
    } catch {
        return New-HashVerificationResult $Algorithm $targetName $expected $actual 'error' $failureCode $failureMessage
    }
}

function Write-HashVerificationResult {
    param([Parameter(Mandatory = $true)]$Result)
    if ($Result.ExitCode -eq 0) {
        Write-Host '[正常] ハッシュ値が一致しました。' -ForegroundColor Green
    } elseif ($Result.ExitCode -eq 1) {
        Write-Host '[警告] ハッシュ値が一致しません。' -ForegroundColor Yellow
    } else {
        Write-Host ('[エラー] ' + $Result.ErrorMessage) -ForegroundColor Red
    }
    Write-Host ('ハッシュ方式 : ' + $Result.Algorithm)
    Write-Host ('確認対象ファイル : ' + $Result.Path)
    Write-Host ('ベンダー提供ハッシュ値 : ' + $Result.ExpectedHash)
    Write-Host ('算出ハッシュ値 : ' + $Result.ActualHash)
}

function Export-HashVerificationResult {
    param([Parameter(Mandatory = $true)]$Result, [Parameter(Mandatory = $true)][string]$CsvPath)
    $safe = [ordered]@{}
    foreach ($property in $Result.PSObject.Properties) {
        $value = $property.Value
        if ($value -is [string] -and ($value.TrimStart() -match '^[=+@-]' -or $value -match '^[\t\r\n]')) {
            $value = "'" + $value
        }
        $safe[$property.Name] = $value
    }
    [pscustomobject]$safe | Export-Csv -LiteralPath $CsvPath -Encoding UTF8 -NoTypeInformation -NoClobber -ErrorAction Stop
}
