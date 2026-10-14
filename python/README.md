# Python 版 HashChecker

Python 標準ライブラリだけで実装した基準CLIと結果集計ツールです。

`summarize_results.py` で共通JSON結果をCSVへ集計できます。直接入力、JSON保存、集計の利用方法は [多言語CLI版手順書](../docs/多言語CLI版手順書.md) を参照してください。

## 実行例

作業フォルダ内の `.txt` ファイルと確認対象ファイルを自動検出する場合:

```powershell
python .\python\hash_checker.py --algorithm sha256 --workspace .\testdata\ok-sha256
```

ハッシュファイルと確認対象ファイルを明示する場合:

```powershell
python .\python\hash_checker.py --algorithm md5 --hash-file .\vendor_hash.txt --target-file .\firmware.bin
```

## 対応アルゴリズム

- `md5`
- `sha256`

## 終了コード

終了コードは [共通仕様](../共通仕様.md) に従います。

