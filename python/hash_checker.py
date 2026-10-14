#!/usr/bin/env python3
"""Compare a file hash with a vendor-provided hash text file."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Iterable


ALGORITHMS = {
    "md5": ("MD5", 32),
    "sha256": ("SHA256", 64),
}

EXCLUDED_EXTENSIONS = {".bat", ".cmd", ".log", ".ps1", ".py", ".txt"}

EXIT_MISMATCH = 1
EXIT_DISCOVERY_ERROR = 2
EXIT_READ_ERROR = 3
EXIT_HASH_FORMAT_ERROR = 4
EXIT_HASH_CALCULATION_ERROR = 5
VERSION = "0.2.0"
STATUSES = ("match", "mismatch", "missing", "error", "added", "removed", "changed")


class HashCheckerError(Exception):
    def __init__(self, message: str, exit_code: int) -> None:
        super().__init__(message)
        self.exit_code = exit_code


class ArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise HashCheckerError("引数が不正です。--help で使用方法を確認してください。", 2)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = ArgumentParser(
        description="Compare a calculated file hash with a vendor-provided hash value."
    )
    parser.add_argument(
        "-a",
        "--algorithm",
        choices=sorted(ALGORITHMS),
        help="Hash algorithm to use.",
    )
    parser.add_argument(
        "-w",
        "--workspace",
        type=Path,
        default=Path.cwd(),
        help="Workspace used for automatic file discovery. Defaults to the current directory.",
    )
    parser.add_argument(
        "--hash-file",
        type=Path,
        help="Text file containing the vendor-provided hash. Auto-detected when omitted.",
    )
    parser.add_argument(
        "--target-file",
        type=Path,
        help="File to calculate and verify. Auto-detected when omitted.",
    )
    parser.add_argument("--expected-hash", help="Expected hash value instead of a text file.")
    parser.add_argument("--format", choices=("text", "json"), default="text")
    parser.add_argument("--version", action="store_true")
    parser.add_argument("--capabilities", action="store_true")
    args = parser.parse_args(argv)
    if not (args.version or args.capabilities):
        if args.algorithm is None:
            parser.error("--algorithm is required")
        if args.expected_hash is not None and args.hash_file is not None:
            parser.error("--expected-hash and --hash-file are mutually exclusive")
    return args


def resolve_path(path: Path, workspace: Path) -> Path:
    if path.is_absolute():
        return path
    return workspace / path


def list_files(files: Iterable[Path]) -> str:
    return "\n".join(f"- {path.name}" for path in files)


def discover_hash_file(workspace: Path) -> Path:
    hash_files = sorted(path for path in workspace.glob("*.txt") if path.is_file())

    if not hash_files:
        raise HashCheckerError(
            "ハッシュファイルが見つかりません。作業フォルダに txt ファイルを1つ配置してください。",
            EXIT_DISCOVERY_ERROR,
        )

    if len(hash_files) >= 2:
        files = list_files(hash_files)
        raise HashCheckerError(
            f"ハッシュファイル候補が複数見つかりました。\n\n見つかった txt ファイル:\n{files}\n\n対応: ハッシュ値を記載した txt ファイルを1つだけ残してください。",
            EXIT_DISCOVERY_ERROR,
        )

    return hash_files[0]


def discover_target_file(workspace: Path) -> Path:
    target_files = sorted(
        path
        for path in workspace.iterdir()
        if path.is_file() and path.suffix.lower() not in EXCLUDED_EXTENSIONS
    )

    if not target_files:
        raise HashCheckerError(
            "確認対象ファイルが見つかりません。作業フォルダに確認したいファイルを1つ配置してください。",
            EXIT_DISCOVERY_ERROR,
        )

    if len(target_files) >= 2:
        files = list_files(target_files)
        raise HashCheckerError(
            f"確認対象ファイル候補が複数見つかりました。\n\n見つかったファイル:\n{files}\n\n対応: 確認したいファイルを1つだけ残してください。",
            EXIT_DISCOVERY_ERROR,
        )

    return target_files[0]


def read_vendor_hash(hash_file: Path, expected_length: int, algorithm_label: str) -> str:
    try:
        raw_text = hash_file.read_text(encoding="utf-8-sig")
    except OSError as exc:
        raise HashCheckerError(
            f"ハッシュファイルを読み取れませんでした。ファイルの権限や状態を確認してください。\n{exc}",
            EXIT_READ_ERROR,
        ) from exc
    except UnicodeError as exc:
        raise HashCheckerError("ハッシュファイルはUTF-8で保存してください。", 4) from exc

    lines = [line.strip() for line in raw_text.splitlines() if line.strip()]

    if not lines:
        raise HashCheckerError(
            "ハッシュファイルにハッシュ値が記載されていません。",
            EXIT_HASH_FORMAT_ERROR,
        )

    if len(lines) >= 2:
        raise HashCheckerError(
            "ハッシュファイルに複数行の値が記載されています。\n\n対応: ハッシュ値のみを1行で記載してください。",
            EXIT_HASH_FORMAT_ERROR,
        )

    return validate_hash(lines[0], expected_length, algorithm_label)


def validate_hash(value: str, expected_length: int, algorithm_label: str) -> str:
    vendor_hash = value.strip()

    if len(vendor_hash) != expected_length:
        raise HashCheckerError(
            f"{algorithm_label} ハッシュ値の文字数が不正です。\n\n期待する文字数 : {expected_length} 文字\n実際の文字数   : {len(vendor_hash)} 文字",
            EXIT_HASH_FORMAT_ERROR,
        )

    if not re.fullmatch(r"[0-9a-fA-F]+", vendor_hash):
        raise HashCheckerError(
            "ハッシュ値の形式が不正です。\n\nハッシュ値には 0-9、a-f、A-F のみ使用できます。",
            EXIT_HASH_FORMAT_ERROR,
        )

    return vendor_hash.lower()


def calculate_file_hash(target_file: Path, algorithm: str) -> str:
    try:
        hasher = hashlib.new(algorithm)
        with target_file.open("rb") as file:
            for chunk in iter(lambda: file.read(1024 * 1024), b""):
                hasher.update(chunk)
    except (OSError, ValueError) as exc:
        raise HashCheckerError(
            f"確認対象ファイルのハッシュ値を算出できませんでした。ファイルの権限や状態を確認してください。\n{exc}",
            EXIT_HASH_CALCULATION_ERROR,
        ) from exc

    return hasher.hexdigest().lower()


def print_header(workspace: Path, algorithm_label: str) -> None:
    print("========================================")
    print(" ハッシュ値確認ツール")
    print("========================================")
    print()
    print(f"作業フォルダ : {workspace}")
    print(f"ハッシュ方式 : {algorithm_label}")
    print()


def print_result(
    matched: bool,
    workspace: Path,
    algorithm_label: str,
    hash_file: Path | None,
    target_file: Path,
    vendor_hash: str,
    actual_hash: str,
) -> None:
    if matched:
        print("[正常] ハッシュ値が一致しました。")
    else:
        print("[警告] ハッシュ値が一致しません。")

    print()
    print(f"作業フォルダ       : {workspace}")
    print(f"ハッシュ方式       : {algorithm_label}")
    print(f"ハッシュファイル   : {hash_file.name if hash_file else '直接入力'}")
    print(f"確認対象ファイル   : {target_file.name}")
    print()
    print(f"ベンダー提供ハッシュ値 : {vendor_hash}")
    print(f"算出ハッシュ値         : {actual_hash}")
    print()

    if matched:
        print("結果 : ファイルはベンダー提供ハッシュ値と一致しています。")
    else:
        print("結果 : ファイルが破損している、または想定と異なる可能性があります。")
        print("確認 : ベンダー提供値、確認対象ファイル、ハッシュ方式を確認してください。")


def make_report(algorithm: str | None, results: list[dict], exit_code: int, error=None) -> dict:
    return {
        "schema_version": 1,
        "algorithm": algorithm,
        "operation": "verify",
        "results": results,
        "summary": {status: sum(row["status"] == status for row in results) for status in STATUSES},
        "exit_code": exit_code,
        "error": error,
    }


def json_requested(argv: list[str]) -> bool:
    return "--format=json" in argv or any(
        token == "--format" and i + 1 < len(argv) and argv[i + 1] == "json"
        for i, token in enumerate(argv)
    )


def run(argv: list[str]) -> int:
    try:
        return verify(argv)
    except Exception as exc:
        if not json_requested(argv):
            raise
        code = exc.exit_code if isinstance(exc, HashCheckerError) else 5
        messages = {
            2: "引数または入力ファイルの指定を確認してください。",
            3: "ハッシュファイルを読み取れませんでした。",
            4: "期待ハッシュ値の形式が不正です。",
            5: "対象ファイルのハッシュ値を算出できませんでした。",
        }
        algorithm = next((argv[i + 1] for i, token in enumerate(argv[:-1]) if token in ("--algorithm", "-a")), None)
        if algorithm not in ALGORITHMS:
            algorithm = None
        print(json.dumps(make_report(algorithm, [], code, {"code": code, "message": messages[code]}), ensure_ascii=False))
        return code


def verify(argv: list[str]) -> int:
    args = parse_args(argv)
    if args.version:
        print(VERSION)
        return 0
    if args.capabilities:
        print(json.dumps({"schema_version": 1, "capabilities": ["single-verify", "expected-hash", "json-report"]}))
        return 0
    algorithm_label, expected_length = ALGORITHMS[args.algorithm]
    workspace = args.workspace.resolve()

    if args.format == "text":
        print_header(workspace, algorithm_label)

    if not workspace.is_dir():
        raise HashCheckerError(
            f"作業フォルダが見つかりません: {workspace}",
            EXIT_DISCOVERY_ERROR,
        )

    hash_file = None if args.expected_hash is not None else (
        resolve_path(args.hash_file, workspace).resolve()
        if args.hash_file
        else discover_hash_file(workspace).resolve()
    )
    target_file = (
        resolve_path(args.target_file, workspace).resolve()
        if args.target_file
        else discover_target_file(workspace).resolve()
    )

    vendor_hash = (
        validate_hash(args.expected_hash, expected_length, algorithm_label)
        if args.expected_hash is not None
        else read_vendor_hash(hash_file, expected_length, algorithm_label)
    )
    actual_hash = calculate_file_hash(target_file, args.algorithm)

    matched = vendor_hash == actual_hash
    exit_code = 0 if matched else EXIT_MISMATCH
    if args.format == "json":
        result = {"path": target_file.name, "status": "match" if matched else "mismatch",
                  "expected_hash": vendor_hash, "actual_hash": actual_hash, "error_code": None}
        print(json.dumps(make_report(args.algorithm, [result], exit_code), ensure_ascii=False))
        return exit_code
    print_result(
        matched,
        workspace,
        algorithm_label,
        hash_file,
        target_file,
        vendor_hash,
        actual_hash,
    )
    return 0 if matched else EXIT_MISMATCH


def configure_cli_streams() -> None:
    # Machine-readable JSON and redirected Japanese output use UTF-8 on every OS.
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")


def main() -> int:
    configure_cli_streams()
    try:
        return run(sys.argv[1:])
    except HashCheckerError as exc:
        print(f"[エラー] {exc}")
        print()
        print("処理を終了します。")
        return exc.exit_code
    except Exception as exc:
        print("[エラー] 想定外のエラーが発生しました。")
        print()
        print(exc)
        return EXIT_HASH_CALCULATION_ERROR


if __name__ == "__main__":
    raise SystemExit(main())

