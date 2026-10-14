# C#/.NET版：Windows GUIと共通検証処理

.NET 10のCLIとWindows専用のWPF GUIです。ファイル選択・ドラッグ、期待値の貼り付け、進捗・キャンセル、結果コピーを提供します。検証処理はCLIとGUIで共有し、追加の外部ライブラリは使用しません。

画面操作・CLIの利用方法は [多言語CLI・GUI版手順書](../docs/多言語CLI版手順書.md)、共通CLIの契約は [共通仕様](../共通仕様.md) を参照してください。

## 開発環境と構成

開発には.NET 10 SDKを使います。CLIと共有処理はWindows・UbuntuのCIで確認し、GUIはWindowsでビルド・実行します。

| プロジェクト | 役割 |
|---|---|
| `HashChecker` | `net10.0`の共通CLI |
| `HashChecker.Core` | 期待値検証、ハッシュ計算、進捗・キャンセル、JSON結果 |
| `HashChecker.Gui` | `net10.0-windows`のWPF画面 |
| `HashChecker.Tests` | 共有処理のテスト。外部テストパッケージは不要 |

## ビルドと個別テスト

リポジトリのルートから実行します。

```powershell
dotnet build .\csharp\HashChecker
dotnet run --project .\csharp\HashChecker.Tests
```

WindowsでGUIをビルドする場合:

```powershell
dotnet build .\csharp\HashChecker.Gui
```

GUIのスモーク確認は、入力、一致・不一致、エラー、キャンセル後の復帰、結果コピーの有効状態を確認し、PNGを出力します。実際のOSファイル選択ダイアログ操作とクリップボード書込みは自動確認の対象外です。

```powershell
New-Item -ItemType Directory -Force -Path .\bin | Out-Null
$previewPath = Join-Path (Get-Location) 'bin\gui-smoke.png'
$process = Start-Process -FilePath '.\csharp\HashChecker.Gui\bin\Debug\net10.0-windows\HashChecker.Gui.exe' -ArgumentList @('--smoke', ('"' + $previewPath + '"')) -WindowStyle Hidden -PassThru
if (-not $process.WaitForExit(30000)) { $process.Kill(); throw 'GUI確認が時間内に完了しませんでした。' }
if ($process.ExitCode -ne 0) { throw 'GUI確認に失敗しました。' }
```

CIでも同じ確認を行います。[全体の検証](../README.md#開発時の検証)を参照してください。

## Windows向けの発行

CLIをランタイム別の単一ファイルとして発行する例:

```powershell
dotnet publish .\csharp\HashChecker -c Release -r win-x64 --self-contained false -p:PublishSingleFile=true -o .\bin\csharp-cli
```

GUIを同じ方式で発行する例:

```powershell
dotnet publish .\csharp\HashChecker.Gui -c Release -r win-x64 --self-contained false -p:PublishSingleFile=true -o .\bin\csharp-gui
```

この指定はランタイムを同梱しません。配布先にはCLI用の.NET 10 Runtime、GUI用の.NET 10 Desktop Runtimeが必要です。発行先フォルダ内の必要なファイルはまとめて配布してください。発行はローカルのビルド操作で、GitHub Releaseへの公開ではありません。
