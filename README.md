# ハッシュ値確認ツール

ベンダーから提供されたハッシュ値と、確認対象ファイルから算出したハッシュ値を比較するためのツールです。

このリポジトリでは、業務制約版の PowerShell 実装を残しながら、学習・ポートフォリオ目的で複数言語版を作成します。

## フォルダ構成

```text
HashChecker/
├─ README.md
├─ go.mod
├─ 共通仕様.md
├─ docs/
│  ├─ HashChecker手順書.md
│  └─ 比較メモ.md
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

## テスト

Python 版のテスト:

```powershell
python -m unittest discover -s python
```

Go 版のテスト:

```powershell
go test .\go
```

## ドキュメント

- [共通仕様](./共通仕様.md)
- [操作手順書](./docs/HashChecker手順書.md)
- [多言語版 比較メモ](./docs/比較メモ.md)
