#!/usr/bin/env python3
from __future__ import annotations

import argparse
import sys
from pathlib import Path

from doc_links import extract_targets, is_within_root, normalize_target


DEFAULT_DOC_GLOBS = (
    "README.md",
    "CONTRIBUTING.md",
    "SUPPORT.md",
    "docs/**/*.md",
)


def iter_doc_files(repo_root: Path, explicit_paths: list[str]) -> list[Path]:
    if explicit_paths:
        files = []
        for raw in explicit_paths:
            path = (repo_root / raw).resolve()
            if path.is_file():
                files.append(path)
        return sorted(set(files))

    files = set()
    for pattern in DEFAULT_DOC_GLOBS:
        files.update(path for path in repo_root.glob(pattern) if path.is_file())
    return sorted(files)


def check_files(repo_root: Path, files: list[Path]) -> list[str]:
    errors: list[str] = []
    repo_root = repo_root.resolve()
    for path in files:
        path = path.resolve()
        for target in extract_targets(path.read_text()):
            normalized = normalize_target(target, allow_root_relative=False)
            if normalized is None:
                continue
            resolved = (path.parent / normalized).resolve()
            rel_source = path.relative_to(repo_root)
            if not is_within_root(repo_root, resolved):
                errors.append(
                    f"{rel_source}: local docs target {normalized!r} escapes repository root"
                )
                continue
            if not resolved.exists():
                errors.append(
                    f"{rel_source}: missing local docs target {normalized!r}"
                )
    return errors


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate local links in repo docs.")
    parser.add_argument("paths", nargs="*", help="Optional subset of doc files to check.")
    args = parser.parse_args(argv)

    repo_root = Path(__file__).resolve().parents[1]
    files = iter_doc_files(repo_root, args.paths)
    errors = check_files(repo_root, files)
    if errors:
        print("Repository docs validation failed:", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1

    print(f"Repository docs validation passed for {len(files)} file(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
