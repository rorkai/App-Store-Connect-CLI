#!/usr/bin/env python3
"""Validate repository-scoped Codex skills and their UI metadata."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SKILLS_ROOT = ROOT / ".agents" / "skills"
AGENTS_PATH = ROOT / "AGENTS.md"
EXPECTED_SKILLS = {
    "audit-asc-pr",
    "develop-asc-change",
    "release-asc-cli",
    "review-wall-of-apps-prs",
    "sync-asc-skills",
    "triage-asc-issue",
    "watch-asc-pr",
}
MARKDOWN_LINK = re.compile(r"\[[^\]]+\]\(([^)]+)\)")


def parse_frontmatter(text: str, path: Path) -> tuple[dict[str, str], str]:
    if not text.startswith("---\n"):
        raise ValueError(f"{path}: missing YAML frontmatter")

    try:
        raw_frontmatter, body = text[4:].split("\n---\n", 1)
    except ValueError as exc:
        raise ValueError(f"{path}: unterminated YAML frontmatter") from exc

    values: dict[str, str] = {}
    for line in raw_frontmatter.splitlines():
        if not line.strip():
            continue
        if line.startswith((" ", "\t")) or ":" not in line:
            raise ValueError(f"{path}: unsupported frontmatter line: {line!r}")
        key, value = line.split(":", 1)
        values[key.strip()] = value.strip().strip('"')
    return values, body


def yaml_string(text: str, key: str) -> str | None:
    match = re.search(rf'^\s+{re.escape(key)}:\s+"([^"]*)"\s*$', text, re.MULTILINE)
    return match.group(1) if match else None


def validate_skill(skill_dir: Path, agents_text: str) -> list[str]:
    errors: list[str] = []
    skill_path = skill_dir / "SKILL.md"
    metadata_path = skill_dir / "agents" / "openai.yaml"

    if not skill_path.is_file():
        return [f"{skill_dir}: missing SKILL.md"]
    if not metadata_path.is_file():
        return [f"{skill_dir}: missing agents/openai.yaml"]

    text = skill_path.read_text(encoding="utf-8")
    try:
        frontmatter, body = parse_frontmatter(text, skill_path)
    except ValueError as exc:
        return [str(exc)]

    if set(frontmatter) != {"name", "description"}:
        errors.append(f"{skill_path}: frontmatter must contain only name and description")
    if frontmatter.get("name") != skill_dir.name:
        errors.append(f"{skill_path}: name must match directory {skill_dir.name!r}")
    if len(frontmatter.get("description", "")) < 50:
        errors.append(f"{skill_path}: description is too short for reliable triggering")
    if not body.strip():
        errors.append(f"{skill_path}: instructions are empty")
    if "TODO" in text:
        errors.append(f"{skill_path}: unresolved TODO placeholder")

    for raw_target in MARKDOWN_LINK.findall(body):
        target = raw_target.split("#", 1)[0].strip()
        if not target or target.startswith(("http://", "https://")):
            continue
        resolved = (skill_path.parent / target).resolve()
        if not resolved.is_relative_to(ROOT):
            errors.append(f"{skill_path}: local link escapes repository: {target}")
        elif not resolved.exists():
            errors.append(f"{skill_path}: missing local link target: {target}")

    metadata = metadata_path.read_text(encoding="utf-8")
    display_name = yaml_string(metadata, "display_name")
    short_description = yaml_string(metadata, "short_description")
    default_prompt = yaml_string(metadata, "default_prompt")
    if not display_name:
        errors.append(f"{metadata_path}: missing quoted display_name")
    if not short_description or not 25 <= len(short_description) <= 64:
        errors.append(f"{metadata_path}: short_description must be 25-64 characters")
    if not default_prompt or f"${skill_dir.name}" not in default_prompt:
        errors.append(f"{metadata_path}: default_prompt must mention ${skill_dir.name}")

    if f"${skill_dir.name}" not in agents_text:
        errors.append(f"{AGENTS_PATH}: missing workflow route for ${skill_dir.name}")
    return errors


def main() -> int:
    errors: list[str] = []
    if not SKILLS_ROOT.is_dir():
        errors.append(f"{SKILLS_ROOT}: skills directory is missing")
        skill_dirs: list[Path] = []
    else:
        skill_dirs = sorted(path for path in SKILLS_ROOT.iterdir() if path.is_dir())

    actual_names = {path.name for path in skill_dirs}
    for missing in sorted(EXPECTED_SKILLS - actual_names):
        errors.append(f"{SKILLS_ROOT}: missing expected skill {missing}")

    agents_text = AGENTS_PATH.read_text(encoding="utf-8")
    for skill_dir in skill_dirs:
        errors.extend(validate_skill(skill_dir, agents_text))

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1

    print(f"Validated {len(skill_dirs)} repository agent skills.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
