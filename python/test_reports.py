from __future__ import annotations

import copy
import csv
import hashlib
import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

import hash_checker
import summarize_results


class ReportTests(unittest.TestCase):
    def test_direct_hash_and_json_match_without_hash_file(self):
        with tempfile.TemporaryDirectory() as folder:
            target = Path(folder) / "日本語 file.bin"
            target.write_bytes(b"sample")
            stream = io.StringIO()
            with redirect_stdout(stream):
                code = hash_checker.run(["--algorithm", "sha256", "--workspace", folder,
                    "--expected-hash", " " + hashlib.sha256(b"sample").hexdigest().upper() + " ", "--format", "json"])
            report = json.loads(stream.getvalue())
            self.assertEqual(code, 0)
            self.assertEqual(report["results"][0]["path"], target.name)
            summarize_results.validate_report(report)
            self.assertNotIn(folder, stream.getvalue())

    def test_json_errors_are_single_safe_documents(self):
        for extra, expected in ((["--hash-file", "private.txt", "--expected-hash", "a"], 2),
                                (["--expected-hash", "invalid", "--target-file", __file__], 4),
                                (["--unknown"], 2)):
            with self.subTest(extra=extra), redirect_stdout(io.StringIO()) as stream:
                code = hash_checker.run(["--algorithm", "sha256", "--format", "json", *extra])
                report = json.loads(stream.getvalue())
                self.assertEqual(code, expected)
                summarize_results.validate_report(report)

    def test_metadata_does_not_require_algorithm(self):
        for flag in ("--version", "--capabilities"):
            with redirect_stdout(io.StringIO()) as stream:
                self.assertEqual(hash_checker.run([flag]), 0)
            self.assertTrue(stream.getvalue().strip())

    def report(self):
        digest = hashlib.sha256(b"data").hexdigest()
        return hash_checker.make_report("sha256", [{"path": "sub/日本語.bin", "status": "match",
            "expected_hash": digest, "actual_hash": digest, "error_code": None}], 0)

    def test_aggregate_counts_errors_and_preserves_existing_csv(self):
        with tempfile.TemporaryDirectory() as folder:
            first, second, error, output = [Path(folder) / name for name in ("a.json", "b.json", "error.json", "report.csv")]
            matched = self.report()
            failed = copy.deepcopy(matched)
            failed["results"][0].update(status="mismatch", actual_hash="0" * 64)
            failed["summary"].update(match=0, mismatch=1)
            failed["exit_code"] = 1
            for path, value in ((first, matched), (second, failed), (error, hash_checker.make_report("sha256", [], 4, {"code": 4, "message": "bad input"}))):
                path.write_text(json.dumps(value), encoding="utf-8-sig")
            self.assertEqual(summarize_results.run([str(first), str(second), str(error), "--output", str(output)]), 0)
            with output.open(encoding="utf-8-sig", newline="") as stream:
                rows = list(csv.DictReader(stream))
            self.assertEqual(rows[0]["error"], "1")
            self.assertEqual(rows[1]["checks"], "2")
            self.assertEqual(rows[1]["mismatch"], "1")
            original = output.read_bytes()
            with self.assertRaises(hash_checker.HashCheckerError):
                summarize_results.run([str(first), "--output", str(output)])
            self.assertEqual(output.read_bytes(), original)

    def test_invalid_reports_are_rejected_before_output(self):
        with tempfile.TemporaryDirectory() as folder:
            source, output = Path(folder) / "input.json", Path(folder) / "output.csv"
            for mutate in (lambda r: r.update(schema_version=2),
                           lambda r: r["summary"].update(match=2),
                           lambda r: r["results"][0].update(status="unknown"),
                           lambda r: r["results"][0].update(path="../escape"),
                           lambda r: r["results"][0].update(actual_hash="0" * 64),
                           lambda r: r.update(exit_code=1)):
                report = self.report()
                mutate(report)
                source.write_text(json.dumps(report), encoding="utf-8")
                with self.assertRaises(hash_checker.HashCheckerError) as context:
                    summarize_results.run([str(source), "--output", str(output)])
                self.assertEqual(context.exception.exit_code, 4)
                self.assertFalse(output.exists())
            source.write_bytes(b"\xff")
            with self.assertRaises(hash_checker.HashCheckerError):
                summarize_results.read_report(source)
            source.write_text('{"schema_version":1,"schema_version":1}', encoding="utf-8")
            with self.assertRaises(hash_checker.HashCheckerError):
                summarize_results.read_report(source)

    def test_missing_and_read_errors_require_the_contract_exit_code(self):
        for status, code in (("missing", 2), ("missing", 5), ("error", 2), ("error", 4)):
            report = self.report()
            report["results"][0].update(status=status, error_code=code, actual_hash=None)
            report["summary"].update(match=0)
            report["summary"][status] = 1
            report["exit_code"] = code
            with self.subTest(status=status, code=code), self.assertRaises(ValueError):
                summarize_results.validate_report(report)

    def test_csv_filenames_are_text_and_json_names_are_preserved(self):
        report = self.report()
        for name in ("=1+1", "+SUM(1,2)", "-1+2", "@SUM(1)", " =1+1"):
            report["results"][0]["path"] = name
            summarize_results.validate_report(report)
            self.assertEqual(report["results"][0]["path"], name)
            with tempfile.TemporaryDirectory() as folder:
                source = Path(folder) / "input.json"
                source.write_text(json.dumps(report), encoding="utf-8")
                with redirect_stdout(io.StringIO()) as stream:
                    summarize_results.run([str(source)])
                row = next(csv.DictReader(io.StringIO(stream.getvalue())))
                self.assertEqual(row["path"], "'" + name)


if __name__ == "__main__":
    unittest.main()
