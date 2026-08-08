#!/usr/bin/env python3
"""Verify that an ASC release checksum manifest covers every asset exactly once."""

from __future__ import annotations

import argparse
import hashlib
import re
import sys
from pathlib import Path
from typing import NoReturn

from release_rehearsal import expected_artifact_names


CHECKSUM_LINE = re.compile(r"^([0-9a-fA-F]{64}) ([ *])(.+)$")


def fail(message: str) -> NoReturn:
    print(f"release asset verification failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--release-dir", required=True, type=Path)
    parser.add_argument("--version", required=True)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    release_dir = args.release_dir
    checksum_name = f"asc_{args.version}_checksums.txt"
    checksum_path = release_dir / checksum_name

    if not release_dir.is_dir():
        fail(f"release directory does not exist: {release_dir}")
    if not checksum_path.is_file() or checksum_path.is_symlink():
        fail(f"missing regular checksum manifest: {checksum_name}")

    actual_names: list[str] = []
    for entry in release_dir.iterdir():
        if entry.name == checksum_name:
            continue
        if not entry.is_file() or entry.is_symlink():
            fail(f"unexpected non-file release entry: {entry.name}")
        if entry.stat().st_size == 0:
            fail(f"release asset is empty: {entry.name}")
        actual_names.append(entry.name)

    canonical_names = set(expected_artifact_names(args.version))
    actual_set = set(actual_names)
    if actual_set != canonical_names:
        missing = sorted(canonical_names - actual_set)
        unexpected = sorted(actual_set - canonical_names)
        fail(
            "canonical release asset mismatch; "
            f"missing assets={missing}, unexpected assets={unexpected}"
        )

    expected_hashes: dict[str, str] = {}
    for line_number, line in enumerate(
        checksum_path.read_text(encoding="utf-8").splitlines(), start=1
    ):
        match = CHECKSUM_LINE.fullmatch(line)
        if match is None:
            fail(f"malformed checksum line {line_number}")
        digest, _, name = match.groups()
        if (
            Path(name).name != name
            or "/" in name
            or "\\" in name
            or name in {".", "..", checksum_name}
        ):
            fail(f"unsafe checksum filename on line {line_number}: {name}")
        if name in expected_hashes:
            fail(f"duplicate checksum entry: {name}")
        expected_hashes[name] = digest.lower()

    expected_set = set(expected_hashes)
    if actual_set != expected_set:
        unexpected = sorted(actual_set - expected_set)
        missing = sorted(expected_set - actual_set)
        fail(
            "checksum coverage mismatch; "
            f"missing entries={missing}, unknown entries={unexpected}"
        )

    for name in sorted(actual_names):
        digest = hashlib.sha256()
        with (release_dir / name).open("rb") as asset:
            for chunk in iter(lambda: asset.read(1024 * 1024), b""):
                digest.update(chunk)
        if digest.hexdigest() != expected_hashes[name]:
            fail(f"checksum mismatch: {name}")

    print(f"Verified {len(actual_names)} release asset checksum(s).")


if __name__ == "__main__":
    main()
