# 多言語CLI・GUI版 手順書

Python / Go / C# / Rustの単体検証、JSON保存、用途別の連携とWindows GUIの操作を説明します。引数・終了コード・結果形式の定義は [共通仕様](../共通仕様.md) を参照してください。

## 事前準備

コマンド例は、WindowsのPowerShellからリポジトリのルートをカレントフォルダにして実行します。使用する実装の開発・実行環境を用意し、ビルドが必要な実装は次のREADMEに従って先にビルドしてください。

| 実装 | 準備 | 例で使用する実行入口 |
|---|---|---|
| Python | [Python版README](../python/README.md) | `python .\python\hash_checker.py` |
| Go | [Go版README](../go/README.md) | `.\bin\hash-checker.exe` |
| C# CLI | [C#/.NET版README](../csharp/README.md) | `dotnet .\csharp\HashChecker\bin\Debug\net10.0\HashChecker.dll` |
| Rust | [Rust版README](../rust/README.md) | `.\rust\target\release\hash_checker.exe` |

以下はPythonで共通操作を示します。他の共通CLIでも上表の実行入口に置き換え、同じ引数を指定します。Ubuntuでは、ビルド先・実行入口のパスを各言語READMEの説明に合わせてください。

## 単体ファイルを確認する

作業フォルダに対象ファイルを1つと、期待値だけを1行で書いた`.txt`を1つ配置します。ハッシュファイルはUTF-8で保存します。同梱サンプルでSHA256を確認する例:

```powershell
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256
$checkExitCode = $LASTEXITCODE
```

MD5の場合は`--algorithm md5`と`testdata\ok-md5`を指定します。一致は終了コード0、不一致は1です。2以上なら引数、期待値、ファイルの状態を確認してください。

候補が複数ある場合や、自動検出から除外される拡張子のファイルを確認する場合は、入力を明示します。

```powershell
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256 --hash-file vendor_hash.txt --target-file firmware.bin
```

`--hash-file`と`--target-file`の相対パスは作業フォルダ基準です。上の例で`testdata\ok-sha256\firmware.bin`を再度指定すると、作業フォルダを二重に含めることになるため、ファイル名だけを指定します。

## 期待値を直接入力する

Webサイト等からコピーした値を`--expected-hash`へ渡すと、ハッシュファイルが不要になります。以下はサンプルの期待値を読み、直接入力として渡す例です。

```powershell
$expected = (Get-Content -LiteralPath .\testdata\ok-sha256\vendor_hash.txt -Raw -Encoding UTF8).Trim()
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256 --expected-hash $expected --target-file firmware.bin
```

`--hash-file`とは同時指定できません。値に空白がある場合は1つの引数として渡してください。

対応機能やCLIのバージョンを調べる場合:

```powershell
python .\python\hash_checker.py --capabilities
python .\python\hash_checker.py --version
```

## JSONを保存する

`--format json`でJSONだけを標準出力へ出します。保存先は対象フォルダと分けてください。以下の例は毎回新しい保存フォルダを作成します。

```powershell
$runFolder = Join-Path '.\bin' ('hash-check-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runFolder -Force | Out-Null
$previousConsoleEncoding = [Console]::OutputEncoding
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
try {
    python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256 --format json | Set-Content -LiteralPath (Join-Path $runFolder 'single.json') -Encoding UTF8
    $checkExitCode = $LASTEXITCODE
} finally {
    [Console]::OutputEncoding = $previousConsoleEncoding
}
```

外部CLIの出力をUTF-8として受け取る設定と、保存先のUTF-8指定を組み合わせています。Windows PowerShell 5.1の`>`はUTF-16LEになるため、この手順では`Set-Content -Encoding UTF8`を使用します。文字コードの背景は [Microsoftの説明](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_character_encoding?view=powershell-5.1) を参照してください。

ビルド済み実行ファイル・DLLを使うと、ビルド時の出力をJSONへ混ぜずに保存できます。自動処理でGoの終了コードを使う場合も、`go run`ではなくビルド済みバイナリを使ってください。終了コードは次の外部コマンドを実行する前に`$LASTEXITCODE`から取得します。

## フォルダを記録して検証と比較を行う

GoとRustのビルドを済ませてから実行します。次の例は、サンプルフォルダを記録してGoで検証し、Rustでフォルダ全体を比較します。

```powershell
$runFolder = Join-Path '.\bin' ('folder-check-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runFolder -Force | Out-Null
$manifestPath = Join-Path $runFolder 'checksums.txt'
.\rust\target\release\hash_checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --create-manifest $manifestPath
$createExitCode = $LASTEXITCODE
if ($createExitCode -ne 0) { throw '一覧を作成できませんでした。' }

$previousConsoleEncoding = [Console]::OutputEncoding
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
try {
    .\bin\hash-checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --manifest $manifestPath --format json | Set-Content -LiteralPath (Join-Path $runFolder 'verified.json') -Encoding UTF8
    $verifyExitCode = $LASTEXITCODE
    .\rust\target\release\hash_checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --compare-manifest $manifestPath --format json | Set-Content -LiteralPath (Join-Path $runFolder 'compared.json') -Encoding UTF8
    $compareExitCode = $LASTEXITCODE
} finally {
    [Console]::OutputEncoding = $previousConsoleEncoding
}
```

一覧の入力・保存パスはカレントフォルダ基準で、一覧内の対象パスは`--workspace`基準です。SHA256一覧モードは、単体用の明示ファイル指定や直接入力とは併用しません。

| 処理 | 検査範囲 | 結果の見方 |
|---|---|---|
| Rustの一覧作成 | サブフォルダを含む通常ファイル | 成功0。既存保存先は上書きせず2 |
| Goの一覧検証 | 一覧に記載された対象 | 一致0、不一致1、欠落・読取失敗3 |
| Rustのフォルダ比較 | 一覧とフォルダ全体 | 一致0。追加`added`・削除`removed`・変更`changed`があれば1 |

フォルダ内の`.txt`や`.log`も一覧に含まれます。このサンプルでは`firmware.bin`と`vendor_hash.txt`を記録します。Goは一覧にない追加ファイルを検査しません。Rustは追加ファイルも検出し、一覧自身だけを除外します。

結果JSONは対象フォルダの外へ保存してください。対象フォルダ内へ保存すると、次回のRust比較で追加ファイルとして検出されます。保存先が存在する一覧・CSVを作り直す場合は、新しい出力フォルダを使います。

Rustの一覧作成先にはNTFS等のhard link対応環境が必要です。FAT等では終了コード3になります。対応一覧はGNU形式の限定範囲で、フォルダ外参照・リンク・重複・未対応行を拒否します。詳細は [一覧の仕様](../共通仕様.md#sha256チェックサム一覧) を参照してください。

Goのテキスト検証で成功出力を抑制する例:

```powershell
.\bin\hash-checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --manifest $manifestPath --quiet
```

不一致・エラーは表示されます。`--format json`の結果は`--quiet`でも省略しません。

## 結果をCSVへ集計する

上のフォルダ確認で作成したJSONを、Pythonで集計します。

```powershell
python .\python\summarize_results.py (Join-Path $runFolder 'verified.json') (Join-Path $runFolder 'compared.json') --output (Join-Path $runFolder 'summary.csv')
$summaryExitCode = $LASTEXITCODE
```

相対パスとアルゴリズムごとに集計し、次の列を出力します。

| 列 | 意味 |
|---|---|
| `path`・`algorithm` | 対象の相対パスと方式 |
| `checks` | その行に集計した結果件数 |
| `match`・`mismatch`・`missing`・`error` | 一致・不一致・欠落・処理エラーの件数 |
| `added`・`removed`・`changed` | フォルダ比較の追加・削除・変更の件数 |

入力全体のエラーは空の`path`行へ集計し、方式不明なら`algorithm`も空になります。異なる基準フォルダでも同じ相対パス・方式は同じ行へ集計するため、同じ対象群の記録だけを指定してください。1入力は16MiB以下のUTF-8 JSONで、BOMも読めます。形式のバージョン、判定とハッシュ値、件数、終了コードの整合性を検査してからCSVを作成します。

集計の終了コードは、成功0、引数不正2、入力読取・保存失敗3、JSON形式不正4です。不一致を含む結果でも、形式が正しければ集計は成功します。対象の不一致はCSV内の件数で確認してください。

保存先の親フォルダは先に作成します。既存CSVは上書きせず、`--output`を省略すると標準出力へCSVを表示します。ファイル保存は日本語を保持するUTF-8（BOM付き）です。

CSVでは、空白を除いた先頭が`=`・`+`・`-`・`@`の文字列等に`'`を付け、表計算ソフトで数式として解釈されることを防ぎます。JSONの元のファイル名は変更しません。

## Windows GUIで確認する

.NET 10 SDKを使い、Windowsで起動します。

```powershell
dotnet run --project .\csharp\HashChecker.Gui
```

1. ファイルを選択するか、画面へ1ファイルをドラッグする。
2. SHA256またはMD5を選び、配布元の期待値を貼り付ける。
3. 「ハッシュを確認」を押し、一致・不一致を確認する。
4. 必要なら「結果をコピー」で記録する。処理中は「キャンセル」で中止する。

GUIは1ファイルの確認を担当します。入力を変更すると以前の結果を消し、処理中は入力を固定します。CLIと検証処理を共有し、GoやRustの実行環境は必要ありません。配布先でのランタイム条件と画面の開発確認は [C#/.NET版README](../csharp/README.md) を参照してください。

## エラー時の確認

- 候補数のエラー: 入力フォルダのファイル数を確認するか、単体の入力を明示する。
- 形式エラー: 方式、期待値、一覧の行形式・パス・重複を確認する。
- 不一致・変更: 配布元の期待値と対象ファイルを確認する。
- 読取・保存エラー: 権限、ロック、親フォルダ、リンク、保存先の対応ファイルシステムを確認する。
- JSON集計エラー: 共通JSONを保存したか、文字コードや件数が壊れていないかを確認する。

終了コードの完全な定義は [共通仕様](../共通仕様.md#終了コード) を参照してください。
