from __future__ import annotations

import hashlib
import tempfile
import unittest
from pathlib import Path

import hash_checker


SAMPLE_BYTES = b"hash checker sample\n"


class HashCheckerTests(unittest.TestCase):
    def test_matching_md5_returns_zero(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            workspace = Path(temp_dir)
            target_file = workspace / "firmware.bin"
            hash_file = workspace / "vendor_hash.txt"
            target_file.write_bytes(SAMPLE_BYTES)
            hash_file.write_text(hashlib.md5(SAMPLE_BYTES).hexdigest(), encoding="utf-8")

            result = hash_checker.run(
                ["--algorithm", "md5", "--workspace", str(workspace)]
            )

        self.assertEqual(result, 0)

    def test_mismatched_md5_returns_one(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            workspace = Path(temp_dir)
            (workspace / "firmware.bin").write_bytes(SAMPLE_BYTES)
            (workspace / "vendor_hash.txt").write_text(
                "00000000000000000000000000000000", encoding="utf-8"
            )

            result = hash_checker.run(
                ["--algorithm", "md5", "--workspace", str(workspace)]
            )

        self.assertEqual(result, hash_checker.EXIT_MISMATCH)

    def test_invalid_hash_raises_format_error(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            workspace = Path(temp_dir)
            (workspace / "firmware.bin").write_bytes(SAMPLE_BYTES)
            (workspace / "vendor_hash.txt").write_text("not-a-hash", encoding="utf-8")

            with self.assertRaises(hash_checker.HashCheckerError) as context:
                hash_checker.run(["--algorithm", "md5", "--workspace", str(workspace)])

        self.assertEqual(
            context.exception.exit_code, hash_checker.EXIT_HASH_FORMAT_ERROR
        )

    def test_multiple_targets_raise_discovery_error(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            workspace = Path(temp_dir)
            (workspace / "firmware-a.bin").write_bytes(SAMPLE_BYTES)
            (workspace / "firmware-b.bin").write_bytes(SAMPLE_BYTES)
            (workspace / "vendor_hash.txt").write_text(
                hashlib.md5(SAMPLE_BYTES).hexdigest(), encoding="utf-8"
            )

            with self.assertRaises(hash_checker.HashCheckerError) as context:
                hash_checker.run(["--algorithm", "md5", "--workspace", str(workspace)])

        self.assertEqual(context.exception.exit_code, hash_checker.EXIT_DISCOVERY_ERROR)


if __name__ == "__main__":
    unittest.main()

