#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
SPEC_PATH = REPO_ROOT / "docs" / "openapi" / "latest.json"
OUT_PATH = REPO_ROOT / "docs" / "openapi" / "paths.txt"


def generate_lines(spec: dict) -> list[str]:
    paths = spec.get("paths", {})
    methods = {"get", "post", "patch", "delete", "put", "head", "options"}
    lines: list[str] = []

    for path, ops in paths.items():
        for method in ops.keys():
            if method.lower() in methods:
                lines.append(f"{method.upper()} {path}")

    return sorted(lines)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Generate the OpenAPI method and path index"
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Fail if paths.txt differs from generated output",
    )
    args = parser.parse_args()

    if not SPEC_PATH.exists():
        raise SystemExit(f"Missing spec file: {SPEC_PATH}")

    spec = json.loads(SPEC_PATH.read_text())
    lines = generate_lines(spec)
    generated = "\n".join(lines) + "\n"

    if args.check:
        current = OUT_PATH.read_text() if OUT_PATH.exists() else ""
        if current != generated:
            print("docs/openapi/paths.txt is out of date.")
            print("Run: make update-openapi")
            return 1
        print(f"paths.txt is up to date ({len(lines)} entries).")
        return 0

    OUT_PATH.write_text(generated)
    print(f"Wrote {OUT_PATH} ({len(lines)} entries)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
