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
    # ffcli writes successful help output to stdout. A cold Go module cache can
    # add download diagnostics to stderr, which must not replace that output.
    return proc.stdout or proc.stderr


def parse_help(help_text: str) -> tuple[str, list[tuple[str, str]], list[tuple[str, list[tuple[str, str]]]]]:
    usage = "asc <subcommand> [flags]"
    flags: list[tuple[str, str]] = []
    groups: list[tuple[str, list[tuple[str, str]]]] = []

    in_flags = False
    in_usage = False
    current_group_index: int | None = None

    for line in help_text.splitlines():
        stripped = line.strip()

        # Only the USAGE section defines the usage pattern; sample invocations
        # elsewhere in the help text must not overwrite it. Any unindented
        # heading ends the section, even one this parser does not model.
        if in_usage:
            if line.startswith("  asc "):
                usage = stripped
                in_usage = False
            elif stripped and not line.startswith(" "):
                in_usage = False

        if stripped == "USAGE":
            in_usage = True
            in_flags = False
            current_group_index = None
            continue
        if stripped == "FLAGS":
            in_usage = False
            in_flags = True
            current_group_index = None
            continue

        group_match = re.match(r"^([A-Z0-9 &/-]+) COMMANDS$", stripped)
        if group_match:
            in_usage = False
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
            "- Any flag that takes a value accepts `@env:NAME` or `@file:PATH` to read it from the environment or a file; escape a literal leading `@` as `@@`. `--output` and the root `--profile`, `--report`, and `--report-file` are always read literally.",
            "- Prefer explicit flags and deterministic outputs in CI scripts.",
            "",
            "## High-Signal Examples",
            "",
            "```bash",
            "# List apps",
            "asc apps list --output table",
            "",
            "# Pause and resume Apple Ads campaigns",
            "asc ads campaigns pause --campaign CAMPAIGN_ID --ad-account AD_ACCOUNT_ID",
            "asc ads campaigns resume --campaign CAMPAIGN_ID --ad-account AD_ACCOUNT_ID --confirm",
            "",
            "# Manage App Store compatibility opt-ins through a web session",
            "asc web apps compatibility view --app \"123456789\"",
            "asc web apps compatibility edit --app \"123456789\" --ios-app-on-mac=false --ios-app-on-vision-pro=false",
            "",
            "# Upload a build",
            "asc builds upload --app \"123456789\" --ipa \"/path/to/MyApp.ipa\"",
            "",
            "# Generate local Xcode metadata before archiving",
            "asc xcode inject --manifest .asc/deployment.json --set version=1.2.3 --set build_number=42 --dry-run --output json",
            "",
            "# Inspect the selected local Xcode toolchain without changing host state",
            "asc xcode doctor --output json",
            "",
            "# Install and verify one signed IPA on an exact connected device",
            "asc xcode install --ipa .asc/artifacts/App.ipa --device-id COREDEVICE_IDENTIFIER --timeout 5m --output json",
            "",
            "# Staple and validate a notarized macOS artifact locally",
            "ASC_BYPASS_KEYCHAIN=1 asc notarization staple --file ./MyApp.dmg --confirm --output json",
            "ASC_BYPASS_KEYCHAIN=1 asc notarization validate --file ./MyApp.dmg --output json",
            "",
            "# Run local Xcode tests with structured results",
            "asc xcode test --project App.xcodeproj --scheme App --destination 'platform=iOS Simulator,name=iPhone 17 Pro' --output json",
            "",
            "# Plan, confirm, resume, check status, and live-verify a private ad hoc distribution run",
            "asc distribute plan --archive-path ./App.xcarchive --config .asc/distribution.json --plan .asc/distribution/plan.json --state-dir .asc/distribution/runs --output json",
            "asc distribute apply --plan .asc/distribution/plan.json --confirm PLAN_HASH --output json",
            "asc distribute resume --run RUN_ID --state-dir .asc/distribution/runs --output json",
            "asc distribute status --run RUN_ID --state-dir .asc/distribution/runs --output json",
            "asc distribute verify --run RUN_ID --state-dir .asc/distribution/runs --timeout 30s --output json",
            "",
            "# Stage an App Store version before submission",
            "asc release stage --app \"123456789\" --version \"1.2.3\" --build-id \"BUILD_ID\" --copy-metadata-from \"1.2.2\" --dry-run",
            "",
            "# Publish an App Store version (high-level)",
            "asc publish appstore --app \"123456789\" --ipa \"/path/to/MyApp.ipa\" --version \"1.2.3\"",
            "asc publish appstore --app \"123456789\" --ipa \"/path/to/MyApp.ipa\" --version \"1.2.3\" --submit --confirm",
            "asc status --app \"123456789\"",
            "",
            "# Canonical readiness and lower-level submission lifecycle flow",
            "asc validate --app \"123456789\" --version \"1.2.3\"",
            "asc validate --app \"123456789\" --version \"1.2.3\" --check-urls",
            "asc submit status --version-id \"VERSION_ID\"",
            "asc submit cancel --version-id \"VERSION_ID\" --confirm",
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
