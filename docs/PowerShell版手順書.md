# PowerShell 版 手順書

この手順書は、業務制約版として残している PowerShell 実装の利用手順です。

PowerShell 版は、`.bat`、`.ps1`、ハッシュファイル、確認対象ファイルを同じ作業フォルダに置いて使う想定です。このリポジトリ内で試す場合は、`powershell/` フォルダを作業フォルダとして使用します。

## MD5 操作手順

1. 作業フォルダに以下を配置する。
   - `Run-MD5Check.bat`
   - `Verify-MD5Hash.ps1`
   - `HashChecker.Common.ps1`
   - ベンダー提供 MD5 値を記載した txt ファイル
   - 確認対象ファイル

2. txt ファイルには MD5 値のみを 1 行で記載する。

3. `Run-MD5Check.bat` をダブルクリックする。

4. `[正常] ハッシュ値が一致しました。` と表示されることを確認する。

5. `[警告]` または `[エラー]` が表示された場合は、対象ファイル、ハッシュ値、配置ファイルを確認する。

## SHA256 操作手順

1. 作業フォルダに以下を配置する。
   - `Run-SHA256Check.bat`
   - `Verify-SHA256Hash.ps1`
   - `HashChecker.Common.ps1`
   - ベンダー提供 SHA256 値を記載した txt ファイル
   - 確認対象ファイル

2. txt ファイルには SHA256 値のみを 1 行で記載する。

3. `Run-SHA256Check.bat` をダブルクリックする。

4. `[正常] ハッシュ値が一致しました。` と表示されることを確認する。

5. `[警告]` または `[エラー]` が表示された場合は、対象ファイル、ハッシュ値、配置ファイルを確認する。

## パラメーター指定とCSV保存

既存の`.bat`による起動に加え、Windows PowerShell 5.1から作業フォルダとCSV保存先を指定できます。

```powershell
.\powershell\Verify-SHA256Hash.ps1 -Workspace .\testdata\ok-sha256 -CsvPath .\sha256-result.csv
```

`-HashFile` と `-TargetFile` は作業フォルダ基準の相対パス、または絶対パスで指定できます。`-ExpectedHash` を指定するとハッシュファイルが不要になります（`-HashFile` との同時指定はエラー）。CSV保存先はカレントフォルダ基準です。

`-PassThru` を指定すると、画面表示の代わりに `Algorithm`、`Path`、`Status`、`ExpectedHash`、`ActualHash`、`ExitCode`、`ErrorMessage` を持つオブジェクトを返します。

```powershell
$result = .\powershell\Verify-SHA256Hash.ps1 -Workspace .\testdata\ok-sha256 -PassThru
$result | Select-Object Path, Status, ExitCode
```

CSVは日本語ファイル名を保持するUTF-8で保存し、既存ファイルは上書きしません。CSV保存に失敗した場合は終了コード3です。自動検出ではCSVを対象から除外するため、保存した結果が次回の検査を妨げません。MD5版にも同じパラメーターを指定できます。

CSVでは数式に見える文字列の先頭に `'` を付けます。結果オブジェクトの元の文字列は変更しません。
