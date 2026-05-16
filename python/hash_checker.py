#!/usr/bin/env python3
"""Compare a file hash with a vendor-provided hash text file."""

from __future__ import annotations

import argparse
import hashlib
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


class HashCheckerError(Exception):
    def __init__(self, message: str, exit_code: int) -> None:
        super().__init__(message)
        self.exit_code = exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Compare a calculated file hash with a vendor-provided hash value."
    )
    parser.add_argument(
        "-a",
        "--algorithm",
        choices=sorted(ALGORITHMS),
        required=True,
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
    return parser.parse_args(argv)


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

    vendor_hash = lines[0]

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
    hash_file: Path,
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
    print(f"ハッシュファイル   : {hash_file.name}")
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


def run(argv: list[str]) -> int:
    args = parse_args(argv)
    algorithm_label, expected_length = ALGORITHMS[args.algorithm]
    workspace = args.workspace.resolve()

    print_header(workspace, algorithm_label)

    if not workspace.is_dir():
        raise HashCheckerError(
            f"作業フォルダが見つかりません: {workspace}",
            EXIT_DISCOVERY_ERROR,
        )

    hash_file = (
        resolve_path(args.hash_file, workspace).resolve()
        if args.hash_file
        else discover_hash_file(workspace).resolve()
    )
    target_file = (
        resolve_path(args.target_file, workspace).resolve()
        if args.target_file
        else discover_target_file(workspace).resolve()
    )

    vendor_hash = read_vendor_hash(hash_file, expected_length, algorithm_label)
    actual_hash = calculate_file_hash(target_file, args.algorithm)

    matched = vendor_hash == actual_hash
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


def main() -> int:
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

