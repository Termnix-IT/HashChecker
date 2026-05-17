# ハッシュ値確認ツール

[![Check](https://github.com/Termnix-IT/HashChecker/actions/workflows/check.yml/badge.svg)](https://github.com/Termnix-IT/HashChecker/actions/workflows/check.yml)

ベンダーから提供されたハッシュ値と、確認対象ファイルから算出したハッシュ値を比較するためのツールです。

このリポジトリでは、業務制約版の PowerShell 実装を残しながら、学習・ポートフォリオ目的で複数言語版を作成します。

## このリポジトリで示すこと

- 実務制約に合わせた PowerShell 版と、共通仕様に沿った CLI 版の設計差
- Python / Go / C#/.NET / Rust で同じ仕様を実装したときの言語ごとの特徴
- 入力検出、ハッシュ値検証、終了コード、エラー表示を言語間で揃える設計
- GitHub Actions と横断確認スクリプトによる仕様差の検出

## 実装比較

| 実装 | 位置づけ | 主な確認ポイント |
|---|---|---|
| PowerShell | 業務制約版 | Windows 標準機能、`.bat` 起動、手順化 |
| Python | 基準実装 | 仕様整理、入力検証、テスト容易性 |
| Go | 単一バイナリ配布の比較 | 標準ライブラリ、明示的なエラー処理、配布性 |
| C#/.NET | Windows 業務ツール寄りの比較 | .NET 標準 API、Windows 配布、将来的な GUI 化 |
| Rust | 型安全性と配布性の比較 | `Result` ベースのエラー処理、依存関係選定、単一バイナリ |

## フォルダ構成

```text
HashChecker/
├─ README.md
├─ go.mod
├─ 共通仕様.md
├─ docs/
│  ├─ HashChecker手順書.md
│  ├─ PowerShell版手順書.md
│  ├─ 多言語CLI版手順書.md
│  └─ 比較メモ.md
├─ scripts/
│  └─ run_cross_checks.py
├─ csharp/
│  ├─ README.md
│  └─ HashChecker/
│     ├─ HashChecker.csproj
│     └─ Program.cs
├─ rust/
│  ├─ Cargo.lock
│  ├─ Cargo.toml
│  ├─ README.md
│  └─ src/
│     └─ main.rs
├─ powershell/
│  ├─ Run-MD5Check.bat
│  ├─ Run-SHA256Check.bat
│  ├─ Verify-MD5Hash.ps1
│  └─ Verify-SHA256Hash.ps1
├─ go/
│  ├─ README.md
│  ├─ hash_checker.go
│  └─ hash_checker_test.go
├─ python/
│  ├─ README.md
│  ├─ hash_checker.py
│  └─ test_hash_checker.py
└─ testdata/
   ├─ ok-md5/
   ├─ ok-sha256/
   ├─ mismatch/
   ├─ invalid-hash/
   └─ multiple-targets/
```

## 共通仕様

各言語版は [共通仕様](./共通仕様.md) に従って実装します。

主な仕様は以下です。

- 対応アルゴリズムは MD5 と SHA256
- ハッシュ値はテキストファイルから読み込む
- 空行は無視する
- 空行を除いて複数行ある場合はエラー
- 大文字・小文字は区別しない
- ハッシュ値の文字数と16進数形式を検証する
- 終了コードを言語間で揃える

## 実装順

1. Python
2. Go
3. C#/.NET
4. Rust

最初に Python 版を基準実装として作成し、その後に Go、C#/.NET、Rust で同じ仕様を実装します。

## PowerShell 版

業務制約版として、Windows PowerShell の `Get-FileHash` を使用します。

MD5確認:

```powershell
.\powershell\Run-MD5Check.bat
```

SHA256確認:

```powershell
.\powershell\Run-SHA256Check.bat
```

PowerShell 版は `.bat`、`.ps1`、ハッシュファイル、確認対象ファイルを同じフォルダに置いて使う想定です。既存どおり使う場合は、`powershell/` フォルダ内に `vendor_hash.txt` と確認対象ファイルを配置してください。

## Python 版

Python 標準ライブラリだけで実装した CLI 版です。

SHA256 の正常系サンプル:

```powershell
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5 の正常系サンプル:

```powershell
python .\python\hash_checker.py --algorithm md5 --workspace .\testdata\ok-md5
```

ハッシュファイルと確認対象ファイルを明示する場合:

```powershell
python .\python\hash_checker.py --algorithm md5 --hash-file .\testdata\ok-md5\vendor_hash.txt --target-file .\testdata\ok-md5\firmware.bin
```

## Go 版

Go 標準ライブラリだけで実装した CLI 版です。単一バイナリ配布を想定した比較対象です。

SHA256 の正常系サンプル:

```powershell
go run .\go --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5 の正常系サンプル:

```powershell
go run .\go --algorithm md5 --workspace .\testdata\ok-md5
```

ビルド例:

```powershell
go build -o .\bin\hash-checker.exe .\go
```

## C#/.NET 版

.NET 標準ライブラリだけで実装した CLI 版です。Windows 業務ツール寄りの配布や将来的な GUI 化を比較するための実装です。

SHA256 の正常系サンプル:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5 の正常系サンプル:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm md5 --workspace .\testdata\ok-md5
```

ビルド例:

```powershell
dotnet build .\csharp\HashChecker
```

## Rust 版

Rust で実装した CLI 版です。単一バイナリ配布、型安全性、依存関係の選定を比較するための実装です。

SHA256 の正常系サンプル:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5 の正常系サンプル:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm md5 --workspace .\testdata\ok-md5
```

ビルド例:

```powershell
cargo build --manifest-path .\rust\Cargo.toml --release
```

## テスト

Python 版のテスト:

```powershell
python -m unittest discover -s python
```

Go 版のテスト:

```powershell
go test .\go
```

C#/.NET 版の確認:

```powershell
dotnet build .\csharp\HashChecker
dotnet run --project .\csharp\HashChecker -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

ローカル確認では .NET 8 Runtime を入れた環境で、C#/.NET 版の build と横断確認が通ることを確認しています。

Rust 版の確認:

```powershell
cargo test --manifest-path .\rust\Cargo.toml
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

言語横断の確認:

```powershell
python .\scripts\run_cross_checks.py
```

特定の言語だけ確認する場合:

```powershell
python .\scripts\run_cross_checks.py --language rust
```

この横断確認では、Python / Go / C#/.NET / Rust 版が同じ `testdata` に対して共通仕様どおりの終了コードを返すかを確認します。

ローカル環境と GitHub Actions の両方で、Python / Go / C#/.NET / Rust 版の横断確認が通ることを確認しています。

GitHub Actions では `push` と `pull_request` のたびに、各言語版の個別テスト・ビルドと横断確認を実行します。

## ドキュメント

- [共通仕様](./共通仕様.md)
- [操作手順書](./docs/HashChecker手順書.md)
- [PowerShell 版 手順書](./docs/PowerShell版手順書.md)
- [多言語 CLI 版 手順書](./docs/多言語CLI版手順書.md)
- [多言語版 比較メモ](./docs/比較メモ.md)
