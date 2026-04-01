#!/usr/bin/env python3
from __future__ import annotations

import argparse
import functools
import re
import shlex
import subprocess
import tempfile
from dataclasses import dataclass
from pathlib import Path


ROOT_COMMAND_RE = re.compile(r"^\s{2}([a-z0-9-]+):\s")
SUBCOMMAND_RE = re.compile(r"^\s{2}([a-z0-9-]+)\s{2,}")
FLAG_RE = re.compile(r"^\s{2}(--[a-z0-9-]+)\s+(.*\S)\s*$")
DEPRECATED_USE_RE = re.compile(r"DEPRECATED:\s+use\s+`([^`]+)`")
DEPRECATED_ALIAS_RE = re.compile(r"Deprecated compatibility alias for\s+`([^`]+)`")
PLACEHOLDER_PATTERNS = (
    re.compile(r"\$\{\{[^}]+\}\}"),
    re.compile(r"<<[^>]+>>"),
    re.compile(r"\$\{[^}]+\}"),
    re.compile(r"\$[A-Za-z_][A-Za-z0-9_]*"),
)
META_TOKEN_RE = re.compile(r"^<[^>]+>$|^\[[^\]]+\]$")
GENERIC_TOKENS = {"command", "subcommand", "subcmd"}
SHELL_OPERATORS = {"|", ";", ">", "<"}
ELLIPSIS_TOKENS = {"...", "…"}
REQUIRED_FLAGS_BY_COMMAND: dict[tuple[str, ...], set[str]] = {
    ("submit", "create"): {"--build", "--confirm"},
}
BOOLEAN_FLAG_OVERRIDES = {"--api-debug", "--debug", "--retry-log"}
HIDDEN_DEPRECATED_ALIAS_FLAGS: dict[tuple[str, ...], dict[str, bool]] = {
    # DeprecatedUsageFunc intentionally hides FLAGS in help output for
    # compatibility aliases, but we still need accurate flag validation so docs
    # examples fail on deprecations instead of bogus unknown-flag errors.
    ("submit", "create"): {
        "--app": False,
        "--build": False,
        "--confirm": True,
        "--output": False,
        "--platform": False,
        "--pretty": True,
        "--version": False,
        "--version-id": False,
    },
    ("submit", "preflight"): {
        "--app": False,
        "--output": False,
        "--platform": False,
        "--pretty": True,
        "--version": False,
    },
}


@dataclass(frozen=True)
class CommandSpec:
    path: tuple[str, ...]
    usage: str
    flags: dict[str, bool]
    subcommands: set[str]


@dataclass(frozen=True)
class Example:
    path: Path
    line_number: int
    raw: str
    tokens: tuple[str, ...]
    source: str = "fenced"


def parse_help_text(help_text: str, *, is_root: bool) -> CommandSpec:
    usage = ""
    flags: dict[str, bool] = {}
    subcommands: set[str] = set()
    in_flags = False
    in_subcommands = False
    expect_usage_line = False

    for line in help_text.splitlines():
        stripped = line.strip()
        if stripped == "USAGE":
            expect_usage_line = True
            continue
        if expect_usage_line and line.startswith("  asc "):
            usage = line.strip()
            expect_usage_line = False
            continue
        if expect_usage_line and stripped:
            expect_usage_line = False
        if stripped == "FLAGS":
            in_flags = True
            in_subcommands = False
            continue
        if stripped == "SUBCOMMANDS":
            in_subcommands = True
            in_flags = False
            continue
        if stripped.isupper() and stripped not in {"FLAGS", "SUBCOMMANDS"}:
            in_flags = False
            in_subcommands = False

        if in_flags:
            match = FLAG_RE.match(line)
            if match:
                flag, description = match.group(1), match.group(2)
                flags[flag] = (
                    flag in BOOLEAN_FLAG_OVERRIDES
                    or description.rstrip().endswith("(default: false)")
                    or description.rstrip().endswith("(default: true)")
                )
            continue

        if is_root:
            match = ROOT_COMMAND_RE.match(line)
            if match:
                subcommands.add(match.group(1))
            continue

        if in_subcommands:
            match = SUBCOMMAND_RE.match(line)
            if match:
                subcommands.add(match.group(1))

    return CommandSpec(path=(), usage=usage, flags=flags, subcommands=subcommands)


def command_help(binary_path: Path, path: tuple[str, ...]) -> str:
    proc = subprocess.run(
        [str(binary_path), *path, "--help"],
        check=True,
        capture_output=True,
        text=True,
    )
    return proc.stderr or proc.stdout


@functools.lru_cache(maxsize=None)
def path_help(binary_path: Path, path: tuple[str, ...]) -> str:
    proc = subprocess.run(
        [str(binary_path), *path, "--help"],
        check=False,
        capture_output=True,
        text=True,
    )
    return proc.stderr or proc.stdout


def build_command_index(binary_path: Path) -> dict[tuple[str, ...], CommandSpec]:
    root_help = command_help(binary_path, ())
    root_spec = parse_help_text(root_help, is_root=True)
    index: dict[tuple[str, ...], CommandSpec] = {
        (): CommandSpec(
            path=(),
            usage=root_spec.usage,
            flags=root_spec.flags,
            subcommands=root_spec.subcommands,
        )
    }
    queue = [()]

    while queue:
        path = queue.pop(0)
        for subcommand in sorted(index[path].subcommands):
            child_path = (*path, subcommand)
            child_help = command_help(binary_path, child_path)
            child_spec = parse_help_text(child_help, is_root=False)
            index[child_path] = CommandSpec(
                path=child_path,
                usage=child_spec.usage,
                flags=child_spec.flags,
                subcommands=child_spec.subcommands,
            )
            queue.append(child_path)

    return index


def iter_fenced_blocks(text: str) -> list[tuple[int, list[str]]]:
    blocks: list[tuple[int, list[str]]] = []
    in_block = False
    block_lines: list[str] = []
    block_start = 0

    for line_number, line in enumerate(text.splitlines(), start=1):
        if line.lstrip().startswith("```"):
            if in_block:
                blocks.append((block_start, block_lines))
                in_block = False
                block_lines = []
                block_start = 0
            else:
                in_block = True
                block_start = line_number + 1
            continue
        if in_block:
            block_lines.append(line)

    return blocks


def iter_logical_lines(start_line: int, lines: list[str]) -> list[tuple[int, str]]:
    logical: list[tuple[int, str]] = []
    current: list[str] = []
    current_start = start_line

    for offset, raw in enumerate(lines):
        line_number = start_line + offset
        line = raw.rstrip()
        if not current:
            current_start = line_number
        if current:
            current.append(line.lstrip())
        else:
            current.append(line)
        if current[-1].endswith("\\"):
            current[-1] = current[-1][:-1].rstrip()
            continue
        logical.append((current_start, " ".join(current).strip()))
        current = []

    if current:
        logical.append((current_start, " ".join(current).strip()))

    return logical


def normalize_placeholders(text: str) -> str:
    for pattern in PLACEHOLDER_PATTERNS:
        text = pattern.sub("PLACEHOLDER", text)
    return text


def truncate_shell_expression(text: str) -> str:
    quote: str | None = None
    escaped = False
    result: list[str] = []
    i = 0
    while i < len(text):
        ch = text[i]
        if escaped:
            result.append(ch)
            escaped = False
            i += 1
            continue

        if ch == "\\" and quote != "'":
            result.append(ch)
            escaped = True
            i += 1
            continue

        if ch in {"'", '"'}:
            if quote == ch:
                quote = None
            elif quote is None:
                quote = ch
            result.append(ch)
            i += 1
            continue

        if quote is None:
            if ch == "<":
                placeholder = re.match(r"<[^>]+>", text[i:])
                if placeholder:
                    result.append(placeholder.group(0))
                    i += len(placeholder.group(0))
                    continue
            if ch == "#" and (i == 0 or text[i - 1].isspace()):
                break
            if ch in SHELL_OPERATORS:
                if ch in {">", "<"} and len(result) >= 2 and result[-1].isdigit() and result[-2].isspace():
                    result.pop()
                break
            if ch == "&" and i + 1 < len(text) and text[i + 1] == "&":
                break

        result.append(ch)
        i += 1

    return "".join(result).strip()


def clean_command_fragment(text: str) -> str:
    cleaned = normalize_placeholders(text)
    cleaned = truncate_shell_expression(cleaned)
    cleaned = cleaned.rstrip(",")
    while cleaned and cleaned[-1] in {")", "}"}:
        cleaned = cleaned[:-1].rstrip()
    return cleaned


def is_command_prefix(prefix: str) -> bool:
    stripped = prefix.rstrip()
    if not stripped:
        return True
    if stripped in {"$", "(", "-", "if", "then", "do"}:
        return True
    if re.search(r'(^|[\s{[(])(?:run|command):\s*["\']?$', stripped):
        return True
    if re.search(r"=\s*\$\($", stripped):
        return True
    if stripped.endswith("&&") or stripped.endswith("||"):
        return True
    return False


def should_skip_tokens(tokens: tuple[str, ...]) -> bool:
    if any(META_TOKEN_RE.match(token) for token in tokens):
        return True
    if any(token in ELLIPSIS_TOKENS for token in tokens):
        return True
    if any(token.lower().startswith("[flags") for token in tokens):
        return True
    lowered = {token.lower() for token in tokens}
    if lowered & (GENERIC_TOKENS | {"cli"}):
        return True
    return False


def extract_fenced_examples(path: Path, text: str) -> list[Example]:
    examples: list[Example] = []
    for block_start, block_lines in iter_fenced_blocks(text):
        for line_number, logical_line in iter_logical_lines(block_start, block_lines):
            if "asc" not in logical_line:
                continue
            for match in re.finditer(r"\basc\b", logical_line):
                if not is_command_prefix(logical_line[: match.start()]):
                    continue
                candidate = clean_command_fragment(logical_line[match.start() :])
                if not candidate or "`" in candidate:
                    continue
                try:
                    tokens = tuple(shlex.split(candidate))
                except ValueError:
                    continue
                if not tokens or tokens[0] != "asc" or should_skip_tokens(tokens):
                    continue
                examples.append(Example(path=path, line_number=line_number, raw=candidate, tokens=tokens))
    return examples


def strip_fenced_blocks(text: str) -> list[str]:
    lines: list[str] = []
    in_block = False
    for line in text.splitlines():
        if line.lstrip().startswith("```"):
            in_block = not in_block
            lines.append("")
            continue
        lines.append("" if in_block else line)
    return lines


def extract_inline_examples(path: Path, text: str) -> list[Example]:
    examples: list[Example] = []
    for line_number, line in enumerate(strip_fenced_blocks(text), start=1):
        if "asc" not in line or "deprecated" in line.lower():
            continue
        for match in re.finditer(r"`([^`]*\basc\b[^`]*)`", line):
            candidate = clean_command_fragment(match.group(1))
            if not candidate:
                continue
            try:
                tokens = tuple(shlex.split(candidate))
            except ValueError:
                continue
            if not tokens or tokens[0] != "asc" or should_skip_tokens(tokens):
                continue
            examples.append(
                Example(
                    path=path,
                    line_number=line_number,
                    raw=candidate,
                    tokens=tokens,
                    source="inline",
                )
            )
    return examples


def extract_examples(website_root: Path) -> list[Example]:
    examples: list[Example] = []
    for path in sorted(website_root.rglob("*.mdx")):
        text = path.read_text()
        examples.extend(extract_fenced_examples(path, text))
        examples.extend(extract_inline_examples(path, text))
    return examples


def usage_tail(spec: CommandSpec) -> str:
    prefix = "asc"
    if spec.path:
        prefix += " " + " ".join(spec.path)
    if spec.usage.startswith(prefix):
        return spec.usage[len(prefix) :].strip()
    return spec.usage.strip()


def usage_position_style(spec: CommandSpec) -> str:
    tail = re.sub(r"--[a-z0-9-]+(?:\s+(?:<[^>]+>|\[[^\]]+\]))?", "", usage_tail(spec))
    if not tail:
        return "no_positionals"
    positional_match = re.search(r"(<[^>]+>|\[[A-Z][^\]]*)", tail)
    if positional_match is None:
        return "no_positionals"
    flag_index = tail.find("[flags]")
    if flag_index == -1:
        return "positionals_only"
    if flag_index < positional_match.start():
        return "flags_before_positionals"
    return "flags_after_positionals"


def deprecation_replacement(help_text: str) -> str | None:
    for pattern in (DEPRECATED_USE_RE, DEPRECATED_ALIAS_RE):
        match = pattern.search(help_text)
        if match:
            return match.group(1)
    return None


def hidden_deprecated_alias_replacement(
    binary_path: Path,
    example: Example,
    root_flags: dict[str, bool],
) -> str | None:
    return deprecation_replacement(path_help(binary_path, token_command_path(example.tokens, root_flags)))


def token_command_path(tokens: tuple[str, ...], root_flags: dict[str, bool]) -> tuple[str, ...]:
    path: list[str] = []
    pending_root_flag: str | None = None
    collecting = False
    for token in tokens[1:]:
        if pending_root_flag is not None:
            pending_root_flag = None
            continue
        if token == "--help" or token.startswith("--"):
            if collecting:
                break
            flag = token.split("=", 1)[0]
            if flag not in root_flags:
                break
            pending_root_flag = flag if "=" not in token and not root_flags[flag] else None
            continue
        collecting = True
        path.append(token)
    return tuple(path)


def hidden_deprecated_alias_spec(
    binary_path: Path,
    example: Example,
    root_flags: dict[str, bool],
) -> CommandSpec | None:
    if hidden_deprecated_alias_replacement(binary_path, example, root_flags) is None:
        return None

    deprecated_path = token_command_path(example.tokens, root_flags)
    if not deprecated_path:
        return None

    deprecated_help = path_help(binary_path, deprecated_path)
    deprecated_spec = parse_help_text(deprecated_help, is_root=False)
    flags = dict(deprecated_spec.flags)
    if not flags:
        flags = dict(HIDDEN_DEPRECATED_ALIAS_FLAGS.get(deprecated_path, {}))

    return CommandSpec(
        path=deprecated_path,
        usage=deprecated_spec.usage,
        flags=flags,
        subcommands=deprecated_spec.subcommands,
    )


def validate_example(
    example: Example,
    index: dict[tuple[str, ...], CommandSpec],
    binary_path: Path | None = None,
) -> list[str]:
    errors: list[str] = []
    root = index[()]
    tokens = example.tokens
    hidden_current: CommandSpec | None = None

    top_level_index: int | None = None
    pending_root_flag: str | None = None
    for i, token in enumerate(tokens[1:], start=1):
        if pending_root_flag is not None:
            if token.startswith("--"):
                errors.append(
                    f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                    f"missing value for global flag {pending_root_flag!r} in {example.raw!r}"
                )
                return errors
            pending_root_flag = None
            continue
        if token == "--help":
            if i == len(tokens) - 1:
                return errors
            continue
        if token in root.subcommands:
            top_level_index = i
            break
        if token.startswith("--"):
            flag = token.split("=", 1)[0]
            if flag not in root.flags:
                errors.append(
                    f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                    f"unknown global flag {flag!r} in {example.raw!r}"
                )
                return errors
            pending_root_flag = flag if "=" not in token and not root.flags[flag] else None
            continue
        if binary_path is not None:
            hidden_current = hidden_deprecated_alias_spec(binary_path, example, root.flags)
            if hidden_current is not None:
                top_level_index = i
                break
        errors.append(
            f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
            f"could not resolve top-level command in {example.raw!r}"
        )
        return errors

    if pending_root_flag is not None:
        errors.append(
            f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
            f"missing value for global flag {pending_root_flag!r} in {example.raw!r}"
        )
        return errors

    if top_level_index is None:
        if tokens_are_root_only_invocation(tokens[1:], root.flags):
            return errors
        errors.append(
            f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
            f"could not resolve top-level command in {example.raw!r}"
        )
        return errors

    if hidden_current is not None:
        current_path = hidden_current.path
        current = hidden_current
        i = top_level_index + len(current_path)
    else:
        current_path = (tokens[top_level_index],)
        current = index.get(current_path)
        assert current is not None
        i = top_level_index + 1

        while i < len(tokens) and current.subcommands and tokens[i] in current.subcommands:
            current_path = (*current_path, tokens[i])
            current = index[current_path]
            i += 1

        if i < len(tokens) and current.subcommands and not tokens[i].startswith("--"):
            hidden_current = None
            if binary_path is not None:
                hidden_current = hidden_deprecated_alias_spec(binary_path, example, root.flags)
            if hidden_current is None:
                errors.append(
                    f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                    f"unknown subcommand {tokens[i]!r} for {' '.join(current.path)!r} in {example.raw!r}"
                )
                return errors
            current_path = hidden_current.path
            current = hidden_current
            i = top_level_index + len(current_path)

    style = usage_position_style(current)
    if current.path == ("workflow", "run"):
        style = "positionals_and_flags"
    pending_flag: str | None = None
    saw_positional = False
    seen_flags: set[str] = set()

    while i < len(tokens):
        token = tokens[i]
        if pending_flag is not None:
            if token.startswith("--"):
                errors.append(
                    f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                    f"missing value for flag {pending_flag!r} in {example.raw!r}"
                )
                return errors
            pending_flag = None
            i += 1
            continue
        if token == "--help":
            i += 1
            continue
        if token.startswith("--"):
            flag = token.split("=", 1)[0]
            if flag in current.flags:
                if saw_positional and style == "flags_before_positionals":
                    errors.append(
                        f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                        f"flag {flag!r} appears after positional arguments in {example.raw!r}"
                    )
                seen_flags.add(flag)
                pending_flag = flag if "=" not in token and not current.flags.get(flag, False) else None
                i += 1
                continue
            if flag in root.flags:
                errors.append(
                    f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                    f"global flag {flag!r} must appear before the top-level command in {example.raw!r}"
                )
                pending_flag = flag if "=" not in token and not root.flags.get(flag, False) else None
            else:
                errors.append(
                    f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                    f"unknown flag {flag!r} for {' '.join(current.path)!r} in {example.raw!r}"
                )
                pending_flag = flag if "=" not in token and not current.flags.get(flag, False) else None
            i += 1
            continue

        saw_positional = True
        if style == "no_positionals":
            errors.append(
                f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
                f"unexpected positional argument {token!r} in {example.raw!r}"
            )
            return errors
        i += 1

    if pending_flag is not None:
        errors.append(
            f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
            f"missing value for flag {pending_flag!r} in {example.raw!r}"
        )
        return errors

    missing_flags = sorted(REQUIRED_FLAGS_BY_COMMAND.get(current.path, set()) - seen_flags)
    if missing_flags:
        errors.append(
            f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
            f"missing required flag(s) {', '.join(missing_flags)!r} for {' '.join(current.path)!r} in {example.raw!r}"
        )

    return errors


def tokens_are_root_only_invocation(
    tokens: tuple[str, ...],
    root_flags: dict[str, bool],
) -> bool:
    pending_value = False
    for token in tokens:
        if pending_value:
            pending_value = False
            continue
        if token == "--help":
            continue
        if not token.startswith("--"):
            return False
        flag = token.split("=", 1)[0]
        if flag not in root_flags:
            return False
        pending_value = "=" not in token and not root_flags[flag]
    return not pending_value


def validate_not_deprecated(
    example: Example,
    binary_path: Path,
    root_flags: dict[str, bool],
) -> list[str]:
    replacement = hidden_deprecated_alias_replacement(binary_path, example, root_flags)
    if replacement is None:
        return []
    return [
        f"{example.path.relative_to(example.path.parents[1])}:{example.line_number}: "
        f"deprecated alias in {example.raw!r}; use {replacement!r}"
    ]


def collect_errors(
    website_root: Path,
    index: dict[tuple[str, ...], CommandSpec],
    binary_path: Path | None = None,
) -> list[str]:
    errors: list[str] = []
    seen: set[tuple[Path, int, tuple[str, ...]]] = set()
    for example in extract_examples(website_root):
        key = (example.path, example.line_number, example.tokens)
        if key in seen:
            continue
        seen.add(key)
        example_errors = validate_example(example, index, binary_path)
        errors.extend(example_errors)
        if example_errors or binary_path is None:
            continue
        errors.extend(validate_not_deprecated(example, binary_path, index[()].flags))
    return errors


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate Mintlify website command examples.")
    parser.add_argument(
        "--website-root",
        default=".",
        help="Path to the Mintlify docs root.",
    )
    args = parser.parse_args(argv)

    repo_root = Path(__file__).resolve().parents[1]
    website_root = (repo_root / args.website_root).resolve()

    with tempfile.TemporaryDirectory() as tmpdir:
        binary_path = Path(tmpdir) / "asc-doc-check"
        subprocess.run(
            ["go", "build", "-o", str(binary_path), "."],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
        )
        index = build_command_index(binary_path)
        errors = collect_errors(website_root, index, binary_path)
        if errors:
            print("Website command validation failed:")
            for error in errors:
                print(f"  - {error}")
            return 1

        print("Website command validation passed.")
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
