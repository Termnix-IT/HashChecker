use md5::Md5;
use sha2::{Digest, Sha256};
use std::env;
use std::error::Error;
use std::fmt;
use std::fs;
use std::io::{self, Read};
use std::path::{Path, PathBuf};
use std::process;

#[derive(Clone, Copy)]
struct AlgorithmConfig {
    key: &'static str,
    label: &'static str,
    expected_length: usize,
}

const ALGORITHMS: [AlgorithmConfig; 2] = [
    AlgorithmConfig {
        key: "md5",
        label: "MD5",
        expected_length: 32,
    },
    AlgorithmConfig {
        key: "sha256",
        label: "SHA256",
        expected_length: 64,
    },
];

const EXCLUDED_EXTENSIONS: [&str; 13] = [
    "bat", "cmd", "cs", "csproj", "exe", "go", "lock", "log", "ps1", "py", "rs", "toml", "txt",
];

const EXIT_MISMATCH: i32 = 1;
const EXIT_DISCOVERY_ERROR: i32 = 2;
const EXIT_READ_ERROR: i32 = 3;
const EXIT_HASH_FORMAT_ERROR: i32 = 4;
const EXIT_HASH_CALCULATION_ERROR: i32 = 5;

#[derive(Debug)]
struct HashCheckerError {
    message: String,
    exit_code: i32,
}

impl HashCheckerError {
    fn new(message: impl Into<String>, exit_code: i32) -> Self {
        Self {
            message: message.into(),
            exit_code,
        }
    }
}

impl fmt::Display for HashCheckerError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(formatter, "{}", self.message)
    }
}

impl Error for HashCheckerError {}

#[derive(Debug)]
struct Options {
    algorithm: String,
    workspace: PathBuf,
    hash_file: Option<PathBuf>,
    target_file: Option<PathBuf>,
}

fn main() {
    process::exit(run(env::args().skip(1).collect()));
}

fn run(args: Vec<String>) -> i32 {
    match execute(args) {
        Ok(matched) => {
            if matched {
                0
            } else {
                EXIT_MISMATCH
            }
        }
        Err(error) => print_error(error),
    }
}

fn execute(args: Vec<String>) -> Result<bool, HashCheckerError> {
    let options = parse_args(&args)?;
    let config = find_algorithm(&options.algorithm).ok_or_else(|| {
        HashCheckerError::new(
            "ハッシュ方式を指定してください。対応方式: md5, sha256",
            EXIT_DISCOVERY_ERROR,
        )
    })?;
    let workspace = absolute_path(&options.workspace)?;

    print_header(&workspace, config.label);

    if !workspace.is_dir() {
        return Err(HashCheckerError::new(
            format!("作業フォルダが見つかりません: {}", workspace.display()),
            EXIT_DISCOVERY_ERROR,
        ));
    }

    let hash_file = match options.hash_file {
        Some(path) => resolve_path(&path, &workspace)?,
        None => discover_hash_file(&workspace)?,
    };
    let target_file = match options.target_file {
        Some(path) => resolve_path(&path, &workspace)?,
        None => discover_target_file(&workspace)?,
    };

    let vendor_hash = read_vendor_hash(&hash_file, config)?;
    let actual_hash = calculate_file_hash(&target_file, config)?;
    let matched = vendor_hash == actual_hash;

    print_result(
        matched,
        &workspace,
        config.label,
        &hash_file,
        &target_file,
        &vendor_hash,
        &actual_hash,
    );

    Ok(matched)
}

fn parse_args(args: &[String]) -> Result<Options, HashCheckerError> {
    let mut algorithm: Option<String> = None;
    let mut workspace = PathBuf::from(".");
    let mut hash_file: Option<PathBuf> = None;
    let mut target_file: Option<PathBuf> = None;
    let mut index = 0;

    while index < args.len() {
        match args[index].as_str() {
            "-a" | "--algorithm" => {
                let option_name = args[index].clone();
                algorithm = Some(read_option_value(args, &mut index, &option_name)?.to_lowercase());
            }
            "-w" | "--workspace" => {
                let option_name = args[index].clone();
                workspace = PathBuf::from(read_option_value(args, &mut index, &option_name)?);
            }
            "--hash-file" => {
                let option_name = args[index].clone();
                hash_file = Some(PathBuf::from(read_option_value(
                    args,
                    &mut index,
                    &option_name,
                )?));
            }
            "--target-file" => {
                let option_name = args[index].clone();
                target_file = Some(PathBuf::from(read_option_value(
                    args,
                    &mut index,
                    &option_name,
                )?));
            }
            unknown => {
                return Err(HashCheckerError::new(
                    format!("不明な引数です: {unknown}"),
                    EXIT_DISCOVERY_ERROR,
                ));
            }
        }
        index += 1;
    }

    let algorithm = algorithm.ok_or_else(|| {
        HashCheckerError::new(
            "ハッシュ方式を指定してください。対応方式: md5, sha256",
            EXIT_DISCOVERY_ERROR,
        )
    })?;

    if find_algorithm(&algorithm).is_none() {
        return Err(HashCheckerError::new(
            "ハッシュ方式を指定してください。対応方式: md5, sha256",
            EXIT_DISCOVERY_ERROR,
        ));
    }

    Ok(Options {
        algorithm,
        workspace,
        hash_file,
        target_file,
    })
}

fn read_option_value(
    args: &[String],
    index: &mut usize,
    option_name: &str,
) -> Result<String, HashCheckerError> {
    if *index + 1 >= args.len() || args[*index + 1].starts_with('-') {
        return Err(HashCheckerError::new(
            format!("{option_name} の値を指定してください。"),
            EXIT_DISCOVERY_ERROR,
        ));
    }

    *index += 1;
    Ok(args[*index].clone())
}

fn find_algorithm(name: &str) -> Option<AlgorithmConfig> {
    ALGORITHMS
        .iter()
        .copied()
        .find(|algorithm| algorithm.key == name)
}

fn absolute_path(path: &Path) -> Result<PathBuf, HashCheckerError> {
    if path.is_absolute() {
        return Ok(path.to_path_buf());
    }

    env::current_dir()
        .map(|current_dir| current_dir.join(path))
        .map_err(|error| {
            HashCheckerError::new(
                format!(
                    "作業フォルダのパスを解決できません: {}\n{error}",
                    path.display()
                ),
                EXIT_DISCOVERY_ERROR,
            )
        })
}

fn resolve_path(path: &Path, workspace: &Path) -> Result<PathBuf, HashCheckerError> {
    if path.is_absolute() {
        Ok(path.to_path_buf())
    } else {
        Ok(workspace.join(path))
    }
}

fn discover_hash_file(workspace: &Path) -> Result<PathBuf, HashCheckerError> {
    let mut hash_files = fs::read_dir(workspace)
        .map_err(|error| {
            HashCheckerError::new(
                format!("ハッシュファイルを検索できませんでした。\n{error}"),
                EXIT_DISCOVERY_ERROR,
            )
        })?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.is_file() && extension_equals(path, "txt"))
        .collect::<Vec<_>>();
    hash_files.sort_by_key(|path| path.to_string_lossy().to_lowercase());

    match hash_files.len() {
        0 => Err(HashCheckerError::new(
            "ハッシュファイルが見つかりません。作業フォルダに txt ファイルを1つ配置してください。",
            EXIT_DISCOVERY_ERROR,
        )),
        1 => Ok(hash_files.remove(0)),
        _ => Err(HashCheckerError::new(
            format!(
                "ハッシュファイル候補が複数見つかりました。\n\n見つかった txt ファイル:\n{}\n\n対応: ハッシュ値を記載した txt ファイルを1つだけ残してください。",
                list_files(&hash_files)
            ),
            EXIT_DISCOVERY_ERROR,
        )),
    }
}

fn discover_target_file(workspace: &Path) -> Result<PathBuf, HashCheckerError> {
    let mut target_files = fs::read_dir(workspace)
        .map_err(|error| {
            HashCheckerError::new(
                format!("確認対象ファイルを検索できませんでした。\n{error}"),
                EXIT_DISCOVERY_ERROR,
            )
        })?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.is_file() && !should_exclude_target_file(path))
        .collect::<Vec<_>>();
    target_files.sort_by_key(|path| path.to_string_lossy().to_lowercase());

    match target_files.len() {
        0 => Err(HashCheckerError::new(
            "確認対象ファイルが見つかりません。作業フォルダに確認したいファイルを1つ配置してください。",
            EXIT_DISCOVERY_ERROR,
        )),
        1 => Ok(target_files.remove(0)),
        _ => Err(HashCheckerError::new(
            format!(
                "確認対象ファイル候補が複数見つかりました。\n\n見つかったファイル:\n{}\n\n対応: 確認したいファイルを1つだけ残してください。",
                list_files(&target_files)
            ),
            EXIT_DISCOVERY_ERROR,
        )),
    }
}

fn should_exclude_target_file(path: &Path) -> bool {
    extension(path)
        .map(|extension| EXCLUDED_EXTENSIONS.contains(&extension.as_str()))
        .unwrap_or(false)
}

fn extension_equals(path: &Path, expected: &str) -> bool {
    extension(path)
        .map(|extension| extension == expected)
        .unwrap_or(false)
}

fn extension(path: &Path) -> Option<String> {
    path.extension()
        .and_then(|extension| extension.to_str())
        .map(|extension| extension.to_ascii_lowercase())
}

fn list_files(paths: &[PathBuf]) -> String {
    paths
        .iter()
        .map(|path| {
            format!(
                "- {}",
                path.file_name()
                    .and_then(|name| name.to_str())
                    .unwrap_or("<unknown>")
            )
        })
        .collect::<Vec<_>>()
        .join("\n")
}

fn read_vendor_hash(hash_file: &Path, config: AlgorithmConfig) -> Result<String, HashCheckerError> {
    let raw_text = fs::read_to_string(hash_file).map_err(|error| {
        HashCheckerError::new(
            format!("ハッシュファイルを読み取れませんでした。ファイルの権限や状態を確認してください。\n{error}"),
            EXIT_READ_ERROR,
        )
    })?;
    let text = raw_text.strip_prefix('\u{feff}').unwrap_or(&raw_text);
    let lines = text
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty())
        .collect::<Vec<_>>();

    if lines.is_empty() {
        return Err(HashCheckerError::new(
            "ハッシュファイルにハッシュ値が記載されていません。",
            EXIT_HASH_FORMAT_ERROR,
        ));
    }

    if lines.len() >= 2 {
        return Err(HashCheckerError::new(
            "ハッシュファイルに複数行の値が記載されています。\n\n対応: ハッシュ値のみを1行で記載してください。",
            EXIT_HASH_FORMAT_ERROR,
        ));
    }

    let vendor_hash = lines[0];
    if vendor_hash.len() != config.expected_length {
        return Err(HashCheckerError::new(
            format!(
                "{} ハッシュ値の文字数が不正です。\n\n期待する文字数 : {} 文字\n実際の文字数   : {} 文字",
                config.label,
                config.expected_length,
                vendor_hash.len()
            ),
            EXIT_HASH_FORMAT_ERROR,
        ));
    }

    if !vendor_hash
        .chars()
        .all(|character| character.is_ascii_hexdigit())
    {
        return Err(HashCheckerError::new(
            "ハッシュ値の形式が不正です。\n\nハッシュ値には 0-9、a-f、A-F のみ使用できます。",
            EXIT_HASH_FORMAT_ERROR,
        ));
    }

    Ok(vendor_hash.to_ascii_lowercase())
}

fn calculate_file_hash(
    target_file: &Path,
    config: AlgorithmConfig,
) -> Result<String, HashCheckerError> {
    let mut file = fs::File::open(target_file).map_err(hash_calculation_error)?;
    let mut buffer = vec![0_u8; 1024 * 1024];

    match config.key {
        "md5" => {
            let mut hasher = Md5::new();
            update_hash(&mut file, &mut buffer[..], &mut hasher)?;
            Ok(to_hex(&hasher.finalize()))
        }
        "sha256" => {
            let mut hasher = Sha256::new();
            update_hash(&mut file, &mut buffer[..], &mut hasher)?;
            Ok(to_hex(&hasher.finalize()))
        }
        _ => Err(HashCheckerError::new(
            "未対応のハッシュ方式です。",
            EXIT_DISCOVERY_ERROR,
        )),
    }
}

fn update_hash<D: Digest>(
    file: &mut fs::File,
    buffer: &mut [u8],
    hasher: &mut D,
) -> Result<(), HashCheckerError> {
    loop {
        let bytes_read = file.read(buffer).map_err(hash_calculation_error)?;
        if bytes_read == 0 {
            return Ok(());
        }
        hasher.update(&buffer[..bytes_read]);
    }
}

fn hash_calculation_error(error: io::Error) -> HashCheckerError {
    HashCheckerError::new(
        format!("確認対象ファイルのハッシュ値を算出できませんでした。ファイルの権限や状態を確認してください。\n{error}"),
        EXIT_HASH_CALCULATION_ERROR,
    )
}

fn to_hex(bytes: &[u8]) -> String {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        output.push(HEX[(byte >> 4) as usize] as char);
        output.push(HEX[(byte & 0x0f) as usize] as char);
    }
    output
}

fn print_header(workspace: &Path, algorithm_label: &str) {
    println!("========================================");
    println!(" ハッシュ値確認ツール");
    println!("========================================");
    println!();
    println!("作業フォルダ : {}", workspace.display());
    println!("ハッシュ方式 : {algorithm_label}");
    println!();
}

fn print_result(
    matched: bool,
    workspace: &Path,
    algorithm_label: &str,
    hash_file: &Path,
    target_file: &Path,
    vendor_hash: &str,
    actual_hash: &str,
) {
    if matched {
        println!("[正常] ハッシュ値が一致しました。");
    } else {
        println!("[警告] ハッシュ値が一致しません。");
    }

    println!();
    println!("作業フォルダ       : {}", workspace.display());
    println!("ハッシュ方式       : {algorithm_label}");
    println!("ハッシュファイル   : {}", file_name(hash_file));
    println!("確認対象ファイル   : {}", file_name(target_file));
    println!();
    println!("ベンダー提供ハッシュ値 : {vendor_hash}");
    println!("算出ハッシュ値         : {actual_hash}");
    println!();

    if matched {
        println!("結果 : ファイルはベンダー提供ハッシュ値と一致しています。");
    } else {
        println!("結果 : ファイルが破損している、または想定と異なる可能性があります。");
        println!("確認 : ベンダー提供値、確認対象ファイル、ハッシュ方式を確認してください。");
    }
}

fn file_name(path: &Path) -> String {
    path.file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("<unknown>")
        .to_string()
}

fn print_error(error: HashCheckerError) -> i32 {
    println!("[エラー] {}", error.message);
    println!();
    println!("処理を終了します。");
    error.exit_code
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn temp_workspace(name: &str) -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("system time should be after unix epoch")
            .as_nanos();
        let path = env::temp_dir().join(format!("hash_checker_{name}_{}_{}", process::id(), nanos));
        fs::create_dir_all(&path).expect("temp workspace should be created");
        path
    }

    #[test]
    fn read_vendor_hash_trims_bom_and_lowercases() {
        let workspace = temp_workspace("vendor_hash");
        let hash_file = workspace.join("vendor_hash.txt");
        fs::write(&hash_file, "\u{feff}ABCDEF0123456789ABCDEF0123456789\n")
            .expect("hash file should be written");

        let config = find_algorithm("md5").expect("md5 config should exist");
        let actual = read_vendor_hash(&hash_file, config).expect("hash should be valid");

        assert_eq!(actual, "abcdef0123456789abcdef0123456789");
        fs::remove_dir_all(workspace).expect("temp workspace should be removed");
    }

    #[test]
    fn read_vendor_hash_rejects_multiple_non_empty_lines() {
        let workspace = temp_workspace("multiple_lines");
        let hash_file = workspace.join("vendor_hash.txt");
        fs::write(
            &hash_file,
            "abcdef0123456789abcdef0123456789\nabcdef0123456789abcdef0123456789\n",
        )
        .expect("hash file should be written");

        let config = find_algorithm("md5").expect("md5 config should exist");
        let error = read_vendor_hash(&hash_file, config).expect_err("multiple lines should fail");

        assert_eq!(error.exit_code, EXIT_HASH_FORMAT_ERROR);
        fs::remove_dir_all(workspace).expect("temp workspace should be removed");
    }

    #[test]
    fn discover_target_file_excludes_hash_text_file() {
        let workspace = temp_workspace("target_discovery");
        let hash_file = workspace.join("vendor_hash.txt");
        let target_file = workspace.join("firmware.bin");
        fs::write(hash_file, "abcdef0123456789abcdef0123456789\n")
            .expect("hash file should be written");
        fs::write(&target_file, b"hello").expect("target file should be written");

        let actual = discover_target_file(&workspace).expect("target should be discovered");

        assert_eq!(actual, target_file);
        fs::remove_dir_all(workspace).expect("temp workspace should be removed");
    }

    #[test]
    fn calculate_file_hash_matches_sha256() {
        let workspace = temp_workspace("sha256");
        let target_file = workspace.join("firmware.bin");
        fs::write(&target_file, b"hello").expect("target file should be written");

        let config = find_algorithm("sha256").expect("sha256 config should exist");
        let actual = calculate_file_hash(&target_file, config).expect("hash should be calculated");

        assert_eq!(
            actual,
            "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
        );
        fs::remove_dir_all(workspace).expect("temp workspace should be removed");
    }
}
