use serde_json::Value;
use std::fs;
use std::path::PathBuf;
use std::process::{Command, Output};
use std::time::{SystemTime, UNIX_EPOCH};

struct Workspace(PathBuf);
impl Workspace {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "hash_checker_cli_{}_{}",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir(&path).unwrap();
        Self(path)
    }
    fn cli(&self, arguments: &[&str]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_hash_checker"))
            .current_dir(&self.0)
            .args(arguments)
            .output()
            .unwrap()
    }
}
impl Drop for Workspace {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn report(output: &Output) -> Value {
    assert!(output.stderr.is_empty());
    serde_json::from_slice(&output.stdout).expect("stdout must be only valid JSON")
}

#[test]
fn cli_version_and_capabilities_do_not_need_algorithm() {
    let workspace = Workspace::new();
    let output = workspace.cli(&["--version"]);
    assert!(output.status.success());
    assert_eq!(String::from_utf8(output.stdout).unwrap().trim(), "0.2.0");
    let output = workspace.cli(&["--capabilities"]);
    assert!(output.status.success());
    let value = report(&output);
    assert_eq!(value["schema_version"], 1);
    assert!(value["capabilities"]
        .as_array()
        .unwrap()
        .contains(&Value::from("manifest-compare")));
}

#[test]
fn cli_json_errors_do_not_disclose_absolute_paths() {
    let workspace = Workspace::new();
    let private_path = workspace
        .0
        .join("private-file.bin")
        .to_string_lossy()
        .into_owned();
    let output = workspace.cli(&[
        "--algorithm",
        "sha256",
        "--format",
        "json",
        "--unexpected",
        &private_path,
    ]);
    assert_eq!(output.status.code(), Some(2));
    let value = report(&output);
    assert_eq!(value["exit_code"], 2);
    assert_eq!(value["results"], serde_json::json!([]));
    assert_eq!(value["error"]["code"], 2);
    assert!(!String::from_utf8(output.stdout)
        .unwrap()
        .contains(&private_path));
}

#[test]
fn cli_missing_and_unknown_algorithm_have_json_null() {
    let workspace = Workspace::new();
    for arguments in [
        vec!["--format", "json"],
        vec!["--algorithm", "unknown", "--format", "json"],
    ] {
        let output = workspace.cli(&arguments);
        assert_eq!(output.status.code(), Some(2));
        assert_eq!(report(&output)["algorithm"], Value::Null);
    }
}

#[test]
fn cli_create_and_compare_use_current_directory_for_manifest() {
    let workspace = Workspace::new();
    fs::create_dir(workspace.0.join("files")).unwrap();
    fs::write(workspace.0.join("files/data.bin"), b"hello").unwrap();
    let output = workspace.cli(&[
        "--algorithm",
        "sha256",
        "--workspace",
        "files",
        "--create-manifest",
        "checksums.txt",
        "--format",
        "json",
    ]);
    assert!(output.status.success());
    assert_eq!(report(&output)["operation"], "create");
    assert!(workspace.0.join("checksums.txt").exists());
    assert!(!workspace.0.join("files/checksums.txt").exists());
    let output = workspace.cli(&[
        "--algorithm",
        "sha256",
        "--workspace",
        "files",
        "--compare-manifest",
        "checksums.txt",
        "--format",
        "json",
    ]);
    assert!(output.status.success());
    assert_eq!(report(&output)["summary"]["match"], 1);
    fs::write(workspace.0.join("files/data.bin"), b"changed").unwrap();
    let output = workspace.cli(&[
        "--algorithm",
        "sha256",
        "--workspace",
        "files",
        "--compare-manifest",
        "checksums.txt",
        "--format",
        "json",
    ]);
    assert_eq!(output.status.code(), Some(1));
    assert_eq!(report(&output)["summary"]["changed"], 1);
}

#[test]
fn cli_single_expected_hash_json_contains_only_filename() {
    let workspace = Workspace::new();
    fs::write(workspace.0.join("data.bin"), b"hello").unwrap();
    let output = workspace.cli(&[
        "--algorithm",
        "md5",
        "--expected-hash",
        "5d41402abc4b2a76b9719d911017c592",
        "--format",
        "json",
    ]);
    assert!(output.status.success());
    let value = report(&output);
    assert_eq!(value["results"][0]["status"], "match");
    assert_eq!(value["results"][0]["path"], "data.bin");
}
