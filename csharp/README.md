# C#/.NET 版 HashChecker

.NET 標準ライブラリだけで実装したハッシュ値確認ツールです。

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
