# HashChecker 操作手順の入口

使用する用途に合わせて、次の手順を選んでください。各手順書のコマンド例は、リポジトリのルートからPowerShellで実行する前提です。

| 用途 | 手順 |
|---|---|
| 追加インストールせず、Windows標準機能で1ファイルを確認する | [PowerShellの`.bat`起動](PowerShell版手順書.md#batファイルで確認する) |
| Windowsの画面でファイルを選択して確認する | [C#のWindows GUI](多言語CLI版手順書.md#windows-guiで確認する) |
| Python / Go / C# / RustのCLIで1ファイルを確認する | [共通CLIの単体検証](多言語CLI版手順書.md#単体ファイルを確認する) |
| Webサイト等からコピーした期待値を使う | [直接入力](多言語CLI版手順書.md#期待値を直接入力する) |
| フォルダを記録し、一括検証・変更検出を行う | [RustとGoの連携](多言語CLI版手順書.md#フォルダを記録して検証と比較を行う) |
| 複数の検証結果をCSVへ集計する | [Pythonの結果集計](多言語CLI版手順書.md#結果をcsvへ集計する) |
| PowerShellの結果をオブジェクトやCSVで記録する | [PowerShellの結果保存](PowerShell版手順書.md#結果を記録する) |

ビルド・配布形式・個別テストは [Python](../python/README.md)、[Go](../go/README.md)、[C#/.NET](../csharp/README.md)、[Rust](../rust/README.md) のREADMEを参照してください。

引数、パスの基準、JSON形式、終了コードを確認する場合は [共通仕様](../共通仕様.md) を参照してください。どの言語を使うかの判断理由は [比較メモ](比較メモ.md) にまとめています。
