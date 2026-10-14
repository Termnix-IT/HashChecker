# Go版：配布用CLIとCI向け一覧検証

共通CLIに加え、SHA256一覧に記載されたファイルを一括検証します。`--quiet`でテキストの成功出力を抑制でき、CIや他のスクリプトでは終了コードと共通JSONを利用できます。標準ライブラリのみを使用します。

操作方法は [多言語CLI・GUI版手順書](../docs/多言語CLI版手順書.md)、一覧・JSON・終了コードの契約は [共通仕様](../共通仕様.md) を参照してください。

## 開発環境とビルド

開発には`go.mod`で指定するGo 1.22以上を使います。次のコマンドはWindowsのPowerShellで、リポジトリのルートから実行します。

```powershell
New-Item -ItemType Directory -Force -Path .\bin | Out-Null
go build -o .\bin\hash-checker.exe .\go
```

Windowsでは`.\bin\hash-checker.exe`が実行入口です。Ubuntuでは`go build -o ./bin/hash-checker ./go`でビルドし、`./bin/hash-checker`を実行します。ビルド済みバイナリの利用者側にGo環境は不要ですが、配布先のOS・CPUに対応するものを用意してください。

`go run`は開発時に利用できますが、検証の非0終了コードをそのまま返しません。CI・自動処理ではビルド済みバイナリを使ってください。

## 個別テスト

```powershell
go test .\go
```

単体検証に加え、一覧形式、UTF-8/BOM、相対パス、重複、リンク・Windowsのreparse point、欠落・不一致の混在、JSON・quietを確認します。Windowsの実symlinkテストは作成権限によりスキップされる場合があります。junctionを使う言語間確認は [横断確認](../README.md#開発時の検証) に含まれます。

## 用途の境界

一覧検証は一覧の順に実行します。一覧にない追加ファイルの検出はRustのフォルダ比較が担当します。一覧の新規作成やCSV集計は行わず、Rust・Pythonと共通形式で受け渡します。

単一バイナリをローカルで生成する手順はありますが、自動リリースやインストーラーの生成は行いません。
