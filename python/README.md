# Python版：基準CLIと結果集計

単体検証の基準実装と、複数の共通JSON結果をCSVへ集計するツールです。ハッシュ計算・引数解析・JSON・CSV・テストはPython標準ライブラリで扱い、追加パッケージは不要です。

操作方法は [多言語CLI・GUI版手順書](../docs/多言語CLI版手順書.md)、入出力の契約は [共通仕様](../共通仕様.md) を参照してください。

## 開発・実行環境

Python 3を使います。Python 3.12と3.14で動作を確認し、CIでは`3.x`を使用しています。Python自体をインストールしている環境で、リポジトリのルートから実行してください。

| ファイル | 役割 |
|---|---|
| `hash_checker.py` | 共通CLIの単体検証とJSON出力 |
| `summarize_results.py` | JSON形式・件数・判定の整合性を確認してCSV集計 |
| `test_hash_checker.py`・`test_reports.py` | 検証、集計、保存先保護、CSVテキスト化、出力文字コードの確認 |

## 個別テスト

```powershell
$env:PYTHONUTF8 = '1'
python -m unittest discover -s python
```

CLIの標準出力・標準エラーはUTF-8です。テストが直接呼び出す処理の日本語表示も揃えるため、上記の環境変数を指定します。

## 配布・組み込み

この実装はスクリプト配布です。利用者にはPythonの実行環境が必要です。exe化やインストーラー生成はリポジトリに実装していません。

Pythonから利用する場合は、`hash_checker.run(argv)`で検証し、`summarize_results.aggregate(paths)`で集計行を取得できます。テキストモードの検証・集計入力エラーは`HashCheckerError`で通知し、`exit_code`で種類を確認できます。JSONモードの検証はエラーも結果JSONへ出力して終了コードを返します。

集計の終了コード0は「CSV集計が成功した」という意味です。元の検証結果に不一致があっても、正しい形式なら集計できます。入力上限、CSV列、ファイル名のテキスト化は [結果集計の手順](../docs/多言語CLI版手順書.md#結果をcsvへ集計する) を参照してください。

横断確認は [全体README](../README.md#開発時の検証) にまとめています。
