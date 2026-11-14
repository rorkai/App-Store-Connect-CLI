#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
SPEC_PATH = REPO_ROOT / "docs" / "openapi" / "latest.json"
OUT_PATH = REPO_ROOT / "docs" / "openapi" / "paths.txt"
WEB_SPEC_PATH = REPO_ROOT / "docs" / "openapi" / "web" / "latest.json"
WEB_OUT_PATH = REPO_ROOT / "docs" / "openapi" / "web" / "paths.txt"


def generate_lines(spec: dict) -> list[str]:
    paths = spec.get("paths", {})
    methods = {"get", "post", "patch", "delete", "put", "head", "options"}
    lines: list[str] = []

    for path, ops in paths.items():
        for method in ops.keys():
            if method.lower() in methods:
                lines.append(f"{method.upper()} {path}")

    return sorted(lines)


def canonical_text(spec: dict) -> str:
    spec = dict(spec)
    spec["paths"] = dict(sorted(spec.get("paths", {}).items()))
    components = dict(spec.get("components", {}))
    if "schemas" in components:
        components["schemas"] = dict(sorted(components["schemas"].items()))
        spec["components"] = components
    return json.dumps(spec, indent=2, ensure_ascii=False) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Generate the OpenAPI method and path index"
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Fail if an index differs from generated output or the web schema is not in canonical form",
    )
    args = parser.parse_args()

    # The published snapshot keeps its downloaded bytes; the web schema is
    # ours, so it is also kept in canonical form for small diffs.
    indexes = (
        (SPEC_PATH, OUT_PATH, "docs/openapi/paths.txt", False),
        (WEB_SPEC_PATH, WEB_OUT_PATH, "docs/openapi/web/paths.txt", True),
    )
    status = 0
    for spec_path, out_path, label, canonical in indexes:
        if not spec_path.exists():
            raise SystemExit(f"Missing spec file: {spec_path}")

        text = spec_path.read_text()
        spec = json.loads(text)
        lines = generate_lines(spec)
        generated = "\n".join(lines) + "\n"
        formatted = canonical_text(spec) if canonical else text

        if args.check:
            if text != formatted:
                print(f"{spec_path.name} for {label} is not in canonical form.")
                print("Run: make update-openapi")
                status = 1
            current = out_path.read_text() if out_path.exists() else ""
            if current != generated:
                print(f"{label} is out of date.")
                print("Run: make update-openapi")
                status = 1
                continue
            print(f"{label} is up to date ({len(lines)} entries).")
            continue

        if text != formatted:
            spec_path.write_text(formatted)
        out_path.write_text(generated)
        print(f"Wrote {out_path} ({len(lines)} entries)")
    return status


if __name__ == "__main__":
    raise SystemExit(main())
