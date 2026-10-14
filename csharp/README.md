# C#/.NET 版 HashChecker

.NET 10 のCLIとWindows向けWPF GUIです。`HashChecker.Core` の検証処理を両方で共有し、追加の外部ライブラリは使用しません。

GUIは `dotnet run --project .\csharp\HashChecker.Gui` で起動します。画面操作と共通CLIの利用方法は [多言語CLI版手順書](../docs/多言語CLI版手順書.md) を参照してください。共有処理のテストは `dotnet run --project .\csharp\HashChecker.Tests` で実行できます。

## 実行例

SHA256:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm md5 --workspace .\testdata\ok-md5
```

ハッシュファイルと確認対象ファイルを明示する場合:

```powershell
dotnet run --project .\csharp\HashChecker -- --algorithm md5 --hash-file .\testdata\ok-md5\vendor_hash.txt --target-file .\testdata\ok-md5\firmware.bin
```

## ビルド例

```powershell
dotnet build .\csharp\HashChecker
```

単一ファイルに近い形で Windows x64 向けに発行する例:

```powershell
dotnet publish .\csharp\HashChecker -c Release -r win-x64 --self-contained false -p:PublishSingleFile=true -o .\bin\csharp
```

## 補足

- 対応アルゴリズムは `md5` と `sha256` です。
- 終了コードは `共通仕様.md` に合わせています。
- 自動検出時は `.txt`, `.log`, `.bat`, `.cmd`, `.ps1`, `.py`, `.go`, `.cs`, `.csproj`, `.exe`, `.dll`, `.pdb` などを確認対象ファイルから除外します。
