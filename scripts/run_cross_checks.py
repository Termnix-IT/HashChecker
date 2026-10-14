#!/usr/bin/env python3
"""Check common CLI contracts and Rust -> Go -> Python interoperability."""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT / 'python'))
from summarize_results import validate_report


@dataclass(frozen=True)
class LanguageCommand:
    name: str
    command_prefix: tuple[str, ...]
    required_tool: str | None


LANGUAGES = (
    LanguageCommand('python', (sys.executable, str(REPO_ROOT / 'python/hash_checker.py')), None),
    LanguageCommand('go', ('go',), 'go'),
    LanguageCommand('csharp', ('dotnet',), 'dotnet'),
    LanguageCommand('rust', ('cargo',), 'cargo'),
)
CASES = (('ok-sha256', 'sha256', 0), ('ok-md5', 'md5', 0),
         ('mismatch', 'md5', 1), ('invalid-hash', 'sha256', 4), ('multiple-targets', 'md5', 2))


def invoke(language, arguments):
    return subprocess.run([*language.command_prefix, *arguments], cwd=REPO_ROOT,
                          capture_output=True, text=True, encoding='utf-8', check=False)


def json_command(language, arguments, expected_code):
    result = invoke(language, arguments)
    assert result.returncode == expected_code, f'{language.name}: expected {expected_code}, got {result.returncode}: {result.stdout} {result.stderr}'
    report = json.loads(result.stdout)
    validate_report(report)
    assert report['exit_code'] == expected_code
    assert str(REPO_ROOT) not in result.stdout, 'Absolute workspace path in JSON'
    return report


def prepare_language(language, build_dir):
    if language.name == 'python':
        return language
    if language.name == 'go':
        binary = build_dir / ('hash_checker_go.exe' if os.name == 'nt' else 'hash_checker_go')
        command = ['go', 'build', '-o', str(binary), './go']
        prefix = (str(binary),)
    elif language.name == 'rust':
        command = ['cargo', 'build', '--manifest-path', str(REPO_ROOT / 'rust/Cargo.toml')]
        binary = REPO_ROOT / 'rust/target/debug' / ('hash_checker.exe' if os.name == 'nt' else 'hash_checker')
        prefix = (str(binary),)
    else:
        output = build_dir / 'csharp'
        command = ['dotnet', 'build', str(REPO_ROOT / 'csharp/HashChecker'), '--output', str(output), '--nologo']
        prefix = ('dotnet', str(output / 'HashChecker.dll'))
    result = subprocess.run(command, cwd=REPO_ROOT, capture_output=True, text=True, encoding='utf-8', errors='replace', check=False)
    if result.returncode:
        raise RuntimeError(f'{language.name} build failed: {result.stdout} {result.stderr}')
    return LanguageCommand(language.name, prefix, language.required_tool)


def check_common(language):
    for case, algorithm, code in CASES:
        workspace = REPO_ROOT / 'testdata' / case
        arguments = ['--algorithm', algorithm, '--workspace', str(workspace)]
        text = invoke(language, arguments)
        assert text.returncode == code, f'{language.name} {case} text exit={text.returncode}'
        report = json_command(language, [*arguments, '--format', 'json'], code)
        if code in (0, 1):
            expected = (workspace / 'vendor_hash.txt').read_text(encoding='utf-8-sig').strip().lower()
            actual = hashlib.new(algorithm, (workspace / 'firmware.bin').read_bytes()).hexdigest()
            row = report['results'][0]
            assert row['path'] == 'firmware.bin' and row['expected_hash'] == expected and row['actual_hash'] == actual
            assert row['status'] == ('match' if code == 0 else 'mismatch')
        else:
            assert report['error']['code'] == code
        print(f'PASS {language.name:<7} {case:<22} text + JSON exit={code}')
    workspace = REPO_ROOT / 'testdata/ok-sha256'
    digest = (workspace / 'vendor_hash.txt').read_text().strip()
    base = ['--algorithm', 'sha256', '--workspace', str(workspace), '--format', 'json']
    direct = json_command(language, [*base, '--expected-hash', ' ' + digest.upper() + ' '], 0)
    assert direct['results'][0]['actual_hash'] == digest
    for extra, code in ((['--expected-hash', 'bad'], 4),
                        (['--expected-hash', digest, '--hash-file', 'vendor_hash.txt'], 2),
                        (['--expected-hash', digest, '--target-file', 'absent.bin'], 5),
                        (['--hash-file', 'absent.txt'], 3), (['--unsupported-option'], 2)):
        json_command(language, [*base, *extra], code)
    for algorithm in (None, 'bogus'):
        args = ['--format', 'json'] + ([] if algorithm is None else ['--algorithm', algorithm])
        assert json_command(language, args, 2)['algorithm'] is None
    version = invoke(language, ['--version'])
    assert version.returncode == 0 and version.stdout.strip() == '0.2.0'
    metadata = invoke(language, ['--capabilities'])
    assert metadata.returncode == 0
    supported = json.loads(metadata.stdout)
    assert supported['schema_version'] == 1
    assert {'single-verify', 'expected-hash', 'json-report'} <= set(supported['capabilities'])
    print(f'PASS {language.name:<7} common extensions (direct, errors, version, capabilities)')


def check_interoperability(go, rust, build_dir):
    root = build_dir / 'interop'
    root.mkdir()
    (root / 'sub').mkdir()
    (root / 'sub/日本語 file.txt').write_bytes(b'same')
    (root / 'changed.bin').write_bytes(b'before')
    (root / 'removed.bin').write_bytes(b'remove')
    manifest = root / 'checksums.txt'
    base = ['--algorithm', 'sha256', '--workspace', str(root), '--format', 'json']
    created = json_command(rust, [*base, '--create-manifest', str(manifest)], 0)
    assert created['summary']['match'] == 3
    initial = json_command(go, [*base, '--manifest', str(manifest)], 0)
    compared = json_command(rust, [*base, '--compare-manifest', str(manifest)], 0)
    assert initial['results'] == compared['results']
    original = manifest.read_bytes()
    json_command(rust, [*base, '--create-manifest', str(manifest)], 2)
    assert manifest.read_bytes() == original
    (root / 'changed.bin').write_bytes(b'after')
    (root / 'removed.bin').unlink()
    (root / 'added.bin').write_bytes(b'add')
    verified = json_command(go, [*base, '--manifest', str(manifest)], 3)
    assert all(verified['summary'][status] == 1 for status in ('match', 'mismatch', 'missing'))
    changes = json_command(rust, [*base, '--compare-manifest', str(manifest)], 1)
    assert all(changes['summary'][status] == 1 for status in ('match', 'added', 'removed', 'changed'))
    reports = []
    for index, report in enumerate((initial, verified, changes)):
        path = build_dir / f'report-{index}.json'
        path.write_text(json.dumps(report), encoding='utf-8')
        reports.append(str(path))
    output = build_dir / 'summary.csv'
    result = subprocess.run([sys.executable, str(REPO_ROOT / 'python/summarize_results.py'), *reports, '--output', str(output)], cwd=REPO_ROOT, capture_output=True, text=True, encoding='utf-8', check=False)
    assert result.returncode == 0, result.stderr
    with output.open(encoding='utf-8-sig', newline='') as stream:
        rows = {row['path']: row for row in csv.DictReader(stream)}
    assert rows['changed.bin']['mismatch'] == rows['changed.bin']['changed'] == '1'
    assert rows['removed.bin']['missing'] == rows['removed.bin']['removed'] == '1'
    assert rows['sub/日本語 file.txt']['checks'] == '3'
    bad = build_dir / 'bad-checksums.txt'
    for text in ('a' * 64 + ' *../outside.bin\n', 'a' * 64 + ' *a.bin\n' + 'b' * 64 + ' *A.bin\n'):
        bad.write_text(text, encoding='utf-8')
        json_command(go, [*base, '--manifest', str(bad)], 4)
        json_command(rust, [*base, '--compare-manifest', str(bad)], 4)
    outside = build_dir / 'outside'
    outside.mkdir()
    (outside / 'data.bin').write_bytes(b'external')
    link = root / 'linked'
    if os.name == 'nt':
        subprocess.run(['cmd', '/c', 'mklink', '/J', str(link), str(outside)], capture_output=True, check=True)
    else:
        link.symlink_to(outside, target_is_directory=True)
    try:
        bad.write_text(hashlib.sha256(b'external').hexdigest() + ' *linked/data.bin\n', encoding='utf-8')
        rejected = json_command(go, [*base, '--manifest', str(bad)], 3)
        assert rejected['results'][0]['status'] == 'error'
        json_command(rust, [*base, '--compare-manifest', str(manifest)], 3)
    finally:
        if os.name == 'nt':
            link.rmdir()
        else:
            link.unlink()
    print('PASS Rust -> Go -> Python interoperability (JSON, Unicode, changes, CSV, overwrite, traversal, links)')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--language', choices=[language.name for language in LANGUAGES], action='append')
    parser.add_argument('--allow-missing', action='store_true')
    args = parser.parse_args()
    selected = set(args.language or [language.name for language in LANGUAGES])
    failed = False
    with tempfile.TemporaryDirectory(prefix='hash_checker_cross_') as folder:
        prepared = {}
        for language in LANGUAGES:
            if language.name not in selected:
                continue
            if language.required_tool and shutil.which(language.required_tool) is None:
                print(f'MISSING {language.name}: {language.required_tool}')
                failed = failed or not args.allow_missing
                continue
            try:
                command = prepare_language(language, Path(folder))
                prepared[language.name] = command
                check_common(command)
            except (AssertionError, ValueError, KeyError, TypeError, OSError, RuntimeError) as exc:
                print(f'FAIL {language.name}: {exc}')
                failed = True
        if 'go' in prepared and 'rust' in prepared:
            try:
                check_interoperability(prepared['go'], prepared['rust'], Path(folder))
            except (AssertionError, ValueError, KeyError, TypeError, OSError, subprocess.CalledProcessError) as exc:
                print(f'FAIL interoperability: {exc}')
                failed = True
    print('Cross-language checks failed.' if failed else 'Cross-language checks passed.')
    return int(failed)


if __name__ == '__main__':
    raise SystemExit(main())
