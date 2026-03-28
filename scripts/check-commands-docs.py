#!/usr/bin/env python3
"""Validate docs command lists against live CLI help output."""

from __future__ import annotations

import re
import subprocess
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
README_PATH = REPO_ROOT / "README.md"
COMMANDS_DOC_PATH = REPO_ROOT / "docs" / "COMMANDS.md"
DOC_EXAMPLE_PATHS = [
    README_PATH,
    REPO_ROOT / "docs" / "WORKFLOWS.md",
    REPO_ROOT / "docs" / "CI_CD.md",
    REPO_ROOT / "docs" / "CONTRIBUTING.md",
    REPO_ROOT / "CONTRIBUTING.md",
]

ROOT_FLAGS_WITH_VALUE = {"--profile", "--report", "--report-file"}


def run_help_text() -> str:
    proc = subprocess.run(
        ["go", "run", ".", "--help"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return proc.stderr or proc.stdout


def parse_live_commands(help_text: str) -> set[str]:
    commands = set()
    for line in help_text.splitlines():
        match = re.match(r"^\s{2}([a-z0-9-]+):\s", line)
        if match:
            commands.add(match.group(1))
    return commands


def parse_documented_commands(path: Path) -> set[str]:
    text = path.read_text()
    entries = set(re.findall(r"^- `([a-z0-9-]+)`", text, flags=re.MULTILINE))
    return {entry for entry in entries if not entry.startswith("--")}


def extract_top_level_command(line: str) -> str | None:
    tokens = line.strip().split()
    if len(tokens) < 2 or tokens[0] != "asc":
        return None

    i = 1
    while i < len(tokens) and tokens[i].startswith("--"):
        token = tokens[i]
        if "=" in token:
            i += 1
            continue
        if token in ROOT_FLAGS_WITH_VALUE and i + 1 < len(tokens):
            i += 2
            continue
        i += 1

    if i >= len(tokens):
        return None
    command = tokens[i]
    if command.startswith("<"):
        return None
    return command


def validate_document_examples(path: Path, live_commands: set[str]) -> list[str]:
    errors: list[str] = []
    for line_number, line in enumerate(path.read_text().splitlines(), start=1):
        stripped = line.strip()
        if not stripped.startswith("asc "):
            continue
        command = extract_top_level_command(stripped)
        if command is None:
            continue
        if command not in live_commands:
            errors.append(
                f"{path.relative_to(REPO_ROOT)}:{line_number}: unknown top-level command '{command}' in '{stripped}'"
            )
    return errors


def main() -> int:
    help_text = run_help_text()
    live_commands = parse_live_commands(help_text)
    doc_commands = parse_documented_commands(COMMANDS_DOC_PATH)

    missing = sorted(live_commands - doc_commands)
    extra = sorted(doc_commands - live_commands)
    example_errors: list[str] = []
    for path in DOC_EXAMPLE_PATHS:
        example_errors.extend(validate_document_examples(path, live_commands))

    problems: list[str] = []
    if missing:
        problems.append(f"Missing in docs/COMMANDS.md: {', '.join(missing)}")
    if extra:
        problems.append(f"Extra in docs/COMMANDS.md: {', '.join(extra)}")
    problems.extend(example_errors)

    if problems:
        print("Command docs are out of sync with live CLI help:")
        for problem in problems:
            print(f"- {problem}")
        print("Run 'go run . --help' and update docs/COMMANDS.md and README.md examples.")
        return 1

    print(
        f"Command docs are up to date ({len(live_commands)} commands, example docs validated)."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
