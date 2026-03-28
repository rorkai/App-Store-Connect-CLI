#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def version_is_documented(changelog: str, version: str) -> bool:
    version = version.lstrip("v")
    pattern = re.compile(rf"(?m)^#+\s+v?{re.escape(version)}\s*$")
    return bool(pattern.search(changelog))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Verify that the website changelog includes the release version."
    )
    parser.add_argument("version", help="Release version, with or without a leading v.")
    args = parser.parse_args(argv)

    repo_root = Path(__file__).resolve().parents[1]
    changelog_path = repo_root / "resources" / "changelog.mdx"
    changelog = changelog_path.read_text()
    if version_is_documented(changelog, args.version):
        print(f"Release docs check passed for {args.version}.")
        return 0

    print(
        f"Release docs check failed: {changelog_path} does not mention {args.version}.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
