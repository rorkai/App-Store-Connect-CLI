#!/usr/bin/env python3
"""Generate docs/COMMANDS.md from live `asc --help` output."""

from __future__ import annotations

import argparse
import re
import subprocess
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
OUTPUT_PATH = REPO_ROOT / "docs" / "COMMANDS.md"

GROUP_TITLE_MAP = {
    "GETTING STARTED COMMANDS": "Getting Started",
    "ANALYTICS & FINANCE COMMANDS": "Analytics and Finance",
    "APP MANAGEMENT COMMANDS": "App Management",
    "TESTFLIGHT & BUILD COMMANDS": "TestFlight and Builds",
    "REVIEW & RELEASE COMMANDS": "Review and Release",
    "MONETIZATION COMMANDS": "Monetization",
    "SIGNING COMMANDS": "Signing",
    "TEAM & ACCESS COMMANDS": "Team and Access",
    "AUTOMATION COMMANDS": "Automation",
    "UTILITY COMMANDS": "Utility",
    "ADDITIONAL COMMANDS": "Additional",
}


def run_help_text() -> str:
    proc = subprocess.run(
        ["go", "run", ".", "--help"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return proc.stderr or proc.stdout


def parse_help(help_text: str) -> tuple[str, list[tuple[str, str]], list[tuple[str, list[tuple[str, str]]]]]:
    usage = "asc <subcommand> [flags]"
    flags: list[tuple[str, str]] = []
    groups: list[tuple[str, list[tuple[str, str]]]] = []

    in_flags = False
    current_group_index: int | None = None

    for line in help_text.splitlines():
        if line.startswith("  asc "):
            usage = line.strip()

        stripped = line.strip()
        if stripped == "FLAGS":
            in_flags = True
            current_group_index = None
            continue

        group_match = re.match(r"^([A-Z0-9 &/-]+) COMMANDS$", stripped)
        if group_match:
            in_flags = False
            groups.append((group_match.group(0), []))
            current_group_index = len(groups) - 1
            continue

        command_match = re.match(r"^\s{2}([a-z0-9-]+):\s+(.*\S)\s*$", line)
        if command_match and current_group_index is not None:
            command, description = command_match.group(1), command_match.group(2)
            groups[current_group_index][1].append((command, description))
            continue

        flag_match = re.match(r"^\s{2}(--[a-z0-9-]+)\s+(.*\S)\s*$", line)
        if flag_match and in_flags:
            flag, description = flag_match.group(1), flag_match.group(2)
            flags.append((flag, description))

    return usage, flags, groups


def normalize_group_title(raw_title: str) -> str:
    return GROUP_TITLE_MAP.get(raw_title, raw_title.title())


def render(usage: str, flags: list[tuple[str, str]], groups: list[tuple[str, list[tuple[str, str]]]]) -> str:
    lines: list[str] = [
        "# Command Reference Guide",
        "",
        "This file is generated from live CLI help output.",
        "For authoritative command behavior, also use:",
        "",
        "```bash",
        "asc --help",
        "asc <command> --help",
        "asc <command> <subcommand> --help",
        "```",
        "",
        "To regenerate:",
        "",
        "```bash",
        "make generate-command-docs",
        "```",
        "",
        "## Usage Pattern",
        "",
        "```bash",
        usage,
        "```",
        "",
        "## Global Flags",
        "",
    ]

    for flag, description in flags:
        lines.append(f"- `{flag}` - {description}")

    lines.extend(["", "## Command Families", ""])

    for raw_title, commands in groups:
        title = normalize_group_title(raw_title)
        lines.append(f"### {title}")
        lines.append("")
        for command, description in commands:
            lines.append(f"- `{command}` - {description}")
        lines.append("")

    lines.extend(
        [
            "## Scripting Tips",
            "",
            "- Output defaults are TTY-aware: interactive terminals default to `table`, while piped/non-interactive output defaults to minified `json`.",
            "- Use `--output table` or `--output markdown` for explicit human-readable output.",
            "- Use `--output json` for explicit machine-readable output.",
            "- Use `--paginate` on list commands to fetch all pages automatically.",
            "- Use `--limit` and `--next` for manual pagination control.",
            "- Prefer explicit flags and deterministic outputs in CI scripts.",
            "",
            "## High-Signal Examples",
            "",
            "```bash",
            "# List apps",
            "asc apps list --output table",
            "",
            "# Upload a build",
            "asc builds upload --app \"123456789\" --ipa \"/path/to/MyApp.ipa\"",
            "",
            "# Release an App Store version (high-level)",
            "asc release run --app \"123456789\" --version \"1.2.3\" --build \"BUILD_ID\" --metadata-dir \"./metadata/version/1.2.3\" --dry-run",
            "asc release run --app \"123456789\" --version \"1.2.3\" --build \"BUILD_ID\" --metadata-dir \"./metadata/version/1.2.3\" --confirm",
            "asc status --app \"123456789\"",
            "",
            "# Lower-level review/submit flow",
            "asc validate --app \"123456789\" --version \"1.2.3\"",
            "asc submit create --app \"123456789\" --version \"1.2.3\" --build \"BUILD_ID\" --confirm",
            "",
            "# Run a local automation workflow",
            "asc workflow run release",
            "```",
            "",
            "## Related Documentation",
            "",
            "- [../README.md](../README.md) - onboarding and common workflows",
            "- [API_NOTES.md](API_NOTES.md) - API-specific behavior and caveats",
            "- [TESTING.md](TESTING.md) - test strategy and patterns",
            "- [CONTRIBUTING.md](CONTRIBUTING.md) - contribution and dev workflow",
            "",
        ]
    )

    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate docs/COMMANDS.md from live CLI help")
    parser.add_argument(
        "--check",
        action="store_true",
        help="Fail if docs/COMMANDS.md differs from generated output",
    )
    args = parser.parse_args()

    usage, flags, groups = parse_help(run_help_text())
    generated = render(usage, flags, groups)

    if args.check:
        current = OUTPUT_PATH.read_text() if OUTPUT_PATH.exists() else ""
        if current != generated:
            print("docs/COMMANDS.md is out of date.")
            print("Run: make generate-command-docs")
            return 1
        print("docs/COMMANDS.md is up to date.")
        return 0

    OUTPUT_PATH.write_text(generated)
    print(f"Generated {OUTPUT_PATH.relative_to(REPO_ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
