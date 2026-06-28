#!/usr/bin/env python3
"""Run deterministic Go test shards for CI.

The script supports two sharding levels:

* package mode: list packages with `go list`, optionally exclude packages, and
  run a stable subset of packages for the requested shard.
* tests mode: list top-level tests in a single package and run a stable subset
  through one `go test -run` regex.

It intentionally keeps selection deterministic from repository contents only so
CI does not depend on checked-in timing data.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys


def run_capture(command: list[str]) -> list[str]:
    result = subprocess.run(command, check=True, text=True, stdout=subprocess.PIPE)
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def go_list(patterns: list[str]) -> list[str]:
    return run_capture(["go", "list", *patterns])


def normalize_packages(patterns: list[str]) -> set[str]:
    if not patterns:
        return set()
    return set(go_list(patterns))


def select_shard(items: list[str], shard_index: int, shard_total: int) -> list[str]:
    return [item for index, item in enumerate(sorted(items)) if index % shard_total == shard_index]


def run_package_shard(args: argparse.Namespace) -> int:
    packages = go_list(args.packages)
    excluded = normalize_packages(args.exclude)
    selected = select_shard(
        [package for package in packages if package not in excluded],
        args.shard_index,
        args.shard_total,
    )
    print(
        f"Selected {len(selected)} package(s) for shard "
        f"{args.shard_index + 1}/{args.shard_total}",
        flush=True,
    )
    for package in selected:
        print(f"  {package}", flush=True)
    if not selected:
        return 0
    return subprocess.run(["go", "test", *args.go_test_args, *selected]).returncode


def run_test_shard(args: argparse.Namespace) -> int:
    tests = run_capture(["go", "test", "-list", "^Test", args.package])
    tests = [test for test in tests if test.startswith("Test")]
    selected = select_shard(tests, args.shard_index, args.shard_total)
    print(
        f"Selected {len(selected)} test(s) from {args.package} for shard "
        f"{args.shard_index + 1}/{args.shard_total}",
        flush=True,
    )
    if not selected:
        return 0
    pattern = "^(?:" + "|".join(re.escape(test) for test in selected) + ")(?:/.*)?$"
    return subprocess.run(
        ["go", "test", *args.go_test_args, "-run", pattern, args.package],
    ).returncode


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="mode", required=True)

    package_parser = subparsers.add_parser("packages")
    package_parser.add_argument("--packages", nargs="+", default=["./..."])
    package_parser.add_argument("--exclude", action="append", default=[])
    package_parser.add_argument("--shard-index", type=int, required=True)
    package_parser.add_argument("--shard-total", type=int, required=True)
    package_parser.add_argument("go_test_args", nargs=argparse.REMAINDER)

    test_parser = subparsers.add_parser("tests")
    test_parser.add_argument("--package", required=True)
    test_parser.add_argument("--shard-index", type=int, required=True)
    test_parser.add_argument("--shard-total", type=int, required=True)
    test_parser.add_argument("go_test_args", nargs=argparse.REMAINDER)

    args = parser.parse_args()
    if args.shard_total < 1:
        parser.error("--shard-total must be at least 1")
    if args.shard_index < 0 or args.shard_index >= args.shard_total:
        parser.error("--shard-index must be between 0 and shard-total - 1")
    if args.go_test_args and args.go_test_args[0] == "--":
        args.go_test_args = args.go_test_args[1:]
    return args


def main() -> int:
    args = parse_args()
    if args.mode == "packages":
        return run_package_shard(args)
    if args.mode == "tests":
        return run_test_shard(args)
    raise AssertionError(f"unknown mode {args.mode}")


if __name__ == "__main__":
    sys.exit(main())
