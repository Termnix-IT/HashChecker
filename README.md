# ハッシュ値確認ツール

[![Check](https://github.com/Termnix-IT/HashChecker/actions/workflows/check.yml/badge.svg)](https://github.com/Termnix-IT/HashChecker/actions/workflows/check.yml)

ベンダーから提供されたハッシュ値と、確認対象ファイルから算出したハッシュ値を比較するためのツールです。

このリポジトリでは、共通の検証仕様を複数言語で実装し、それぞれを用途別のツールとして発展させています。

## このリポジトリで示すこと

- 実務制約に合わせた PowerShell 版と、共通仕様に沿った CLI 版の設計差
- Python / Go / C#/.NET / Rust で同じ仕様を実装したときの言語ごとの特徴
- 入力検出、ハッシュ値検証、終了コード、エラー表示を言語間で揃える設計
- GitHub Actions と横断確認スクリプトによる仕様差の検出
- Rustで作成した一覧をGoで検証し、Pythonで結果を集計する言語間の受け渡し

## 実装比較

| 実装 | 位置づけ | 主な確認ポイント |
|---|---|---|
| PowerShell | 業務制約版 | Windows 標準機能、`.bat` 起動、結果オブジェクト・CSV保存 |
| Python | 基準実装・結果集計 | 共通JSONの検証、複数結果をCSVへ集計 |
| Go | 配布用CLI・CI検証 | SHA256一覧の一括検証、成功時の出力抑制 |
| C#/.NET | Windows向けGUI | ファイル選択・ドラッグ、貼り付け、進捗・キャンセル |
| Rust | フォルダ状態の記録 | SHA256一覧作成、追加・削除・変更検出 |

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
│  ├─ run_cross_checks.py
│  └─ test_powershell.ps1
├─ csharp/
│  ├─ README.md
│  ├─ HashChecker.Core/
│  ├─ HashChecker.Gui/
│  ├─ HashChecker.Tests/
│  └─ HashChecker/
│     ├─ HashChecker.csproj
│     └─ Program.cs
├─ rust/
│  ├─ Cargo.lock
│  ├─ Cargo.toml
│  ├─ README.md
│  ├─ src/
│  │  ├─ main.rs
│  │  └─ manifest.rs
│  └─ tests/
│     └─ cli.rs
├─ powershell/
│  ├─ Run-MD5Check.bat
│  ├─ Run-SHA256Check.bat
│  ├─ HashChecker.Common.ps1
│  ├─ Verify-MD5Hash.ps1
│  └─ Verify-SHA256Hash.ps1
├─ go/
│  ├─ README.md
│  ├─ hash_checker.go
│  ├─ hash_checker_test.go
│  └─ manifest_test.go
├─ python/
│  ├─ README.md
│  ├─ hash_checker.py
│  ├─ summarize_results.py
│  ├─ test_hash_checker.py
│  └─ test_reports.py
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
- CLIの直接入力、JSON結果、バージョン・対応機能表示

独自機能は全言語へ追加せず、対応する実装同士で入出力形式を揃えます。共通機能と受け渡し形式の境界は [共通仕様](./共通仕様.md)、用途別の実行方法は [多言語CLI版手順書](./docs/多言語CLI版手順書.md) を参照してください。

## 実装順

1. Python
2. Go
3. C#/.NET
4. Rust

共通CLIはPython版を基準に揃え、用途別機能は上の担当に沿って実装します。

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

.NET 10 の共通処理をCLIとWindows向けWPF GUIで共有します。GUIは以下で起動できます。

```powershell
dotnet run --project .\csharp\HashChecker.Gui
```

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

共通処理のテストとWindows GUIの確認:

```powershell
dotnet run --project .\csharp\HashChecker.Tests
dotnet build .\csharp\HashChecker.Gui
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\test_powershell.ps1
```

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

この横断確認では、同じ `testdata` に対する終了コードとJSON内容、直接入力、エラー、対応機能、Rust → Go → Pythonの受け渡しを確認します。

GitHub Actionsでは `push` と `pull_request` のたびにUbuntuとWindowsで個別テスト・ビルド・横断確認を実行します。WindowsではPowerShell 5.1とWPF画面のスモーク確認も行い、GUIのPNGを成果物に残します。

## ドキュメント

- [共通仕様](./共通仕様.md)
- [操作手順書](./docs/HashChecker手順書.md)
- [PowerShell 版 手順書](./docs/PowerShell版手順書.md)
- [多言語 CLI 版 手順書](./docs/多言語CLI版手順書.md)
- [多言語版 比較メモ](./docs/比較メモ.md)
