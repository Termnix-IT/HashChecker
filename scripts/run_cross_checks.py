#!/usr/bin/env python3
"""Run the same behavior checks against every HashChecker implementation."""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class LanguageCommand:
    name: str
    command_prefix: tuple[str, ...]
    required_tool: str | None


@dataclass(frozen=True)
class CheckCase:
    name: str
    algorithm: str
    workspace: str
    expected_exit_code: int


LANGUAGES: tuple[LanguageCommand, ...] = (
    LanguageCommand(
        name="python",
        command_prefix=(sys.executable, str(REPO_ROOT / "python" / "hash_checker.py")),
        required_tool=None,
    ),
    LanguageCommand(
        name="go",
        command_prefix=("go", "run", "./go"),
        required_tool="go",
    ),
    LanguageCommand(
        name="csharp",
        command_prefix=("dotnet", "run", "--project", str(REPO_ROOT / "csharp" / "HashChecker"), "--"),
        required_tool="dotnet",
    ),
    LanguageCommand(
        name="rust",
        command_prefix=("cargo", "run", "--manifest-path", str(REPO_ROOT / "rust" / "Cargo.toml"), "--"),
        required_tool="cargo",
    ),
)

CHECK_CASES: tuple[CheckCase, ...] = (
    CheckCase("ok-sha256", "sha256", "testdata/ok-sha256", 0),
    CheckCase("ok-md5", "md5", "testdata/ok-md5", 0),
    CheckCase("mismatch-md5", "md5", "testdata/mismatch", 1),
    CheckCase("invalid-hash-sha256", "sha256", "testdata/invalid-hash", 4),
    CheckCase("multiple-targets-md5", "md5", "testdata/multiple-targets", 2),
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run cross-language HashChecker behavior checks."
    )
    parser.add_argument(
        "--language",
        choices=[language.name for language in LANGUAGES],
        action="append",
        help="Run only the selected language. Can be specified multiple times.",
    )
    parser.add_argument(
        "--allow-missing",
        action="store_true",
        help="Treat missing language runtimes as skipped instead of failed.",
    )
    return parser.parse_args()


def selected_languages(language_names: list[str] | None) -> list[LanguageCommand]:
    if not language_names:
        return list(LANGUAGES)

    requested = set(language_names)
    return [language for language in LANGUAGES if language.name in requested]


def build_command(language: LanguageCommand, check_case: CheckCase) -> list[str]:
    return [
        *language.command_prefix,
        "--algorithm",
        check_case.algorithm,
        "--workspace",
        str(REPO_ROOT / check_case.workspace),
    ]


def print_command(command: list[str]) -> str:
    return " ".join(command)


def print_process_output(result: subprocess.CompletedProcess[str]) -> None:
    if result.stdout.strip():
        print("stdout:")
        print(result.stdout.rstrip())
    if result.stderr.strip():
        print("stderr:")
        print(result.stderr.rstrip())


def run_check(language: LanguageCommand, check_case: CheckCase) -> bool:
    command = build_command(language, check_case)
    result = subprocess.run(
        command,
        cwd=REPO_ROOT,
        text=True,
        capture_output=True,
        check=False,
    )

    if result.returncode == check_case.expected_exit_code:
        print(f"PASS {language.name:<7} {check_case.name:<22} exit={result.returncode}")
        return True

    print(
        f"FAIL {language.name:<7} {check_case.name:<22} "
        f"expected={check_case.expected_exit_code} actual={result.returncode}"
    )
    print(f"command: {print_command(command)}")
    print_process_output(result)
    return False


def runtime_available(language: LanguageCommand) -> bool:
    return language.required_tool is None or shutil.which(language.required_tool) is not None


def main() -> int:
    args = parse_args()
    languages = selected_languages(args.language)
    failed = False

    print(f"Repository: {REPO_ROOT}")
    print()

    for language in languages:
        if not runtime_available(language):
            print(f"MISSING {language.name:<7} required tool: {language.required_tool}")
            failed = failed or not args.allow_missing
            continue

        for check_case in CHECK_CASES:
            if not run_check(language, check_case):
                failed = True

    print()
    if failed:
        print("Cross-language checks failed.")
        return 1

    print("Cross-language checks passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
