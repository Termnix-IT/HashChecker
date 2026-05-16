# Go 版 HashChecker

Go 標準ライブラリだけで実装したハッシュ値確認ツールです。

## 実行例

SHA256:

```powershell
go run .\go --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
go run .\go --algorithm md5 --workspace .\testdata\ok-md5
```

ハッシュファイルと確認対象ファイルを明示する場合:

```powershell
go run .\go --algorithm md5 --hash-file .\testdata\ok-md5\vendor_hash.txt --target-file .\testdata\ok-md5\firmware.bin
```

## ビルド例

```powershell
go build -o .\bin\hash-checker.exe .\go
```

## テスト

```powershell
go test .\go
```

## 補足

- 対応アルゴリズムは `md5` と `sha256` です。
- 終了コードは `共通仕様.md` に合わせています。
- 自動検出時は `.txt`, `.log`, `.bat`, `.cmd`, `.ps1`, `.py`, `.go`, `.exe` を確認対象ファイルから除外します。
