#!/usr/bin/env python3
"""Run deterministic Go test shards for CI and local runs.

The script supports three modes:

* package mode: list packages with `go list`, optionally exclude packages, and
  run a stable subset of packages for the requested shard.
* tests mode: list top-level tests in a single package and run a stable subset
  through one `go test -run` regex.
* local mode: run every package at once on one host, splitting the slowest
  packages into test shards. Half the CPU budget (--jobs, else GOMAXPROCS,
  else the CPU count) runs shards at GOMAXPROCS=1 each; the remaining packages
  run as one `go test` with GOMAXPROCS and -p set to the other half. Without
  split packages, or with a budget too small to give each one a shard, it runs
  one `go test` with GOMAXPROCS and -p set to the whole budget.

It intentionally keeps selection deterministic from repository contents only so
CI does not depend on checked-in timing data.
"""

from __future__ import annotations

import argparse
import os
import re
import signal
import subprocess
import sys
import tempfile

GO = os.environ.get("GO") or "go"

def run_capture(command: list[str]) -> list[str]:
    result = subprocess.run(command, check=True, text=True, stdout=subprocess.PIPE)
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def go_list(patterns: list[str]) -> list[str]:
    return run_capture([GO, "list", *patterns])


def normalize_packages(patterns: list[str]) -> set[str]:
    if not patterns:
        return set()
    return set(go_list(patterns))


def affected_packages(packages: list[str], files: list[str]) -> list[str] | None:
    """Return the packages whose tests build against the given Go files.

    A non-test file reaches every package that imports its package, tests
    included; a _test.go file reaches only its own package. Returns None when
    a file's package is not among packages (deleted or outside the module),
    so the caller tests everything.
    """
    module = run_capture([GO, "list", "-m"])[0]
    changed, changed_tests = set(), set()
    for path in files:
        directory = os.path.dirname(os.path.normpath(path))
        package = f"{module}/{directory}" if directory else module
        (changed_tests if path.endswith("_test.go") else changed).add(package)
    if not (changed | changed_tests) <= set(packages):
        return None
    affected = changed | changed_tests
    # Test variants print as "pkg [pkg.test]"; Deps is transitive and covers
    # the package's own and external tests.
    for line in run_capture([GO, "list", "-test", "-e", "-f", '{{.ImportPath}}{{range .Deps}}\t{{.}}{{end}}', *packages]):
        name, *deps = line.split("\t")
        if changed.intersection(dep.split(" [")[0] for dep in deps):
            tested = name.split(" [")[1].rstrip("]") if " [" in name else name
            affected.add(tested.removesuffix(".test"))
    return [package for package in packages if package in affected]


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
    return subprocess.run([GO, "test", *args.go_test_args, *selected]).returncode


def run_test_shard(args: argparse.Namespace) -> int:
    tests = list_tests(args.package)
    selected = select_shard(tests, args.shard_index, args.shard_total)
    print(
        f"Selected {len(selected)} test(s) from {args.package} for shard "
        f"{args.shard_index + 1}/{args.shard_total}",
        flush=True,
    )
    if not selected:
        return 0
    return subprocess.run(
        [GO, "test", *args.go_test_args, "-run", run_pattern(selected), args.package],
    ).returncode


def run_pattern(tests: list[str]) -> str:
    return "^(?:" + "|".join(re.escape(test) for test in tests) + ")(?:/.*)?$"


def list_tests(package: str, *flags: str, env: dict[str, str] | None = None) -> list[str]:
    result = subprocess.run([GO, "test", *flags, "-list", "^Test", package], check=True, text=True, stdout=subprocess.PIPE, env=env)
    return [line.strip() for line in result.stdout.splitlines() if line.strip().startswith("Test")]


def run_local(args: argparse.Namespace) -> int:
    signal.signal(signal.SIGTERM, lambda signum, frame: sys.exit(128 + signum))
    gomaxprocs = os.environ.get("GOMAXPROCS") or "0"
    if not gomaxprocs.isdigit():
        print(f"error: GOMAXPROCS must be a positive integer, got {gomaxprocs!r}", file=sys.stderr)
        return 2
    budget = args.jobs or int(gomaxprocs) or os.cpu_count() or 1
    packages = go_list(args.packages)
    if args.affected_by:
        affected = affected_packages(packages, args.affected_by)
        if affected is not None:
            print(f"Testing {len(affected)} of {len(packages)} package(s) reached by the changed Go files", flush=True)
            packages = affected
        if not packages:
            return 0
    split = [package for package in dict.fromkeys(go_list(args.split) if args.split else []) if package in packages]
    rest = [package for package in packages if package not in split]
    # Half the budget runs shards of the split packages, one CPU per shard,
    # handed out in --split order; the other half runs the remaining packages.
    shard_slots = budget // 2
    if not split or shard_slots < len(split):
        split, rest, shard_slots = [], packages, 0

    failed = []
    with tempfile.TemporaryDirectory(prefix="go-test-local-") as directory:
        runs = []

        def start(label: str, command: list[str], cpus: int = 1) -> None:
            log = open(os.path.join(directory, f"{len(runs)}.log"), "w+b")
            environment = {**os.environ, "GOMAXPROCS": str(cpus)}
            process = subprocess.Popen(
                command, stdout=log, stderr=subprocess.STDOUT, env=environment, start_new_session=True
            )
            runs.append((label, log, process))

        try:
            if rest or not split:
                rest_jobs = budget - shard_slots
                start(f"packages, -p={rest_jobs}", [GO, "test", f"-p={rest_jobs}", *args.go_test_args, *rest], rest_jobs)
            # Listing builds each split package's test binary within the
            # shards' share of the budget, so the shards reuse it. A package
            # that fails to build runs whole and reports the failure.
            listed = {}
            for package in split:
                try:
                    listed[package] = list_tests(package, f"-p={shard_slots}", env={**os.environ, "GOMAXPROCS": "1"})
                except subprocess.CalledProcessError:
                    listed[package] = None
            for index, package in enumerate(split):
                if listed[package] is None:
                    start(package, [GO, "test", *args.go_test_args, package])
                    continue
                shards = shard_slots // len(split) + (index < shard_slots % len(split))
                for shard in range(shards):
                    selected = select_shard(listed[package], shard, shards)
                    if selected:
                        start(f"{package} shard {shard + 1}/{shards}",
                              [GO, "test", *args.go_test_args, "-run", run_pattern(selected), package])
            for label, log, process in runs:
                status = process.wait()
                log.seek(0)
                print(f"==> {label} (exit {status})", flush=True)
                sys.stdout.buffer.write(log.read())
                sys.stdout.flush()
                if status != 0:
                    failed.append(label)
        finally:
            # Stop every run, test binaries included, before the caller removes
            # their shared state.
            for _, log, process in runs:
                if process.poll() is None:
                    try:
                        os.killpg(process.pid, signal.SIGTERM)
                    except (ProcessLookupError, PermissionError):
                        pass
                process.wait()
                log.close()
    for label in failed:
        print(f"FAILED: {label}", file=sys.stderr)
    return 1 if failed else 0


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

    local_parser = subparsers.add_parser("local")
    local_parser.add_argument("--packages", nargs="+", default=["./..."])
    local_parser.add_argument("--split", action="append", default=[])
    local_parser.add_argument("--jobs", type=int, default=0)
    local_parser.add_argument("--affected-by", nargs="+", default=[])
    local_parser.add_argument("go_test_args", nargs=argparse.REMAINDER)

    args = parser.parse_args()
    if args.go_test_args and args.go_test_args[0] == "--":
        args.go_test_args = args.go_test_args[1:]
    if args.mode == "local":
        if args.jobs < 0:
            parser.error("--jobs must be a positive integer")
        return args
    if args.shard_total < 1:
        parser.error("--shard-total must be at least 1")
    if args.shard_index < 0 or args.shard_index >= args.shard_total:
        parser.error("--shard-index must be between 0 and shard-total - 1")
    return args


def main() -> int:
    args = parse_args()
    if args.mode == "packages":
        return run_package_shard(args)
    if args.mode == "tests":
        return run_test_shard(args)
    if args.mode == "local":
        try:
            return run_local(args)
        except KeyboardInterrupt:
            return 130
        except subprocess.CalledProcessError as error:
            # go already printed the failure to stderr.
            return error.returncode
    raise AssertionError(f"unknown mode {args.mode}")


if __name__ == "__main__":
    sys.exit(main())
