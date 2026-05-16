# Rust 版 HashChecker

Rust で実装したハッシュ値確認ツールです。

ハッシュ計算には `md-5` と `sha2` crate を使用し、CLI 引数解析とファイル検出は標準ライブラリで実装しています。

## 実行例

SHA256:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm sha256 --workspace .\testdata\ok-sha256
```

MD5:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm md5 --workspace .\testdata\ok-md5
```

ハッシュファイルと確認対象ファイルを明示する場合:

```powershell
cargo run --manifest-path .\rust\Cargo.toml -- --algorithm md5 --hash-file .\testdata\ok-md5\vendor_hash.txt --target-file .\testdata\ok-md5\firmware.bin
```

## ビルド例

```powershell
cargo build --manifest-path .\rust\Cargo.toml --release
```

Windows 向けの実行ファイルは以下に出力されます。

```text
rust\target\release\hash_checker.exe
```

## 補足

- 対応アルゴリズムは `md5` と `sha256` です。
- 終了コードは `共通仕様.md` に合わせています。
- 自動検出時は `.txt`, `.log`, `.bat`, `.cmd`, `.ps1`, `.py`, `.go`, `.rs`, `.toml`, `.lock`, `.exe` などを確認対象ファイルから除外します。
