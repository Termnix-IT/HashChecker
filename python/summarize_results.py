#!/usr/bin/env python3
"""Aggregate versioned HashChecker reports into a CSV table."""

from __future__ import annotations

import argparse
import csv
import json
import re
import sys
from collections import defaultdict
from pathlib import Path

from hash_checker import ALGORITHMS, STATUSES, HashCheckerError, configure_cli_streams


MAX_REPORT_BYTES = 16 * 1024 * 1024
FIELDS = ("path", "algorithm", "checks", *STATUSES)


def csv_text(value):
    # CSV quoting does not stop spreadsheets from interpreting a filename as a formula.
    if isinstance(value, str) and (value.lstrip().startswith(("=", "+", "-", "@")) or value.startswith(("\t", "\r", "\n"))):
        return "'" + value
    return value


def write_csv(stream, rows):
    writer = csv.DictWriter(stream, fieldnames=FIELDS)
    writer.writeheader()
    writer.writerows({key: csv_text(value) for key, value in row.items()} for row in rows)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("重複したJSONキーです。")
        result[key] = value
    return result


def read_report(path: Path) -> dict:
    try:
        with path.open("rb") as stream:
            data = stream.read(MAX_REPORT_BYTES + 1)
    except OSError as exc:
        raise HashCheckerError("結果JSONを読み取れませんでした。", 3) from exc
    try:
        if len(data) > MAX_REPORT_BYTES:
            raise ValueError("結果JSONは16MiB以下にしてください。")
        report = json.loads(data.decode("utf-8-sig"), object_pairs_hook=unique_object)
        validate_report(report)
    except (ValueError, UnicodeError, TypeError, KeyError, RecursionError) as exc:
        raise HashCheckerError("結果JSONの形式またはバージョンが不正です。", 4) from exc
    return report


def validate_report(report: dict) -> None:
    if not isinstance(report, dict) or type(report.get("schema_version")) is not int or report["schema_version"] != 1:
        raise ValueError("対応していないバージョンです。")
    if report.get("operation") not in ("verify", "compare", "create"):
        raise ValueError("operationが不正です。")
    algorithm = report["algorithm"]
    if algorithm not in ALGORITHMS and algorithm is not None:
        raise ValueError("algorithmが不正です。")
    if type(report.get("exit_code")) is not int or report["exit_code"] not in range(6):
        raise ValueError("exit_codeが不正です。")
    if not isinstance(report.get("results"), list) or not isinstance(report.get("summary"), dict):
        raise ValueError("resultsまたはsummaryが不正です。")
    error = report["error"]
    if error is not None:
        if (not isinstance(error, dict) or type(error.get("code")) is not int
                or error["code"] not in range(2, 6) or error["code"] != report["exit_code"]
                or not isinstance(error.get("message"), str) or report["results"]):
            raise ValueError("全体エラーが不正です。")
    elif algorithm is None:
        raise ValueError("正常結果にはalgorithmが必要です。")
    seen = set()
    counts = {status: 0 for status in STATUSES}
    for row in report["results"]:
        if not isinstance(row, dict) or row.get("status") not in STATUSES:
            raise ValueError("判定が不正です。")
        name = row["path"]
        if (not isinstance(name, str) or not name or "\\" in name or ":" in name
                or any(part in ("", ".", "..") for part in name.split("/"))
                or any(ord(char) < 32 or ord(char) == 127 for char in name)):
            raise ValueError("相対パスが不正です。")
        if name in seen:
            raise ValueError("結果の対象パスが重複しています。")
        seen.add(name)
        counts[row["status"]] += 1
        for field in ("expected_hash", "actual_hash"):
            value = row[field]
            if value is not None and (algorithm not in ALGORITHMS or not isinstance(value, str)
                    or re.fullmatch(r"[0-9a-f]{%d}" % ALGORITHMS[algorithm][1], value) is None):
                raise ValueError("ハッシュ値が不正です。")
        code = row["error_code"]
        if code is not None and (type(code) is not int or code not in range(2, 6)):
            raise ValueError("error_codeが不正です。")
        expected, actual = row["expected_hash"], row["actual_hash"]
        status = row["status"]
        if status == "match" and (actual is None or (expected is None and report["operation"] != "create") or (expected is not None and actual != expected)):
            raise ValueError("一致結果とハッシュ値が矛盾しています。")
        if status in ("mismatch", "changed") and (expected is None or actual is None or expected == actual):
            raise ValueError("変更結果とハッシュ値が矛盾しています。")
        if status in ("missing", "removed") and (expected is None or actual is not None):
            raise ValueError("欠落結果が不正です。")
        if status == "added" and (expected is not None or actual is None):
            raise ValueError("追加結果が不正です。")
        if (status in ("error", "missing")) != (code is not None):
            raise ValueError("状態とerror_codeが矛盾しています。")
        if (status == "missing" and code != 3) or (status == "error" and code not in (3, 5)):
            raise ValueError("欠落または処理エラーの終了コードが不正です。")
        if report["operation"] == "verify" and status in ("added", "removed", "changed"):
            raise ValueError("検証モードの状態が不正です。")
        if report["operation"] == "compare" and status in ("mismatch", "missing"):
            raise ValueError("比較モードの状態が不正です。")
        if report["operation"] == "create" and status != "match":
            raise ValueError("作成モードの状態が不正です。")
    if set(report["summary"]) != set(STATUSES) or any(type(report["summary"][status]) is not int or report["summary"][status] != counts[status] for status in STATUSES):
        raise ValueError("集計件数が一致しません。")
    if error is None:
        codes = [row["error_code"] or (1 if row["status"] in ("mismatch", "added", "removed", "changed") else 0) for row in report["results"]]
        if max(codes, default=0) != report["exit_code"]:
            raise ValueError("終了コードと判定が矛盾しています。")


def aggregate(paths: list[Path]) -> list[dict]:
    grouped = defaultdict(lambda: {status: 0 for status in STATUSES})
    for path in paths:
        report = read_report(path)
        algorithm = report["algorithm"] or ""
        if report["error"] is not None:
            grouped[("", algorithm)]["error"] += 1
        for result in report["results"]:
            grouped[(result["path"], algorithm)][result["status"]] += 1
    return [{"path": path, "algorithm": algorithm, "checks": sum(counts.values()), **counts}
            for (path, algorithm), counts in sorted(grouped.items())]


def run(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description="Aggregate HashChecker JSON reports into CSV.")
    parser.add_argument("reports", type=Path, nargs="+")
    parser.add_argument("--output", type=Path, help="New CSV file; existing files are never overwritten.")
    args = parser.parse_args(argv)
    rows = aggregate(args.reports)
    if args.output:
        try:
            # Validate every input before creating output. Exclusive creation protects existing records.
            with args.output.open("x", encoding="utf-8-sig", newline="") as stream:
                write_csv(stream, rows)
        except OSError as exc:
            raise HashCheckerError("CSV保存に失敗しました。保存先の権限と既存ファイルを確認してください。", 3) from exc
    else:
        write_csv(sys.stdout, rows)
    return 0


def main() -> int:
    configure_cli_streams()
    try:
        return run(sys.argv[1:])
    except HashCheckerError as exc:
        print(f"[エラー] {exc}", file=sys.stderr)
        return exc.exit_code


if __name__ == "__main__":
    raise SystemExit(main())
