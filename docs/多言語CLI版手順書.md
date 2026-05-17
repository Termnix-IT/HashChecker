# 多言語 CLI 版 手順書

この手順書は、共通仕様に沿って実装した Python / Go / C#/.NET / Rust 版の利用手順です。

各 CLI 版は、以下の共通入力を扱います。

- `--algorithm`: `md5` または `sha256`
- `--workspace`: ハッシュファイルと確認対象ファイルを自動検出する作業フォルダ
- `--hash-file`: ハッシュファイルを明示する場合のパス
- `--target-file`: 確認対象ファイルを明示する場合のパス

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

## 終了コード

| 終了コード | 意味 |
|---:|---|
| 0 | ハッシュ値が一致 |
| 1 | ハッシュ値が不一致 |
| 2 | 入力ファイルの検出エラー |
| 3 | ファイル読み取りエラー |
| 4 | ハッシュ値の形式エラー |
| 5 | ハッシュ値算出エラー、または想定外エラー |
