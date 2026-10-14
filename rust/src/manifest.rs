use super::{calculate_file_hash, find_algorithm, item, make_report, HashCheckerError};
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{self, Metadata, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

fn failure(message: &str, code: i32) -> HashCheckerError {
    HashCheckerError::new(message, code)
}

fn is_link(metadata: &Metadata) -> bool {
    if metadata.file_type().is_symlink() {
        return true;
    }
    #[cfg(windows)]
    {
        use std::os::windows::fs::MetadataExt;
        if metadata.file_attributes() & 0x400 != 0 {
            return true;
        }
    }
    false
}

// Check every existing component before canonicalizing: otherwise a link in an
// ancestor would disappear from the canonical path and bypass the policy.
fn checked_path(path: &Path, allow_missing_leaf: bool) -> Result<PathBuf, HashCheckerError> {
    let mut current = PathBuf::new();
    for component in path.components() {
        current.push(component);
        match fs::symlink_metadata(&current) {
            Ok(metadata) if is_link(&metadata) => {
                return Err(failure("リンクまたは reparse point は使用できません。", 3));
            }
            Ok(_) => {}
            Err(error)
                if allow_missing_leaf
                    && current == path
                    && error.kind() == std::io::ErrorKind::NotFound => {}
            Err(_) => return Err(failure("一覧または対象のパスを読み取れません。", 3)),
        }
    }
    if allow_missing_leaf {
        let parent = path
            .parent()
            .ok_or_else(|| failure("保存先が不正です。", 2))?;
        let parent =
            fs::canonicalize(parent).map_err(|_| failure("保存先フォルダを読み取れません。", 3))?;
        Ok(parent.join(
            path.file_name()
                .ok_or_else(|| failure("保存先が不正です。", 2))?,
        ))
    } else {
        fs::canonicalize(path).map_err(|_| failure("対象パスを読み取れません。", 3))
    }
}

fn valid_relative(path: &str) -> bool {
    !path.is_empty()
        && !path
            .chars()
            .any(|c| c == ':' || c == '\\' || c.is_control())
        && path
            .split('/')
            .all(|part| !part.is_empty() && part != "." && part != "..")
}

fn parse_manifest(text: &str) -> Result<BTreeMap<String, String>, HashCheckerError> {
    let text = text.strip_prefix('\u{feff}').unwrap_or(text);
    let mut entries = BTreeMap::new();
    let mut folded_paths = BTreeSet::new();
    for line in text.lines() {
        if line.trim().is_empty() {
            continue;
        }
        let bytes = line.as_bytes();
        if bytes.len() < 67
            || !bytes[..64].iter().all(u8::is_ascii_hexdigit)
            || bytes[64] != b' '
            || (bytes[65] != b'*' && bytes[65] != b' ')
        {
            return Err(failure("一覧の行形式が不正です。", 4));
        }
        let path = &line[66..];
        if !valid_relative(path) || !folded_paths.insert(path.to_ascii_lowercase()) {
            return Err(failure("一覧の相対パスが不正、または重複しています。", 4));
        }
        entries.insert(path.to_string(), line[..64].to_ascii_lowercase());
    }
    if entries.is_empty() {
        return Err(failure("一覧が空です。", 4));
    }
    Ok(entries)
}

fn scan(root: &Path, excluded: &Path) -> Result<BTreeMap<String, PathBuf>, HashCheckerError> {
    fn walk(
        root: &Path,
        directory: &Path,
        excluded: &Path,
        files: &mut BTreeMap<String, PathBuf>,
        folded: &mut BTreeSet<String>,
    ) -> Result<(), HashCheckerError> {
        let entries =
            fs::read_dir(directory).map_err(|_| failure("対象フォルダを読み取れません。", 3))?;
        for entry in entries {
            let entry = entry.map_err(|_| failure("対象フォルダの項目を読み取れません。", 3))?;
            let path = entry.path();
            let metadata = fs::symlink_metadata(&path)
                .map_err(|_| failure("対象の状態を読み取れません。", 3))?;
            if is_link(&metadata) {
                return Err(failure(
                    "対象にリンクまたは reparse point が含まれています。",
                    3,
                ));
            }
            if path == excluded {
                continue;
            }
            if metadata.is_dir() {
                walk(root, &path, excluded, files, folded)?;
            } else if metadata.is_file() {
                let relative = path.strip_prefix(root).unwrap();
                let parts: Option<Vec<&str>> = relative.iter().map(|part| part.to_str()).collect();
                let name = parts
                    .ok_or_else(|| failure("UTF-8 で表せないファイル名です。", 4))?
                    .join("/");
                if !valid_relative(&name) || !folded.insert(name.to_ascii_lowercase()) {
                    return Err(failure("対象の相対パスが不正、または重複しています。", 4));
                }
                files.insert(name, path);
            } else {
                return Err(failure("通常ファイル以外の対象は使用できません。", 3));
            }
        }
        Ok(())
    }
    let mut files = BTreeMap::new();
    walk(root, root, excluded, &mut files, &mut BTreeSet::new())?;
    Ok(files)
}

fn hash(path: &Path) -> Result<String, HashCheckerError> {
    // Ordinary file open/read errors are read failures for manifest operations.
    // The existing single-file CLI retains its calculation error code.
    calculate_file_hash(path, find_algorithm("sha256").unwrap())
        .map_err(|_| failure("対象ファイルを読み取れません。", 3))
}

pub(super) fn create(workspace: &Path, destination: &Path) -> Result<Value, HashCheckerError> {
    if fs::symlink_metadata(destination).is_ok() {
        return Err(failure("保存先が存在します。上書きは行いません。", 2));
    }
    let root = checked_path(workspace, false)?;
    let destination = checked_path(destination, true)?;
    let files = scan(&root, &destination)?;
    if files.is_empty() {
        return Err(failure("一覧に記録するファイルがありません。", 4));
    }
    let mut results = vec![];
    let mut content = String::new();
    for (name, path) in files {
        let actual = hash(&path)?;
        content.push_str(&format!("{actual} *{name}\n"));
        results.push(item(&name, "match", None, Some(&actual), None));
    }
    // Hard-link publication is atomic and refuses an existing destination.
    // Write/sync a private sibling first; remove it on every completion path.
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let temporary = destination
        .parent()
        .unwrap()
        .join(format!(".hash-checker-{}-{nanos}.tmp", std::process::id()));
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&temporary)
        .map_err(|_| failure("一覧の一時保存先を作成できません。", 3))?;
    let write_result = file
        .write_all(content.as_bytes())
        .and_then(|_| file.sync_all());
    drop(file);
    let result = match write_result {
        Err(_) => Err(failure("一覧を保存できません。", 3)),
        Ok(_) => fs::hard_link(&temporary, &destination).map_err(|_| {
            failure(
                "一覧を保存できません。保存先の存在やファイルシステムを確認してください。",
                3,
            )
        }),
    };
    let _ = fs::remove_file(&temporary);
    result?;
    Ok(make_report("sha256", "create", results, 0, None))
}

pub(super) fn compare(workspace: &Path, manifest: &Path) -> Result<Value, HashCheckerError> {
    let root = checked_path(workspace, false)?;
    let manifest = checked_path(manifest, false)?;
    let bytes = fs::read(&manifest).map_err(|_| failure("一覧を読み取れません。", 3))?;
    let text =
        std::str::from_utf8(&bytes).map_err(|_| failure("一覧は UTF-8 で記載してください。", 4))?;
    let expected = parse_manifest(text)?;
    let files = scan(&root, &manifest)?;
    if let Ok(relative) = manifest.strip_prefix(&root) {
        let relative = relative.to_string_lossy().replace('\\', "/");
        if expected
            .keys()
            .any(|name| name.eq_ignore_ascii_case(&relative))
        {
            return Err(failure("一覧自身を検証対象にできません。", 4));
        }
    }
    let names: BTreeSet<&String> = expected.keys().chain(files.keys()).collect();
    let mut results = vec![];
    let mut exit_code = 0;
    for name in names {
        let vendor = expected.get(name);
        let actual = files.get(name).map(|path| hash(path));
        let result = match actual {
            Some(Err(error)) => {
                exit_code = exit_code.max(error.exit_code);
                item(
                    name,
                    "error",
                    vendor.map(String::as_str),
                    None,
                    Some(error.exit_code),
                )
            }
            Some(Ok(actual)) => {
                let status = match vendor {
                    None => "added",
                    Some(vendor) if vendor == &actual => "match",
                    Some(_) => "changed",
                };
                if status != "match" {
                    exit_code = exit_code.max(1);
                }
                item(
                    name,
                    status,
                    vendor.map(String::as_str),
                    Some(&actual),
                    None,
                )
            }
            None => {
                exit_code = exit_code.max(1);
                item(name, "removed", vendor.map(String::as_str), None, None)
            }
        };
        results.push(result);
    }
    Ok(make_report("sha256", "compare", results, exit_code, None))
}

pub(super) fn print_report(report: &Value) {
    for result in report["results"].as_array().unwrap() {
        println!(
            "{} : {}",
            result["path"].as_str().unwrap(),
            result["status"].as_str().unwrap()
        );
    }
    println!("終了コード : {}", report["exit_code"]);
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicU64, Ordering};

    static SEQUENCE: AtomicU64 = AtomicU64::new(0);
    struct Workspace(PathBuf);
    impl Workspace {
        fn new() -> Self {
            let path = std::env::temp_dir().join(format!(
                "hash_checker_manifest_{}_{}_{}",
                std::process::id(),
                SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap()
                    .as_nanos(),
                SEQUENCE.fetch_add(1, Ordering::Relaxed)
            ));
            fs::create_dir(&path).unwrap();
            Self(path)
        }
    }
    impl Drop for Workspace {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }
    fn digest() -> String {
        "a".repeat(64)
    }

    #[test]
    fn create_compare_roundtrip_includes_all_extensions_and_self_excludes() {
        let workspace = Workspace::new();
        fs::create_dir(workspace.0.join("sub")).unwrap();
        fs::write(workspace.0.join("sub/日本語 name.txt"), b"hello").unwrap();
        fs::write(workspace.0.join("z.log"), b"world").unwrap();
        let manifest = workspace.0.join("checksums.txt");
        let report = create(&workspace.0, &manifest).unwrap();
        assert_eq!(report["operation"], "create");
        assert_eq!(report["summary"]["match"], 2);
        assert_eq!(report["results"][0]["path"], "sub/日本語 name.txt");
        assert_eq!(report["results"][0]["expected_hash"], Value::Null);
        let text = fs::read_to_string(&manifest).unwrap();
        assert!(!text.contains("checksums.txt"));
        assert!(text.contains(" *sub/日本語 name.txt\n"));
        let report = compare(&workspace.0, &manifest).unwrap();
        assert_eq!(report["exit_code"], 0);
        assert_eq!(report["summary"]["match"], 2);
        assert_eq!(report["schema_version"], 1);
        assert_eq!(report["operation"], "compare");
        assert_eq!(report["error"], Value::Null);
    }

    #[test]
    fn compare_detects_content_change_added_and_removed() {
        let workspace = Workspace::new();
        fs::write(workspace.0.join("changed.bin"), b"old").unwrap();
        fs::write(workspace.0.join("removed.bin"), b"gone").unwrap();
        fs::write(workspace.0.join("match.bin"), b"same").unwrap();
        let manifest = workspace.0.join("checksums.txt");
        create(&workspace.0, &manifest).unwrap();
        fs::write(workspace.0.join("changed.bin"), b"new").unwrap();
        fs::remove_file(workspace.0.join("removed.bin")).unwrap();
        fs::write(workspace.0.join("added.bin"), b"new").unwrap();
        let report = compare(&workspace.0, &manifest).unwrap();
        assert_eq!(report["exit_code"], 1);
        for status in ["added", "removed", "changed", "match"] {
            assert_eq!(report["summary"][status], 1);
        }
        assert_eq!(report["results"][0]["expected_hash"], Value::Null);
        assert_eq!(report["results"][3]["actual_hash"], Value::Null);
    }

    #[test]
    fn create_refuses_overwrite_and_empty_workspace_without_partial_output() {
        let workspace = Workspace::new();
        let manifest = workspace.0.join("checksums.txt");
        assert_eq!(create(&workspace.0, &manifest).unwrap_err().exit_code, 4);
        assert!(!manifest.exists());
        fs::write(&manifest, b"preserve").unwrap();
        assert_eq!(create(&workspace.0, &manifest).unwrap_err().exit_code, 2);
        assert_eq!(fs::read(&manifest).unwrap(), b"preserve");
        assert_eq!(fs::read_dir(&workspace.0).unwrap().count(), 1);
    }

    #[test]
    fn manifest_accepts_bom_crlf_text_marker_and_uppercase_hash() {
        let entries = parse_manifest(&format!(
            "\u{feff}{}  sub/file name.bin\r\n\r\n",
            "A".repeat(64)
        ))
        .unwrap();
        assert_eq!(entries["sub/file name.bin"], digest());
    }

    #[test]
    fn manifest_rejects_unsupported_paths_and_duplicates() {
        for path in [
            "", "/root", "a//b", "a/./b", "a/../b", "a/", "C:foo", "a\\b", "a\tb", "a\0b",
        ] {
            assert!(
                parse_manifest(&format!("{} *{path}\n", digest())).is_err(),
                "path: {path:?}"
            );
        }
        for text in [
            String::new(),
            "\n\n".into(),
            format!("{} *a\n{} *A\n", digest(), digest()),
            format!("\\{} *escaped\n", digest()),
            format!("{}+*a\n", digest()),
            format!("{} *a\n", "g".repeat(64)),
        ] {
            assert_eq!(parse_manifest(&text).unwrap_err().exit_code, 4);
        }
    }

    #[test]
    fn compare_rejects_non_utf8_manifest_and_self_reference() {
        let workspace = Workspace::new();
        let manifest = workspace.0.join("checksums.txt");
        fs::write(&manifest, [0xff, 0xfe]).unwrap();
        assert_eq!(compare(&workspace.0, &manifest).unwrap_err().exit_code, 4);
        fs::write(&manifest, format!("{} *checksums.txt\n", digest())).unwrap();
        assert_eq!(compare(&workspace.0, &manifest).unwrap_err().exit_code, 4);
    }

    #[test]
    fn creation_failures_do_not_leave_temporary_files() {
        let workspace = Workspace::new();
        fs::write(workspace.0.join("data.bin"), b"hello").unwrap();
        let manifest = workspace.0.join("missing/checksums.txt");
        assert_eq!(create(&workspace.0, &manifest).unwrap_err().exit_code, 3);
        assert_eq!(fs::read_dir(&workspace.0).unwrap().count(), 1);
    }

    #[cfg(unix)]
    #[test]
    fn reject_links_and_linked_workspace_ancestor() {
        use std::os::unix::fs::symlink;
        let workspace = Workspace::new();
        let outside = Workspace::new();
        fs::write(outside.0.join("data.bin"), b"hello").unwrap();
        symlink(&outside.0, workspace.0.join("linked")).unwrap();
        assert_eq!(
            create(&workspace.0, &workspace.0.join("checksums.txt"))
                .unwrap_err()
                .exit_code,
            3
        );
        assert_eq!(
            create(
                &workspace.0.join("linked"),
                &workspace.0.join("checksums.txt")
            )
            .unwrap_err()
            .exit_code,
            3
        );
    }

    #[cfg(windows)]
    #[test]
    fn reject_windows_junctions_and_linked_workspace_ancestor() {
        let workspace = Workspace::new();
        let outside = Workspace::new();
        fs::write(outside.0.join("data.bin"), b"hello").unwrap();
        let junction = workspace.0.join("linked");
        let output = std::process::Command::new("cmd")
            .args(["/C", "mklink", "/J"])
            .arg(&junction)
            .arg(&outside.0)
            .output()
            .unwrap();
        assert!(output.status.success(), "junction creation failed");
        assert_eq!(
            create(&workspace.0, &workspace.0.join("checksums.txt"))
                .unwrap_err()
                .exit_code,
            3
        );
        assert_eq!(
            create(&junction, &workspace.0.join("checksums.txt"))
                .unwrap_err()
                .exit_code,
            3
        );
        fs::remove_dir(&junction).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn creation_rejects_case_duplicate_names() {
        let workspace = Workspace::new();
        fs::write(workspace.0.join("data.bin"), b"a").unwrap();
        fs::write(workspace.0.join("DATA.bin"), b"b").unwrap();
        assert_eq!(
            create(&workspace.0, &workspace.0.join("checksums.txt"))
                .unwrap_err()
                .exit_code,
            4
        );
    }

    #[cfg(windows)]
    #[test]
    fn compare_aggregates_read_error_above_changed() {
        use std::os::windows::fs::OpenOptionsExt;
        let workspace = Workspace::new();
        let locked_path = workspace.0.join("locked.bin");
        fs::write(&locked_path, b"hello").unwrap();
        fs::write(workspace.0.join("changed.bin"), b"old").unwrap();
        let manifest = workspace.0.join("checksums.txt");
        create(&workspace.0, &manifest).unwrap();
        fs::write(workspace.0.join("changed.bin"), b"new").unwrap();
        let locked = OpenOptions::new()
            .read(true)
            .share_mode(0)
            .open(&locked_path)
            .unwrap();
        let report = compare(&workspace.0, &manifest).unwrap();
        assert_eq!(report["exit_code"], 3);
        assert_eq!(report["summary"]["changed"], 1);
        assert_eq!(report["summary"]["error"], 1);
        assert_eq!(report["results"][1]["error_code"], 3);
        assert_eq!(report["results"][1]["actual_hash"], Value::Null);
        drop(locked);
    }
}
