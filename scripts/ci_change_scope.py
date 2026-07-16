#!/usr/bin/env python3
"""Classify changed paths for conservative CI test selection."""

from __future__ import annotations

import argparse
import sys
from collections.abc import Iterable


WALL_SOURCE = "docs/wall-of-apps.json"
OPENAPI_SNAPSHOT = "docs/openapi/latest.json"
EMBEDDED_GUIDES = {"docs/API_NOTES.md", "docs/WORKFLOWS.md"}
WEBSITE_FILES = {".mintignore", "docs.json"}
WEBSITE_PREFIXES = (
    ".mintlify/",
    "commands/",
    "concepts/",
    "configuration/",
    "guides/",
    "resources/",
)
TELEMETRY_PREFIX = "internal/telemetry/"
TELEMETRY_CLI_PREFIX = "internal/cli/telemetry/"
def path_kind(path: str) -> str:
    if path == WALL_SOURCE:
        return "wall"
    if path == OPENAPI_SNAPSHOT or path in EMBEDDED_GUIDES:
        return "full"
    if path.startswith(TELEMETRY_CLI_PREFIX):
        return "full"
    if path.startswith(TELEMETRY_PREFIX):
        return "telemetry"
    if path.endswith(".go"):
        return "full"
    if path in WEBSITE_FILES or path.startswith(WEBSITE_PREFIXES):
        return "website"
    if "/" not in path and path.endswith(".mdx"):
        return "website"
    if path.startswith("docs/"):
        return "docs"
    if "/" not in path and path.endswith(".md"):
        return "docs"
    return "full"


def classify(paths: Iterable[str]) -> str:
    normalized = [path.strip() for path in paths if path.strip()]
    if not normalized:
        return "full"
    if normalized == [WALL_SOURCE]:
        return "wall"

    kinds = {path_kind(path) for path in normalized}
    if "wall" in kinds or "full" in kinds:
        return "full"

    if kinds == {"telemetry"}:
        return "telemetry"
    if kinds == {"website"}:
        return "website"
    if kinds <= {"docs", "website"}:
        return "docs"
    return "full"


def affects_website(paths: Iterable[str]) -> bool:
    return any(path_kind(path.strip()) == "website" for path in paths if path.strip())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--github-output", action="store_true")
    args = parser.parse_args()
    paths = list(sys.stdin)

    if args.github_output:
        print(f"scope={classify(paths)}")
        print(f"website_affected={str(affects_website(paths)).lower()}")
        return
    print(classify(paths))


if __name__ == "__main__":
    main()
