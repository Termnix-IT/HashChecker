# Rust版：フォルダ記録と変更検出

共通CLIに加え、フォルダ配下の通常ファイルからSHA256一覧を作成し、後で追加・削除・内容変更を検出します。ハッシュ計算には`md-5`・`sha2`、JSON出力には`serde_json`を使います。

操作方法は [多言語CLI・GUI版手順書](../docs/多言語CLI版手順書.md)、一覧形式・終了コードは [共通仕様](../共通仕様.md) を参照してください。

## 開発環境とビルド

Rust stableとCargoを使います。editionは2021で、最低Rustバージョンは`Cargo.toml`に定義していません。依存関係の解決内容は`Cargo.lock`で管理します。

リポジトリのルートからビルドします。

```powershell
cargo build --manifest-path .\rust\Cargo.toml --release
```

Windowsでは`.\rust\target\release\hash_checker.exe`、Ubuntuでは`./rust/target/release/hash_checker`が実行入口です。ビルド済みバイナリの利用者側にRust環境は不要ですが、配布先のOS・CPUに対応するものを用意してください。

## 個別テストと静的確認

```powershell
cargo test --manifest-path .\rust\Cargo.toml
cargo fmt --manifest-path .\rust\Cargo.toml -- --check
cargo clippy --manifest-path .\rust\Cargo.toml --all-targets -- -D warnings
```

単体・CLIテストで、一覧の作成・再検証、追加・削除・変更、UTF-8/BOM、パス・重複・リンク、保存先保護、JSON結果を確認します。fmtとclippyには、Rustツールチェーンの`rustfmt`・`clippy`コンポーネントが必要です。

## 保存と比較の設計

一覧は全ファイルのハッシュを計算し、一時ファイルへの書込み完了後に公開します。保存先がすでに存在する場合は上書きしません。この公開方式にはhard link対応のファイルシステムが必要で、NTFS等を使います。FAT等の保存先では終了コード3になります。

フォルダ比較は時刻・サイズだけで判定せず、内容のSHA256を計算します。一覧自身は対象から除外し、拡張子による暗黙の除外は行いません。並列化や他言語との速度差は、この実装の性能保証には含めていません。

言語間の受け渡しは [横断確認](../README.md#開発時の検証) に含まれます。自動リリースやインストーラーの生成は行いません。
