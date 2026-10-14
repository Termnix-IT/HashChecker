# HashChecker

[![Check](https://github.com/Termnix-IT/HashChecker/actions/workflows/check.yml/badge.svg)](https://github.com/Termnix-IT/HashChecker/actions/workflows/check.yml)

配布元のハッシュ値とファイルから計算した値を比較するツールです。MD5とSHA256に対応しています。

共通の検証仕様をPython / Go / C# / Rustで実装し、各言語を用途別に発展させています。PowerShellは、Windows標準機能で使う業務版として維持しています。

## 用途で選ぶ

| やりたいこと | 実装 | 主な機能 | 開発・ビルド情報 |
|---|---|---|---|
| Windows標準機能で確認・記録する | PowerShell | `.bat`起動、結果オブジェクト、CSV保存 | [PowerShell版手順書](docs/PowerShell版手順書.md) |
| 検証結果を加工・集計する | Python | 単体検証、複数JSON結果のCSV集計 | [Python版](python/README.md) |
| スクリプトやCIで一括検証する | Go | SHA256一覧の検証、成功時の出力抑制 | [Go版](go/README.md) |
| ファイル選択や貼り付けで確認する | C#/.NET | Windows GUI、進捗、キャンセル、結果コピー | [C#/.NET版](csharp/README.md) |
| フォルダの状態を記録・比較する | Rust | SHA256一覧作成、追加・削除・変更検出 | [Rust版](rust/README.md) |

Python / Go / C# / RustのCLIは、単体検証、期待値の直接入力、共通JSON結果、バージョン・対応機能表示を共有します。用途別機能の実装は全言語に義務づけず、対応する実装間で入出力形式を揃えます。

例えば、Rustで一覧を作成し、Goで検証して、Pythonで結果を集計できます。Goは一覧に記載されたファイルを確認し、Rustはフォルダに追加されたファイルも検出します。

## まず試す

以下のコマンドはリポジトリのルートからPowerShellで実行します。Pythonを利用できる環境なら、同梱サンプルで単体検証を試せます。

```powershell
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256
```

`[正常]`の表示と終了コード0が一致を示します。GUIや一括検証を試す場合は、[操作手順の入口](docs/HashChecker手順書.md)から用途を選んでください。

使用する実装に合わせて実行・ビルド環境を用意してください。現行コードを試す手順はソースからの実行・ビルドを前提とし、インストーラーやリリース成果物の作成を伴いません。

## ドキュメントの役割

| 文書 | 内容 |
|---|---|
| [操作手順の入口](docs/HashChecker手順書.md) | 用途に合う操作手順へ案内 |
| [多言語CLI・GUI版手順書](docs/多言語CLI版手順書.md) | 単体検証、JSON保存、Rust→Go→Pythonの連携、GUI操作 |
| [PowerShell版手順書](docs/PowerShell版手順書.md) | `.bat`での確認、パラメーター、結果オブジェクト、CSV保存 |
| [共通仕様](共通仕様.md) | 引数・パス・終了コード・JSON・チェックサム一覧の契約 |
| [比較メモ](docs/比較メモ.md) | 用途分担、設計理由、依存関係、保守上の判断 |
| 言語別README | その実装の開発環境、ビルド、配布形式、個別テスト |
| [検証用データ](testdata/README.md) | 固定サンプルの目的、期待結果、バイト列の保持 |

## リポジトリの構成

| 場所 | 役割 |
|---|---|
| `powershell/` | 固定アルゴリズムの起動スクリプトと共通処理 |
| `python/` | 基準CLI、JSON結果の集計、単体テスト |
| `go/` | 配布用CLI、単体・一覧検証のテスト |
| `csharp/HashChecker/` | 共通CLI |
| `csharp/HashChecker.Core/` | CLIとGUIが共有する検証処理 |
| `csharp/HashChecker.Gui/` | Windows向けWPF画面 |
| `csharp/HashChecker.Tests/` | 共有処理のテスト |
| `rust/src/`・`rust/tests/` | CLI、フォルダ記録・比較、単体・CLIテスト |
| `scripts/` | 言語横断確認とPowerShell確認 |
| `docs/`・`testdata/` | 手順・設計説明と検証用データ |

## 開発時の検証

個別のビルド・テストは言語別READMEを参照してください。4言語の開発環境を用意すると、共通CLIと受け渡しをまとめて確認できます。

```powershell
$env:PYTHONUTF8 = '1'
python .\scripts\run_cross_checks.py
```

この確認は各CLIをビルドし、[固定サンプル](testdata/README.md)の終了コードとJSON内容、直接入力、エラー、バージョン・対応機能を検査します。GoとRustを選択した場合は、Rustの一覧作成→Goの検証→Pythonの集計も確認します。

特定の実装だけを確認する場合は、`--language`を指定します。複数指定もできます。

```powershell
python .\scripts\run_cross_checks.py --language go --language rust
```

`--allow-missing`は不足する開発環境をスキップするローカル用の指定です。未実施の言語があっても終了コード0になり得るため、`MISSING`の表示で確認範囲を把握してください。CIではこの指定を使用しません。

[GitHub Actions](.github/workflows/check.yml)はUbuntuとWindowsで個別テスト・ビルド・横断確認を実行します。WindowsではPowerShell 5.1とWPF画面の確認も行い、`windows-gui-preview`という名前で画面のPNGを保存します。

## 過去のプレリリース資料

[v0.1.0-pre.1のリリースノート](docs/リリースノート-v0.1.0-pre.1.md)と[公開手順の記録](docs/プレリリース作業メモ-v0.1.0-pre.1.md)は、初期のCLI比較版に関する資料です。現在の操作・機能は上記の手順書と共通仕様を参照してください。
