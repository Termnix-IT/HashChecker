# PowerShell版 手順書

Windows標準の`Get-FileHash`を使う業務版です。Windows PowerShell 5.1で、既存の`.bat`起動と結果のCSV保存に対応します。共通CLIの`--format json`とは別の、PowerShellオブジェクトによる記録方式です。

コマンド例はリポジトリのルートから実行します。

## batファイルで確認する

1. 作業フォルダに対象ファイルを1つと、期待ハッシュ値だけを1行で記載した`.txt`を1つ配置する。ハッシュ値は前後の空白・空行を許容する。テキストはUTF-8で保存する。
2. 使用する方式に合わせて、次のスクリプトを同じフォルダへコピーする。
3. `.bat`をダブルクリックする。
4. `[正常]`なら一致、`[警告]`なら不一致、`[エラー]`なら入力やファイルの状態を確認する。

| 方式 | 配置するスクリプト |
|---|---|
| SHA256 | `Run-SHA256Check.bat`、`Verify-SHA256Hash.ps1`、`HashChecker.Common.ps1` |
| MD5 | `Run-MD5Check.bat`、`Verify-MD5Hash.ps1`、`HashChecker.Common.ps1` |

`.bat`はスクリプトの配置先を作業フォルダにし、`powershell.exe -NoProfile -ExecutionPolicy Bypass`で起動します。リポジトリ内で使う場合は、`powershell/`が既定の作業フォルダです。

サンプルを別フォルダのまま確認する場合は、次のようにパラメーターを指定できます。

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\powershell\Verify-SHA256Hash.ps1 -Workspace .\testdata\ok-sha256
```

## 入力を指定する

| パラメーター | 動作・パスの基準 |
|---|---|
| `-Workspace` | 作業フォルダ。省略時はスクリプトの配置先。相対指定はカレントフォルダ基準 |
| `-HashFile` | 期待値を記載したファイル。相対指定は作業フォルダ基準 |
| `-TargetFile` | 確認対象。相対指定は作業フォルダ基準 |
| `-ExpectedHash` | 期待値の直接入力。`-HashFile`との同時指定はエラー |
| `-CsvPath` | CSV保存先。相対指定はカレントフォルダ基準 |
| `-PassThru` | 画面表示の代わりに結果オブジェクトを返す |

ハッシュファイルを省略すると作業フォルダ内の`.txt`から、対象を省略すると通常ファイルから自動検出します。候補が0件または複数ならエラーです。対象の自動検出では`.ps1`、`.bat`、`.cmd`、`.txt`、`.log`、`.csv`を除外します。明示指定した対象にはこの拡張子除外を適用しません。

SHA256版はSHA256、MD5版はMD5を使用します。方式を変えるパラメーターはなく、起動するスクリプトで選択します。ハッシュ値の文字数・16進数・正規化のルールは [共通仕様](../共通仕様.md#ハッシュ値の検証) を参照してください。

## 結果を記録する

新しい保存先へCSVを出力する例です。保存先の親フォルダを先に作成します。

```powershell
$runFolder = Join-Path '.\bin' ('powershell-check-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runFolder -Force | Out-Null
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\powershell\Verify-SHA256Hash.ps1 -Workspace .\testdata\ok-sha256 -CsvPath (Join-Path $runFolder 'result.csv')
$checkExitCode = $LASTEXITCODE
```

結果オブジェクトは、スクリプトを実行できるPowerShellセッション内で取得します。

```powershell
$result = & .\powershell\Verify-SHA256Hash.ps1 -Workspace .\testdata\ok-sha256 -PassThru
$checkExitCode = $LASTEXITCODE
$result | Select-Object Path, Status, ExitCode
```

| 項目 | 意味 |
|---|---|
| `Algorithm`・`Path` | 小文字の方式名と対象ファイル名 |
| `Status` | `match`、`mismatch`、`error` |
| `ExpectedHash`・`ActualHash` | 期待値と算出値。取得できない値は空文字列 |
| `ExitCode` | 一致0、不一致1、入力指定2、ハッシュファイル読取・CSV保存3、形式4、対象の計算5 |
| `ErrorMessage` | エラー時の説明。正常・不一致では空文字列 |

CSVはUTF-8で保存し、既存ファイルは上書きしません。保存に失敗した場合は終了コード3で通知します。保存失敗は返す結果を`error`へ変更するため、検証が一致していても終了コードが0とは限りません。

CSVでは数式に見える文字列の先頭に`'`を付けます。元の結果オブジェクトの文字列は変更しません。自動検出からCSVを除外するので、保存した記録が次回の対象候補になることを避けられます。

## エラー時の確認

候補数のエラーではファイル数と明示パラメーターを、形式エラーでは方式と期待値を、読取エラーでは権限・ファイルの状態を確認します。CSV保存エラーでは保存先が存在していないか、親フォルダを作成済みかも確認してください。

スクリプト実行が拒否された場合は、利用環境の実行ポリシーを確認してください。上記の起動方式でも実行を禁止する設定がある環境では、その環境の運用手順に従います。

開発時の確認は以下で実行します。

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\test_powershell.ps1
```
