# Verify-MD5Hash.ps1
# 説明:
#   同一フォルダ内のtxtファイルに記載されたMD5ハッシュ値と、
#   同一フォルダ内の確認対象ファイルから算出したMD5ハッシュ値を比較する。

$Algorithm = "MD5"
$ExpectedHashLength = 32

function Write-ErrorAndExit {
    param (
        [string]$Message,
        [int]$ExitCode
    )

    Write-Host ""
    Write-Host "[エラー] $Message" -ForegroundColor Red
    Write-Host ""
    Write-Host "処理を終了します。"
    exit $ExitCode
}

function Write-FileList {
    param (
        [string]$Title,
        [array]$Files
    )

    Write-Host $Title
    foreach ($file in $Files) {
        Write-Host "- $($file.Name)"
    }
}

try {
    # スクリプト配置フォルダを作業フォルダとして取得
    if ([string]::IsNullOrWhiteSpace($PSScriptRoot)) {
        $Workspace = Split-Path -Parent $MyInvocation.MyCommand.Path
    } else {
        $Workspace = $PSScriptRoot
    }

    Write-Host "========================================"
    Write-Host " ハッシュ値確認ツール"
    Write-Host "========================================"
    Write-Host ""
    Write-Host "作業フォルダ : $Workspace"
    Write-Host "ハッシュ方式 : $Algorithm"
    Write-Host ""

    # txtファイルを検索
    $HashFiles = @(Get-ChildItem -Path $Workspace -File -Filter "*.txt" -ErrorAction Stop)

    if ($HashFiles.Count -eq 0) {
        Write-ErrorAndExit "ハッシュファイルが見つかりません。同一フォルダにtxtファイルを1つ配置してください。" 2
    }

    if ($HashFiles.Count -ge 2) {
        Write-Host "[エラー] ハッシュファイル候補が複数見つかりました。" -ForegroundColor Red
        Write-Host ""
        Write-FileList "見つかったtxtファイル:" $HashFiles
        Write-Host ""
        Write-Host "対応: ハッシュ値を記載したtxtファイルを1つだけ残してください。"
        exit 2
    }

    $HashFile = $HashFiles[0]

    # 確認対象ファイルを検索
    $ExcludedExtensions = @(".ps1", ".bat", ".cmd", ".txt", ".log")

    $TargetFiles = @(
        Get-ChildItem -Path $Workspace -File -ErrorAction Stop |
            Where-Object {
                $ExcludedExtensions -notcontains $_.Extension.ToLower()
            }
    )

    if ($TargetFiles.Count -eq 0) {
        Write-ErrorAndExit "確認対象ファイルが見つかりません。同一フォルダに確認したいファイルを1つ配置してください。" 2
    }

    if ($TargetFiles.Count -ge 2) {
        Write-Host "[エラー] 確認対象ファイル候補が複数見つかりました。" -ForegroundColor Red
        Write-Host ""
        Write-FileList "見つかったファイル:" $TargetFiles
        Write-Host ""
        Write-Host "対応: 確認したいファイルを1つだけ残してください。"
        exit 2
    }

    $TargetFile = $TargetFiles[0]

    # ハッシュファイル読み込み
    try {
        $RawHashText = Get-Content -Path $HashFile.FullName -Raw -ErrorAction Stop

        $HashLines = @(
            $RawHashText -split "\r?\n" |
                ForEach-Object { ([string]$_).Trim() } |
                Where-Object { $_ -ne "" }
        )
    } catch {
        Write-ErrorAndExit "ハッシュファイルを読み取れませんでした。ファイルの権限や状態を確認してください。" 3
    }

    if ($HashLines.Count -eq 0) {
        Write-ErrorAndExit "ハッシュファイルにハッシュ値が記載されていません。" 4
    }

    if ($HashLines.Count -ge 2) {
        Write-Host "[エラー] ハッシュファイルに複数行の値が記載されています。" -ForegroundColor Red
        Write-Host ""
        Write-Host "対応: ハッシュ値のみを1行で記載してください。"
        exit 4
    }

    $VendorHash = $HashLines[0].Trim()

    # 文字数チェック
    if ($VendorHash.Length -ne $ExpectedHashLength) {
        Write-Host "[エラー] $Algorithm ハッシュ値の文字数が不正です。" -ForegroundColor Red
        Write-Host ""
        Write-Host "期待する文字数 : $ExpectedHashLength 文字"
        Write-Host "実際の文字数   : $($VendorHash.Length) 文字"
        Write-Host ""
        Write-Host "ハッシュファイルには $Algorithm ハッシュ値のみを1行で記載してください。"
        exit 4
    }

    # 16進数チェック
    if ($VendorHash -notmatch "^[0-9a-fA-F]+$") {
        Write-Host "[エラー] ハッシュ値の形式が不正です。" -ForegroundColor Red
        Write-Host ""
        Write-Host "ハッシュ値には 0-9、a-f、A-F のみ使用できます。"
        Write-Host "ハッシュファイルの内容を確認してください。"
        exit 4
    }

    # 確認対象ファイルのハッシュ値を算出
    try {
        $ActualHash = (Get-FileHash -Path $TargetFile.FullName -Algorithm $Algorithm -ErrorAction Stop).Hash
    } catch {
        Write-ErrorAndExit "確認対象ファイルのハッシュ値を算出できませんでした。ファイルの権限や状態を確認してください。" 5
    }

    $VendorHashNormalized = $VendorHash.ToLower()
    $ActualHashNormalized = $ActualHash.ToLower()

    if ($VendorHashNormalized -eq $ActualHashNormalized) {
        Write-Host "[正常] ハッシュ値が一致しました。" -ForegroundColor Green
        Write-Host ""
        Write-Host "作業フォルダ       : $Workspace"
        Write-Host "ハッシュ方式       : $Algorithm"
        Write-Host "ハッシュファイル   : $($HashFile.Name)"
        Write-Host "確認対象ファイル   : $($TargetFile.Name)"
        Write-Host ""
        Write-Host "ベンダー提供ハッシュ値 : $VendorHashNormalized"
        Write-Host "算出ハッシュ値         : $ActualHashNormalized"
        Write-Host ""
        Write-Host "結果 : ファイルはベンダー提供ハッシュ値と一致しています。"
        exit 0
    } else {
        Write-Host "[警告] ハッシュ値が一致しません。" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "作業フォルダ       : $Workspace"
        Write-Host "ハッシュ方式       : $Algorithm"
        Write-Host "ハッシュファイル   : $($HashFile.Name)"
        Write-Host "確認対象ファイル   : $($TargetFile.Name)"
        Write-Host ""
        Write-Host "ベンダー提供ハッシュ値 : $VendorHashNormalized"
        Write-Host "算出ハッシュ値         : $ActualHashNormalized"
        Write-Host ""
        Write-Host "結果 : ファイルが破損している、または想定と異なる可能性があります。"
        Write-Host "確認 : ベンダー提供値、確認対象ファイル、ハッシュ方式を確認してください。"
        exit 1
    }

} catch {
    Write-Host ""
    Write-Host "[エラー] 想定外のエラーが発生しました。" -ForegroundColor Red
    Write-Host ""
    Write-Host $_.Exception.Message
    exit 5
}