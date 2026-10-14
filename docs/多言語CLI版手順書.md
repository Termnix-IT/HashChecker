# 多言語 CLI 版 手順書

この手順書は、共通仕様に沿って実装した Python / Go / C#/.NET / Rust 版の利用手順です。

各 CLI 版は、以下の共通入力を扱います。

- `--algorithm`: `md5` または `sha256`
- `--workspace`: ハッシュファイルと確認対象ファイルを自動検出する作業フォルダ
- `--hash-file`: ハッシュファイルを明示する場合のパス
- `--target-file`: 確認対象ファイルを明示する場合のパス
- `--expected-hash`: テキストファイルを作らず期待値を直接指定（`--hash-file` と併用不可）
- `--format json`: 共通JSONだけを標準出力へ表示（既定は `text`）
- `--version` / `--capabilities`: 単独でバージョン・対応機能を確認

## 事前準備

1. 作業フォルダを用意する。

2. 作業フォルダに以下を配置する。
   - ベンダー提供ハッシュ値を記載した txt ファイル
   - 確認対象ファイル

3. txt ファイルにはハッシュ値のみを 1 行で記載する。

このリポジトリ内で試す場合は、`testdata/ok-sha256` や `testdata/ok-md5` を使用できます。

## Python 版

SHA256:

```powershell
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
python .\python\hash_checker.py --algorithm md5 --workspace .\testdata\ok-md5
```

## Go 版

SHA256:

```powershell
go run .\go --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
go run .\go --algorithm md5 --workspace .\testdata\ok-md5
```

## C#/.NET 版

SHA256:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm md5 --workspace .\testdata\ok-md5
```

## Rust 版

SHA256:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm md5 --workspace .\testdata\ok-md5
```

## ファイルを明示する場合

自動検出ではなく、ハッシュファイルと確認対象ファイルを明示する場合は `--hash-file` と `--target-file` を指定します。

Python 版の例:

```powershell
python .\python\hash_checker.py --algorithm md5 --hash-file .\testdata\ok-md5\vendor_hash.txt --target-file .\testdata\ok-md5\firmware.bin
```

他の言語版でも、実行コマンドの後ろに同じ `--algorithm`、`--hash-file`、`--target-file` を指定します。

## 直接入力とJSON保存

```powershell
$expected = (Get-Content .\testdata\ok-sha256\vendor_hash.txt -Raw).Trim()
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256 --expected-hash $expected --format json | Set-Content -Encoding UTF8 .\single-result.json
```

PowerShell 5.1の `>` はUTF-16になるため、JSONを保存する際は `Set-Content -Encoding UTF8` を指定してください。GoやRustはビルド済みバイナリ、C#はビルド済みDLLを実行すると、ビルド出力をJSONに混ぜずに保存できます。

## Rustの一覧作成とGoの一括検証

対象フォルダを記録し、その一覧を使って検証します。作成・比較はサブフォルダも対象です。一覧と結果JSON以外のファイルが対象フォルダに追加されないよう、結果JSONは別フォルダへ保存してください。

```powershell
cargo build --manifest-path .\rust\Cargo.toml
go build -o .\bin\hash-checker.exe .\go
.\rust\target\debug\hash_checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --create-manifest .\checksums.txt
.\bin\hash-checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --manifest .\checksums.txt --format json | Set-Content -Encoding UTF8 .\verified.json
.\rust\target\debug\hash_checker.exe --algorithm sha256 --workspace .\testdata\ok-sha256 --compare-manifest .\checksums.txt --format json | Set-Content -Encoding UTF8 .\compared.json
```

一覧の入力・保存パスはカレントフォルダ基準です。対象ファイルは `--workspace` 基準です。一覧の保存先が存在する場合は上書きしません。作成先にはNTFS等のhard link対応ファイルシステムが必要で、FAT等では終了コード3になります。

Goの `--quiet` はテキスト表示で正常行と成功概要を抑制します。不一致やエラーは表示し、JSON出力は省略しません。スクリプトやCIでは `go run` が非0終了コードを変換するため、上記のビルド済み実行ファイルを使用してください。

Rust比較では `added`（追加）、`removed`（削除）、`changed`（変更）、`match`（一致）を区別します。Goは一覧に記載された対象を検証し、一覧にない追加ファイルは検査しません。終了条件・一覧の対応範囲は [共通仕様](../共通仕様.md) を参照してください。

## Pythonの結果集計

```powershell
python .\python\summarize_results.py .\verified.json .\compared.json --output .\summary.csv
```

相対パスとアルゴリズムごとに確認回数と各判定の件数を集計します。入力全体のエラーは空のパス行へ集計します。既存CSVは上書きせず、入力の形式・件数・終了コードの整合性を確認してから保存します。1入力は16MiB以下のUTF-8 JSONです。

異なる基準フォルダでも相対パスが同じ結果は同じ行へ集計されます。同じ対象群の結果だけを指定してください。集計プログラムの終了コード0は集計の成功を表し、対象ファイルの一致を意味しません。不一致はCSVの件数で確認します。`--output` を省略すると標準出力へCSVを表示します。

CSVでは `=`、`+`、`-`、`@` から始まる文字列に先頭の `'` を付け、表計算ソフトがファイル名を数式として扱うことを防ぎます。JSON内の元のファイル名は変更しません。

## Windows GUI

.NET 10 SDKを使って起動します。

```powershell
dotnet run --project .\csharp\HashChecker.Gui
```

1. ファイルを選択するか、画面へ1ファイルをドラッグする。
2. SHA256またはMD5を選び、配布元の期待ハッシュ値を貼り付ける。
3. 「ハッシュを確認」を押し、一致・不一致を確認する。
4. 必要なら「結果をコピー」で記録する。処理中は「キャンセル」で中止できる。

GUIはWindows専用で、CLIと検証処理を共有します。GUIの起動にGoやRustは必要ありません。

## 終了コード

| 終了コード | 意味 |
|---:|---|
| 0 | ハッシュ値が一致 |
| 1 | ハッシュ値が不一致 |
| 2 | 入力ファイルの検出エラー |
| 3 | ファイル読み取りエラー |
| 4 | ハッシュ値の形式エラー |
| 5 | ハッシュ値算出エラー、または想定外エラー |
