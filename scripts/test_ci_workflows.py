#!/usr/bin/env python3
"""Protect CI runner, build, artifact, and security-check contracts."""

import os
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
PR_WORKFLOW = ROOT / ".github/workflows/pr-checks.yml"
MAIN_WORKFLOW = ROOT / ".github/workflows/main-branch.yml"
RELEASE_WORKFLOW = ROOT / ".github/workflows/release.yml"
WEBSITE_WORKFLOW = ROOT / ".github/workflows/website-checks.yml"
GOVULNCHECK_WORKFLOW = ROOT / ".github/workflows/govulncheck.yml"
DEPENDABOT_CONFIG = ROOT / ".github/dependabot.yml"
MAKEFILE = ROOT / "Makefile"


def assert_lint_timeout_budget() -> None:
    makefile = MAKEFILE.read_text()
    match = re.search(r"^GOLANGCI_LINT_TIMEOUT \?= (\d+)m$", makefile, re.MULTILINE)
    assert match, "Makefile must define the golangci-lint timeout in minutes"
    assert int(match.group(1)) >= 15, "full-module lint needs at least a 15-minute cold-run budget"


def assert_go_toolchain_source() -> None:
    workflow_dir = ROOT / ".github/workflows"
    workflows = sorted([*workflow_dir.glob("*.yml"), *workflow_dir.glob("*.yaml")])
    assert_go_toolchain_workflows([(path, path.read_text()) for path in workflows])


def assert_go_toolchain_workflows(workflows: list[tuple[Path, str]]) -> None:
    setup_go_count = 0

    for path, workflow in workflows:
        assert "go-version:" not in workflow, f"{path}: source Go versions from go.mod"
        lines = workflow.splitlines()
        for index, line in enumerate(lines):
            uses = re.match(r"^(\s*)(-\s*)?uses:\s*(.*?)\s*$", line)
            if not uses:
                continue
            uses_value = normalize_yaml_scalar(uses.group(3))
            if not uses_value.startswith("actions/setup-go@") or uses_value == "actions/setup-go@":
                continue

            setup_go_count += 1
            uses_indent = len(uses.group(1)) + len(uses.group(2) or "")
            assert setup_go_step_uses_go_mod(lines[index + 1 :], uses_indent), (
                f"{path}: every setup-go step must source go.mod"
            )

    assert setup_go_count > 0, "expected at least one setup-go step"


def normalize_yaml_scalar(value: str) -> str:
    scalar = value.strip()
    quote = ""
    escaped = False
    index = 0

    while index < len(scalar):
        char = scalar[index]
        if quote == '"':
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == quote:
                quote = ""
        elif quote == "'":
            if char == quote:
                if index + 1 < len(scalar) and scalar[index + 1] == quote:
                    index += 1
                else:
                    quote = ""
        elif char in {'"', "'"}:
            quote = char
        elif char == "#" and (index == 0 or scalar[index - 1].isspace()):
            scalar = scalar[:index].rstrip()
            break
        index += 1

    if len(scalar) >= 2 and scalar[0] == scalar[-1] and scalar[0] in {'"', "'"}:
        scalar = scalar[1:-1]
        if value.strip().startswith("'"):
            scalar = scalar.replace("''", "'")
    return scalar


def setup_go_step_uses_go_mod(lines: list[str], uses_indent: int) -> bool:
    for index, line in enumerate(lines):
        if line.strip() and len(line) - len(line.lstrip()) < uses_indent:
            return False

        match = re.match(r"^(\s*)with:\s*$", line)
        if not match or len(match.group(1)) != uses_indent:
            continue

        input_indent = 0
        for setting in lines[index + 1 :]:
            if not setting.strip() or setting.lstrip().startswith("#"):
                continue
            setting_indent = len(setting) - len(setting.lstrip())
            if setting_indent <= uses_indent:
                break
            if input_indent == 0:
                input_indent = setting_indent
            if setting_indent != input_indent:
                continue

            go_version_file = re.match(r"^\s*go-version-file:\s*(.*?)\s*$", setting)
            if go_version_file and normalize_yaml_scalar(go_version_file.group(1)) == "go.mod":
                return True
        return False
    return False


def assert_go_toolchain_source_accepts_normalized_scalars() -> None:
    valid_workflow = """jobs:
  test:
    steps:
      - uses: "actions/setup-go@v6"
        with:
          go-version-file: "go.mod" # quoted source of truth
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod # source of truth
"""
    assert_go_toolchain_workflows([(Path("normalized-scalars.yml"), valid_workflow)])


def assert_go_toolchain_source_rejects_step_local_violations() -> None:
    invalid_workflows = {
        "missing-before-valid.yml": """jobs:
  test:
    steps:
      - uses: "actions/setup-go@v6"
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
""",
        "comment-only.yml": """jobs:
  test:
    steps:
      - uses: actions/setup-go@v6
        with:
          go-version-file: # go.mod
""",
    }

    for filename, workflow in invalid_workflows.items():
        path = Path(filename)
        expected = f"{path}: every setup-go step must source go.mod"
        try:
            assert_go_toolchain_workflows([(path, workflow)])
        except AssertionError as error:
            if str(error) != expected:
                raise
        else:
            raise AssertionError(f"{filename} must fail: {expected}")


def assert_govulncheck_version_source() -> None:
    makefile = MAKEFILE.read_text()
    workflow = GOVULNCHECK_WORKFLOW.read_text()

    assert re.search(r"^GOVULNCHECK_VERSION \?= v\d+\.\d+\.\d+$", makefile, re.MULTILINE)
    assert "golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)" in makefile
    assert "run: make install-govulncheck" in workflow
    assert "golang.org/x/vuln/cmd/govulncheck@" not in workflow
    assert "govulncheck@latest" not in makefile


def job_block(workflow: str, job: str) -> str:
    marker = f"  {job}:\n"
    start = workflow.find(marker)
    if start < 0:
        raise AssertionError(f"missing job {job!r}")

    end = len(workflow)
    offset = start + len(marker)
    for line in workflow[offset:].splitlines(keepends=True):
        content = line.rstrip("\r\n")
        if content.startswith("  ") and not content.startswith("    ") and content.endswith(":"):
            end = offset
            break
        offset += len(line)
    return workflow[start:end]


def matrix_command_for_runner(job: str, runner: str) -> str:
    marker = f"runner: {runner}"
    start = job.find(marker)
    if start < 0:
        return ""
    rest = job[start:]
    command_at = rest.find("command: |")
    next_runner = rest.find("\n            runner:", len(marker))
    if command_at < 0 or (0 <= next_runner < command_at):
        return ""
    command = rest[command_at:]
    bounds = [
        index
        for index in (
            command.find("\n          - name:"),
            command.find("\n    steps:"),
        )
        if index >= 0
    ]
    if bounds:
        command = command[: min(bounds)]
    return command


def assert_optimized_workflow(path: Path, test_job: str) -> None:
    assert_optimized_workflow_text(path, path.read_text(), test_job)


def assert_optimized_workflow_text(path: Path, workflow: str, test_job: str) -> None:
    assert "actions/upload-artifact" not in workflow, f"{path}: development artifacts must not be uploaded"
    changes = job_block(workflow, "changes")
    assert "scope: ${{ steps.scope.outputs.scope }}" in changes
    assert "website_affected: ${{ steps.scope.outputs.website_affected }}" in changes
    assert "python3 scripts/ci_change_scope.py --github-output" in changes
    assert "git diff --name-only --no-renames" in changes
    for guarded_path in (
        ".github/workflows/*",
        "scripts/ci_change_scope.py",
        "scripts/test_ci_change_scope.py",
        "scripts/test_ci_workflows.py",
    ):
        assert guarded_path in changes
    assert changes.index("force_full=false") < changes.index(
        "python3 scripts/ci_change_scope.py --github-output"
    )
    assert 'if [ "$force_full" = true ]; then' in changes
    assert "website_affected=true" in changes
    assert "wall|docs|website|telemetry|full" in changes
    assert "invalid CI scope" in changes

    wall_only = job_block(workflow, "wall-only-check")
    assert "runs-on: ubuntu-latest" in wall_only
    assert "make check-wall-of-apps" in wall_only, f"{path}: wall-only-check must validate the Wall source"
    assert "runs-on: ubuntu-latest" in job_block(workflow, "format-and-lint")
    quality = job_block(workflow, "quality-checks")
    assert "runs-on: ubuntu-latest" in quality
    assert "python3 scripts/test_ci_change_scope.py" in quality
    # Agents.md: formatting, documentation, and lint must keep running on PR and main.
    for command in (
        "make format-check",
        "python3 scripts/test_check_docs.py",
        "make check-docs",
        "make check-wall-of-apps",
    ):
        assert command in quality, f"{path}: quality-checks must run {command!r}"
    lint = job_block(workflow, "lint")
    assert "make lint" in lint, f"{path}: lint must run 'make lint'"
    # Every test job runs on ubuntu, so darwin- and windows-gated sources and tests
    # are never type-checked unless CI vets them explicitly. golangci-lint covers
    # GOOS=linux with tests enabled, so only the two absent platforms need a pass.
    vet = job_block(workflow, "vet-platforms")
    assert "goos: [darwin, windows]" in vet, f"{path}: vet-platforms must type-check darwin and windows"
    assert "run: GOOS=${{ matrix.goos }} go vet ./..." in vet
    for job in (lint, vet):
        assert "runs-on: ubuntu-latest" in job
        assert "contains(fromJSON('[\"telemetry\", \"full\"]'), needs.changes.outputs.scope)" in job
    gate = job_block(workflow, "format-and-lint")
    assert "needs: [changes, wall-only-check, quality-checks, lint, vet-platforms, website-checks]" in gate
    for result in ("needs.quality-checks.result", "needs.lint.result", "needs.vet-platforms.result"):
        assert result in gate, f"{path}: format-and-lint must require {result}"
    website = job_block(workflow, "website-checks")
    assert "uses: ./.github/workflows/website-checks.yml" in website
    assert "needs.changes.outputs.website_affected == 'true'" in website
    tests = job_block(workflow, test_job)
    assert "runs-on: ubuntu-latest" in tests
    assert "needs.changes.outputs.scope == 'full'" in tests
    # Agents.md: the Go test suite must keep running, over the whole module, with the
    # keychain bypass that stops runners from prompting or leaking host profile state.
    assert "python3 scripts/go_test_shard.py" in tests, f"{path}: {test_job} must run the sharded Go test suite"
    assert "--packages ./..." in tests, f"{path}: {test_job} must cover every package"
    assert "go test" in tests, f"{path}: {test_job} must invoke go test"
    assert 'ASC_BYPASS_KEYCHAIN: "1"' in tests, f"{path}: {test_job} must bypass the keychain for all commands"

    build_platforms = job_block(workflow, "build-platforms")
    assert "needs.changes.outputs.scope == 'full'" in build_platforms
    for runner in ("macos-latest", "ubuntu-latest", "windows-latest"):
        assert f"runner: {runner}" in build_platforms, f"{path}: missing native build runner {runner}"
    assert "go test -short ./internal/screenshots" in build_platforms, f"{path}: missing Darwin-only tests"
    assert "ASC_BYPASS_KEYCHAIN=1 go test -count=1 ./internal/screenshots" in build_platforms, (
        f"{path}: missing Windows screenshot runtime tests"
    )
    for runner in ("macos-latest", "windows-latest"):
        command = matrix_command_for_runner(build_platforms, runner)
        assert "ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/rootfs" in command, (
            f"{path}: {runner} must run ./internal/rootfs with the keychain bypass"
        )
    windows = matrix_command_for_runner(build_platforms, "windows-latest")
    macos = matrix_command_for_runner(build_platforms, "macos-latest")
    for runner, command, tests in (
        ("macos-latest", macos, (
            "ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/secureopen -run 'NoInherit'",
            "ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/cli/certificates -run 'ACL|OwnerOnlyAtCreation'",
        )),
        ("windows-latest", windows, (
            "ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/secureopen\n",
            "ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/cli/certificates -run '^TestCreateCertificateExportStagingFileProtectsDACLAtCreation$'",
        )),
    ):
        for test in tests:
            assert test in command, f"{path}: {runner} must run native file-permission test {test.strip()!r}"
    assert "ASC_BYPASS_KEYCHAIN=1 ASC_WINDOWS_CREDENTIAL_SMOKE=1 ASC_WINDOWS_SMOKE_BINARY=../../../build/asc_dev_windows_amd64.exe go test -count=1 ./internal/cli/auth -run '^TestWindowsCredentialManagerNativeRoundTrip$'" in windows, (
        f"{path}: missing opt-in native Windows Credential Manager smoke"
    )
    for variable in ("CGO_CFLAGS", "CGO_LDFLAGS"):
        assert f'export {variable}="-O2 -g -mmacosx-version-min=13.0"' in macos, (
            f"{path}: Darwin minimum must participate in the CGO build cache key"
        )
    for arch in ("amd64", "arm64"):
        command = f"MACOSX_DEPLOYMENT_TARGET=13.0 CGO_ENABLED=1 GOOS=darwin GOARCH={arch} go build"
        assert command in build_platforms, f"{path}: missing cgo-enabled Darwin {arch} build"
        assert f"asc_dev_macos_{arch}" in build_platforms
    for os_name, arch in (("linux", "amd64"), ("linux", "arm64"), ("windows", "amd64")):
        command = f"CGO_ENABLED=0 GOOS={os_name} GOARCH={arch} go build"
        assert command in build_platforms, f"{path}: missing cgo-disabled {os_name} {arch} build"

    # Native backend failures must block the existing platform build gate.
    for name, runner, install in (
        ("security-key macOS", "macos-latest", "brew install libfido2 pkg-config"),
    ):
        entry = re.search(
            rf"          - name: {re.escape(name)}\n(.*?)(?=\n          - name:|\n    steps:)",
            build_platforms,
            re.DOTALL,
        )
        assert entry is not None, f"{path}: missing native {name} build"
        command = entry.group(1)
        for required in (
            f"runner: {runner}",
            install,
            "pkg-config --atleast-version=1.14.0 libfido2",
            "export CGO_ENABLED=1",
            "go build -tags fido2",
            "ASC_BYPASS_KEYCHAIN=1 go test -race -tags fido2 -p=1 -parallel=4 -count=1",
            "./internal/securitykey ./internal/web ./internal/cli/web ./internal/cli/cmdtest",
        ):
            assert required in command, f"{path}: {name} must run {required!r}"

    ordinary_build = job_block(workflow, "ordinary-build")
    assert "needs.changes.outputs.scope == 'telemetry'" in ordinary_build
    assert "runner: ubuntu-latest" in ordinary_build
    assert "runner: macos-latest" in ordinary_build
    assert "run: go build ." in ordinary_build
    assert "go build ./..." not in ordinary_build

    build = job_block(workflow, "build")
    assert "needs: [changes, build-platforms, ordinary-build]" in build
    assert "if: always()" in build
    assert "needs.build-platforms.result" in build
    assert "needs.ordinary-build.result" in build


def assert_optimized_workflow_rejects_weakened_checks() -> None:
    """Prove the guard fails when a required check is deleted from PR or main CI."""
    for path, test_job in ((PR_WORKFLOW, "unit-test-shards"), (MAIN_WORKFLOW, "test-shards")):
        workflow = path.read_text()
        for command in (
            "make format-check",
            "python3 scripts/test_check_docs.py",
            "make check-docs",
            "make check-wall-of-apps",
            "make lint",
            "goos: [darwin, windows]",
            "GOOS=${{ matrix.goos }} go vet ./...",
            "needs.lint.result",
            "needs.vet-platforms.result",
            "./internal/secureopen -run 'NoInherit'",
            "./internal/cli/certificates -run 'ACL|OwnerOnlyAtCreation'",
            "./internal/cli/certificates -run '^TestCreateCertificateExportStagingFileProtectsDACLAtCreation$'",
            "python3 scripts/go_test_shard.py",
            "--packages ./...",
            "ASC_BYPASS_KEYCHAIN=1",
            'ASC_BYPASS_KEYCHAIN: "1"',
            'export CGO_CFLAGS="-O2 -g -mmacosx-version-min=13.0"',
            'export CGO_LDFLAGS="-O2 -g -mmacosx-version-min=13.0"',
            "MACOSX_DEPLOYMENT_TARGET=13.0 CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build",
            "MACOSX_DEPLOYMENT_TARGET=13.0 CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build",
            "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build",
            "CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build",
            "CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build",
            "brew install libfido2 pkg-config",
            "pkg-config --atleast-version=1.14.0 libfido2",
            "export CGO_ENABLED=1",
            "go build -tags fido2",
            "ASC_BYPASS_KEYCHAIN=1 go test -race -tags fido2 -p=1 -parallel=4 -count=1",
            "./internal/securitykey ./internal/web ./internal/cli/web ./internal/cli/cmdtest",
            "ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/rootfs",
        ):
            assert command in workflow, f"{path}: expected to find {command!r}"
            weakened = workflow.replace(command, "true")
            try:
                assert_optimized_workflow_text(path, weakened, test_job)
            except AssertionError:
                continue
            raise AssertionError(f"{path}: guard accepts CI with {command!r} removed")

        rootfs_command = "              ASC_BYPASS_KEYCHAIN=1 go test -short -count=1 ./internal/rootfs\n"
        windows_only_removed = workflow
        last = workflow.rfind(rootfs_command)
        if last >= 0:
            windows_only_removed = workflow[:last] + workflow[last + len(rootfs_command) :]
        for runner, weakened in (
            ("macOS", workflow.replace(rootfs_command, "", 1)),
            ("Windows", windows_only_removed),
        ):
            try:
                assert_optimized_workflow_text(path, weakened, test_job)
            except AssertionError:
                continue
            raise AssertionError(
                f"{path}: guard accepts CI with ./internal/rootfs removed from {runner} only"
            )


def step_script(job: str, step_name: str) -> str:
    """Return the dedented `run: |` script of the named step inside a job block."""
    marker = f"- name: {step_name}\n"
    start = job.find(marker)
    if start < 0:
        raise AssertionError(f"missing step {step_name!r}")
    rest = job[start + len(marker) :]
    run_at = rest.find("run: |\n")
    if run_at < 0:
        raise AssertionError(f"step {step_name!r} must use a `run: |` block")
    body = rest[run_at + len("run: |\n") :]
    script_indent = None
    lines: list[str] = []
    for line in body.splitlines():
        if line.strip():
            indent = len(line) - len(line.lstrip())
            if script_indent is None:
                script_indent = indent
            if indent < script_indent:
                break
        lines.append(line)
    return textwrap.dedent("\n".join(lines)) + "\n"


def shell_functions(script: str) -> dict[str, str]:
    """Map every top-level `name() {` ... `}` definition to its full source."""
    functions: dict[str, str] = {}
    current = None
    body: list[str] = []
    for line in script.splitlines():
        if current is None:
            match = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)\(\)\s*\{\s*$", line)
            if match:
                current = match.group(1)
                body = [line]
            continue
        body.append(line)
        if line == "}":
            functions[current] = "\n".join(body) + "\n"
            current = None
    if current is not None:
        raise AssertionError(f"unterminated shell function {current!r}")
    return functions


WINGET_RETRY_HELPER = "retry_transient"
# The clone uses --filter=blob:none, so checkout and sparse-checkout lazily fetch
# blobs over the network and must be retried like any other transfer.
WINGET_NETWORK_CALL = re.compile(
    r"\b(gh api user|gh api \"?repos/|gh repo view|gh repo fork|gh pr list|gh pr create"
    r"|git clone|git fetch|git push|git checkout|git sparse-checkout)"
)
# `gh pr list --head` only accepts a bare branch name, so an owner:branch lookup
# silently matches nothing; the cross-fork check must use the REST head filter.
WINGET_HEAD_LOOKUP = 'repos/microsoft/winget-pkgs/pulls?state=open&head=${WINGET_FORK_OWNER}:${BRANCH}'


def assert_winget_submission_retries_transient_failures_text(path: Path, workflow: str) -> None:
    script = step_script(job_block(workflow, "winget"), "Submit WinGet package PR")
    functions = shell_functions(script)
    for helper in (WINGET_RETRY_HELPER, "is_transient_failure"):
        assert helper in functions, f"{path}: WinGet submission must define the {helper}() helper"
    for knob in ("WINGET_RETRY_ATTEMPTS", "WINGET_RETRY_BASE_DELAY", "WINGET_RETRY_MAX_DELAY"):
        assert knob in functions[WINGET_RETRY_HELPER], f"{path}: {WINGET_RETRY_HELPER} must honor {knob}"
    assert "RANDOM" in functions[WINGET_RETRY_HELPER], f"{path}: {WINGET_RETRY_HELPER} must add jitter"

    # Compare invocation sites, not definitions: the preflight lives in a
    # function, so its body position says nothing about when it runs.
    preflight_functions = {name for name, body in functions.items() if "gh api rate_limit" in body}
    assert preflight_functions, f"{path}: WinGet submission must log the GitHub rate limit before submitting"
    top_level = []
    current_function = None
    for line in script.splitlines():
        header = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)\(\)\s*\{\s*$", line)
        if header:
            current_function = header.group(1)
            continue
        if line == "}":
            current_function = None
            continue
        top_level.append((current_function, line))
    preflight_call = next(
        (i for i, (fn, line) in enumerate(top_level) if fn is None and re.match(r"^\s*(\w+)\s*$", line) and line.strip() in preflight_functions),
        None,
    )
    first_call = next((i for i, (fn, line) in enumerate(top_level) if fn is None and "gh api user" in line), None)
    assert preflight_call is not None, f"{path}: the rate_limit preflight function must be invoked at the top level"
    assert first_call is not None, f"{path}: WinGet submission must authenticate with gh api user"
    assert preflight_call < first_call, f"{path}: the rate_limit preflight must run before the first API call"

    # Any function whose body performs a network call must be invoked only
    # through the retry helper, at every call site.
    network_functions = {
        name
        for name, body in functions.items()
        if name != WINGET_RETRY_HELPER
        and any(WINGET_NETWORK_CALL.search(l) and not l.lstrip().startswith("#") for l in body.splitlines())
    }
    retried_functions: set[str] = set()
    for fn, line in top_level:
        if line.lstrip().startswith("#"):
            continue
        for name in network_functions:
            for match in re.finditer(rf"(?<![A-Za-z0-9_]){re.escape(name)}(?![A-Za-z0-9_(])", line):
                prefix = line[: match.start()].rstrip()
                assert prefix.endswith(WINGET_RETRY_HELPER), (
                    f"{path}: {name}() performs network calls and must be invoked via {WINGET_RETRY_HELPER}: {line.strip()}"
                )
                retried_functions.add(name)
        call = WINGET_NETWORK_CALL.search(line)
        if not call:
            continue
        wrapped = WINGET_RETRY_HELPER in line or fn in network_functions
        assert wrapped, f"{path}: unguarded WinGet network call {call.group(1)!r}: {line.strip()}"
    assert network_functions <= retried_functions, (
        f"{path}: network-bearing functions never invoked via {WINGET_RETRY_HELPER}: {sorted(network_functions - retried_functions)}"
    )

    # verify_winget_scope must stay a hard refusal that no retry loop can paper over.
    assert "verify_winget_scope" in functions, f"{path}: WinGet submission must keep verify_winget_scope"
    assert "verify_winget_scope" not in retried_functions, f"{path}: verify_winget_scope must not be retried"
    for want in (
        "WinGet manifests for ${VERSION} are already published or up to date.",
        "WinGet PR already exists for Rorkai.ASC ${VERSION}.",
        "WinGet PR already exists for ${VERSION}.",
    ):
        assert want in script, f"{path}: WinGet submission must keep the idempotency branch {want!r}"
    # A retried PR creation must re-check for an existing PR so a lost response
    # cannot produce a duplicate microsoft/winget-pkgs PR.
    create_functions = [name for name, body in functions.items() if "gh pr create" in body]
    assert create_functions, f"{path}: gh pr create must live inside a retried function"
    for name in create_functions:
        assert name in retried_functions, f"{path}: {name} must be invoked through {WINGET_RETRY_HELPER}"
        body = functions[name]
        create_at = body.find("gh pr create")
        for lookup in ("gh pr list", WINGET_HEAD_LOOKUP):
            lookup_at = body.find(lookup)
            assert lookup_at >= 0, f"{path}: {name} must check for an existing PR ({lookup!r}) before creating one"
            assert lookup_at < create_at, f"{path}: {name} must run {lookup!r} before gh pr create"
    for line in script.splitlines():
        assert not ("gh pr list" in line and "--head" in line and ":" in line.split("--head", 1)[1]), (
            f"{path}: gh pr list --head does not support owner:branch: {line.strip()}"
        )
    # The best-effort PR count must stay numeric even when every attempt fails.
    assert "|| OPEN_PACKAGE_PRS=\"\"" in script, f"{path}: OPEN_PACKAGE_PRS must fall back explicitly on failure"
    assert "''|*[!0-9]*) OPEN_PACKAGE_PRS=\"0\" ;;" in script, f"{path}: OPEN_PACKAGE_PRS must be normalized to a number"
    for knob, default in (
        ("WINGET_RETRY_ATTEMPTS", "5"),
        ("WINGET_RETRY_BASE_DELAY", "10"),
        ("WINGET_RETRY_MAX_DELAY", "120"),
    ):
        assert f"${{{knob}:-{default}}}" in functions[WINGET_RETRY_HELPER], (
            f"{path}: {WINGET_RETRY_HELPER} must default {knob} so a missing env cannot unbound the loop"
        )
    clone_functions = [name for name, body in functions.items() if "git clone" in body]
    assert clone_functions, f"{path}: git clone must live inside a retried function that clears partial clones"
    for name in clone_functions:
        assert name in retried_functions, f"{path}: {name} must be invoked through {WINGET_RETRY_HELPER}"
        assert "rm -rf winget-pkgs" in functions[name], f"{path}: {name} must remove a partial clone before retrying"


def assert_winget_submission_retries_transient_failures() -> None:
    assert_winget_submission_retries_transient_failures_text(RELEASE_WORKFLOW, RELEASE_WORKFLOW.read_text())


def assert_winget_retry_guard_rejects_unguarded_calls() -> None:
    workflow = RELEASE_WORKFLOW.read_text()
    for wrapped in (
        f"{WINGET_RETRY_HELPER} gh api user",
        f"{WINGET_RETRY_HELPER} gh repo view",
        f"{WINGET_RETRY_HELPER} gh pr list",
        f'{WINGET_RETRY_HELPER} git push origin "${{BRANCH}}"',
    ):
        assert wrapped in workflow, f"{RELEASE_WORKFLOW}: expected to find {wrapped!r}"
        weakened = workflow.replace(wrapped, wrapped[len(WINGET_RETRY_HELPER) + 1 :], 1)
        try:
            assert_winget_submission_retries_transient_failures_text(RELEASE_WORKFLOW, weakened)
        except AssertionError:
            continue
        raise AssertionError(f"{RELEASE_WORKFLOW}: guard accepts an unguarded {wrapped!r}")

    auth_call = f"AUTH_LOGIN=$({WINGET_RETRY_HELPER} gh api user --jq .login)"
    assert auth_call in workflow, f"{RELEASE_WORKFLOW}: expected to find {auth_call!r}"
    for label, weakened in (
        ("a missing rate_limit preflight", workflow.replace("gh api rate_limit", "gh api user", 1)),
        (
            "a preflight invoked after the first API call",
            workflow.replace("\n          log_rate_limit\n", "\n", 1).replace(
                auth_call, f"{auth_call}\n          log_rate_limit", 1
            ),
        ),
        (
            "a bare call to a network-bearing function",
            workflow.replace(
                f"{WINGET_RETRY_HELPER} clone_winget_fork",
                f"{WINGET_RETRY_HELPER} clone_winget_fork\n          clone_winget_fork",
                1,
            ),
        ),
        (
            "PR creation ahead of the existing-PR lookups",
            workflow.replace(
                "          submit_winget_pr() {\n            local existing_pr\n",
                "          submit_winget_pr() {\n            local existing_pr\n"
                '            gh pr create --repo microsoft/winget-pkgs --head "${WINGET_FORK_OWNER}:${BRANCH}" --title t --body b || true\n',
                1,
            ),
        ),
    ):
        assert weakened != workflow, f"{RELEASE_WORKFLOW}: could not construct weakening {label!r}"
        try:
            assert_winget_submission_retries_transient_failures_text(RELEASE_WORKFLOW, weakened)
        except AssertionError:
            continue
        raise AssertionError(f"{RELEASE_WORKFLOW}: guard accepts {label}")


FAKE_GH = """#!/bin/sh
count=$(cat "$ATTEMPT_FILE" 2>/dev/null || echo 0)
count=$((count + 1))
echo "$count" > "$ATTEMPT_FILE"
if [ "$count" -lt "$SUCCEED_ON" ]; then
  echo "partial stdout from attempt $count"
  printf '%s\\n' "$FAILURE_MESSAGE" >&2
  exit 1
fi
echo "ok $*"
"""


def run_winget_retry_helper(
    helpers: str, failure_message: str, succeed_on: int, attempts: int | None = 3
) -> tuple[subprocess.CompletedProcess[str], int]:
    with tempfile.TemporaryDirectory() as tmpdir:
        fake_gh = Path(tmpdir) / "gh"
        fake_gh.write_text(FAKE_GH)
        fake_gh.chmod(0o755)
        attempt_file = Path(tmpdir) / "attempts"
        env = os.environ.copy()
        env.update(
            {
                "PATH": f"{tmpdir}{os.pathsep}{env['PATH']}",
                "ATTEMPT_FILE": str(attempt_file),
                "SUCCEED_ON": str(succeed_on),
                "FAILURE_MESSAGE": failure_message,
                "WINGET_RETRY_BASE_DELAY": "0",
                "WINGET_RETRY_MAX_DELAY": "0",
            }
        )
        env.pop("WINGET_RETRY_ATTEMPTS", None)
        if attempts is not None:
            env["WINGET_RETRY_ATTEMPTS"] = str(attempts)
        # GitHub runs `run:` steps with `bash -e`; mirror that so a failure inside
        # the helper surfaces the same way it would in the release job.
        result = subprocess.run(
            ["bash", "-e", "-c", helpers + "\nretry_transient gh api user --jq .login\n"],
            env=env,
            capture_output=True,
            text=True,
            check=False,
        )
        made = int(attempt_file.read_text().strip()) if attempt_file.exists() else 0
        return result, made


def assert_winget_retry_helper_behavior() -> None:
    """Exercise the real bash helper: back off on throttles, fail fast on everything else."""
    script = step_script(job_block(RELEASE_WORKFLOW.read_text(), "winget"), "Submit WinGet package PR")
    functions = shell_functions(script)
    helpers = functions["is_transient_failure"] + functions[WINGET_RETRY_HELPER]

    transient = (
        "HTTP 403: API rate limit exceeded for user ID 1 "
        "(https://docs.github.com/rest/overview/rate-limits-for-the-rest-api)",
        "HTTP 403: You have exceeded a secondary rate limit. Please wait a few minutes before you try again.",
        "HTTP 429: Too Many Requests",
        "HTTP 408: Request Timeout",
        "HTTP 502: Server Error",
        "GraphQL: API rate limit already exceeded for user ID 1",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': "
        "The requested URL returned error: 429",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': "
        "The requested URL returned error: 503",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': "
        "The requested URL returned error: 408",
        "error: RPC failed; curl 56 Recv failure: Connection reset by peer",
        "fatal: early EOF",
        "Get \"https://api.github.com/user\": EOF",
        "Post \"https://api.github.com/graphql\": write tcp 10.0.0.2:51234->140.82.112.6:443: write: broken pipe",
        "fatal: the remote end hung up unexpectedly",
        "Post \"https://api.github.com/graphql\": dial tcp: i/o timeout",
        "fatal: unable to access 'https://github.com/': Could not resolve host: github.com",
        "Get \"https://api.github.com/user\": dial tcp: lookup api.github.com on 127.0.0.53:53: server misbehaving",
        "Get \"https://api.github.com/user\": dial tcp: lookup api.github.com: no such host",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': "
        "LibreSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': "
        "OpenSSL SSL_connect: Connection reset by peer in connection to github.com:443",
        "Get \"https://api.github.com/user\": dial tcp 140.82.112.6:443: connect: network is unreachable",
        "Get \"https://api.github.com/user\": dial tcp 140.82.112.6:443: connect: no route to host",
        "Post \"https://api.github.com/graphql\": read tcp 10.0.0.2:51234->140.82.112.6:443: read: connection reset by peer",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': "
        "Failure when receiving data from the peer",
        "error: RPC failed; curl 18 transfer closed with outstanding read data remaining",
        "fatal: unable to access 'https://github.com/example/winget-pkgs.git/': Could not resolve proxy: proxy.example",
    )
    for message in transient:
        result, made = run_winget_retry_helper(helpers, message, succeed_on=3)
        assert result.returncode == 0, f"{message!r}: helper must recover after a transient failure\n{result.stderr}"
        assert made == 3, f"{message!r}: expected 3 attempts, made {made}"
        assert result.stdout == "ok api user --jq .login\n", (
            f"{message!r}: failed attempts must not leak stdout: {result.stdout!r}"
        )
        assert "retrying in" in result.stderr, f"{message!r}: helper must announce the retry\n{result.stderr}"
        assert message in result.stderr, f"{message!r}: helper must surface the underlying error"

    fail_fast = (
        "HTTP 401: Bad credentials (https://api.github.com/user)",
        "HTTP 403: Resource not accessible by personal access token",
        "HTTP 404: Not Found",
        "HTTP 422: Validation Failed (https://api.github.com/repos/microsoft/winget-pkgs/pulls)",
        "GraphQL: Could not resolve to a Repository with the name 'example/winget-pkgs'. (repository)",
        "fatal: could not read Username for 'https://github.com': terminal prompts disabled",
        "::error::WinGet branch contains changes outside the 1.2.3 manifest directory",
    )
    for message in fail_fast:
        result, made = run_winget_retry_helper(helpers, message, succeed_on=99)
        assert result.returncode == 1, f"{message!r}: helper must propagate a genuine failure"
        assert made == 1, f"{message!r}: genuine failures must not be retried, made {made} attempts"
        assert "retrying in" not in result.stderr, f"{message!r}: helper must not retry\n{result.stderr}"
        assert "not retrying" in result.stderr, f"{message!r}: helper must explain why it stopped\n{result.stderr}"
        assert message in result.stderr, f"{message!r}: helper must surface the underlying error"

    result, made = run_winget_retry_helper(helpers, "HTTP 429: Too Many Requests", succeed_on=99, attempts=4)
    assert result.returncode == 1, "helper must fail once the attempt budget is spent"
    assert made == 4, f"helper must stop at WINGET_RETRY_ATTEMPTS, made {made} attempts"
    assert "giving up" in result.stderr, result.stderr

    result, made = run_winget_retry_helper(helpers, "HTTP 429: Too Many Requests", succeed_on=99, attempts=None)
    assert result.returncode == 1, "helper must fail once the default attempt budget is spent"
    assert made == 5, f"helper must default WINGET_RETRY_ATTEMPTS to 5 when unset, made {made} attempts"


def run_security_target(path: str) -> subprocess.CompletedProcess[str]:
    make = shutil.which("make")
    if make is None:
        raise AssertionError("make is required to test Makefile contracts")

    env = os.environ.copy()
    env["PATH"] = path
    return subprocess.run(
        [
            make,
            "--no-print-directory",
            "-f",
            str(MAKEFILE),
            "VERSION=test",
            "COMMIT=test",
            "DATE=test",
            "GOBIN=/tmp/asc-test-bin",
            "GO_TOOLCHAIN_VERSION=test",
            "security",
        ],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )


def assert_security_target_contract() -> None:
    with tempfile.TemporaryDirectory() as tmpdir:
        fake_go = Path(tmpdir) / "go"
        fake_go.write_text("#!/bin/sh\nexit 0\n")
        fake_go.chmod(0o755)

        missing = run_security_target(tmpdir)
        assert missing.returncode == 0, missing.stderr
        assert "Install gosec for security checks" in missing.stdout

        fake_gosec = Path(tmpdir) / "gosec"
        fake_gosec.write_text("#!/bin/sh\necho scanner-result >&2\nexit 23\n")
        fake_gosec.chmod(0o755)

        finding = run_security_target(tmpdir)
        assert finding.returncode != 0, (
            "make security must fail when gosec fails; "
            f"stdout:\n{finding.stdout}\nstderr:\n{finding.stderr}"
        )
        assert "scanner-result" in finding.stderr
        assert "Install gosec for security checks" not in finding.stdout

        fake_gosec.write_text("#!/bin/sh\nexit 0\n")
        success = run_security_target(tmpdir)
        assert success.returncode == 0, success.stderr
        assert "Install gosec for security checks" not in success.stdout


def assert_test_result_cache_disabled() -> None:
    makefile = MAKEFILE.read_text()
    for target in ("test", "test-short"):
        recipe = re.search(rf"^{target}:\n(.*?)(?=^\S|\Z)", makefile, re.MULTILINE | re.DOTALL)
        assert recipe is not None
        assert "-count=1" in recipe.group(1), f"{target} must run tests without cached results"
        assert "run_isolated_tests" in recipe.group(1), f"{target} must retain isolated test environment"


def assert_go_tool_cache_identity() -> None:
    for path in (PR_WORKFLOW, MAIN_WORKFLOW):
        workflow = path.read_text()
        keys = [line for line in workflow.splitlines() if "key:" in line and "go-tools" in line]
        assert keys, f"{path}: missing tool cache"
        for key in keys:
            assert "runner.arch" in key, f"{path}: tool cache must include architecture"
            assert "steps.go-tool-versions.outputs.key" in key, f"{path}: tool cache must key on tool and Go versions"
            assert "hashFiles('Makefile'" not in key, f"{path}: unrelated Makefile edits must not rebuild tools"
        assert "GOFUMPT|GOLANGCI_LINT" in workflow and "go env GOVERSION" in workflow


def assert_lint_analysis_cache() -> None:
    prefix = "golangci-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.mod', 'go.sum', 'Makefile', '.golangci.yml') }}-"
    for path in (PR_WORKFLOW, MAIN_WORKFLOW):
        quality = job_block(path.read_text(), "lint")
        cache = re.search(r"- name: Cache lint analysis\n(.*?)(?=\n      - )", quality, re.DOTALL)
        assert cache is not None, f"{path}: persist the actual lint analysis cache"
        block = cache.group(1)
        assert "uses: actions/cache@v6" in block
        assert "path: ${{ github.workspace }}/.golangci-cache" in block
        assert "key: " + prefix + "${{ github.sha }}" in block
        assert "restore-keys: |\n            " + prefix in block
        lint = re.search(r"- name: Run lint\n(.*?)(?=\n          - name:|\Z)", quality, re.DOTALL)
        assert lint is not None and "make lint" in lint.group(1)
        assert "cache-hit" not in lint.group(1), f"{path}: restored analysis must not skip lint"


def assert_workload_cache_action(action: str) -> None:
    assert "go-version-file: go.mod" in action
    assert "cache: false" in action, "setup-go must not race to save its shared cache"
    assert "go env GOCACHE" in action and "go env GOMODCACHE" in action
    prefix = "go-v1-${{ runner.os }}-${{ runner.arch }}-${{ steps.go.outputs.go-version }}-${{ inputs.workload }}-${{ hashFiles('go.mod', 'go.sum') }}-"
    assert action.count("key: " + prefix + "${{ github.sha }}") == 2, "save newly compiled source entries"
    assert action.count("restore-keys: |\n          " + prefix) == 2, "restore only a compatible workload cache"
    main_push = "github.event_name == 'push' && github.ref == 'refs/heads/main'"
    save = re.search(r"- name: Cache Go compilation\n(.*?)(?=\n    - |\Z)", action, re.DOTALL)
    restore = re.search(r"- name: Restore Go compilation\n(.*?)(?=\n    - |\Z)", action, re.DOTALL)
    assert save is not None and "if: " + main_push in save.group(1), "only main pushes may save the workload cache"
    assert "uses: actions/cache@v6" in save.group(1)
    assert restore is not None, "other events must restore the workload cache"
    assert "if: github.event_name != 'push' || github.ref != 'refs/heads/main'" in restore.group(1)
    assert "uses: actions/cache/restore@v6" in restore.group(1), "PR runs must not save workload caches"


def assert_ci_workload_caches() -> None:
    action_path = ROOT / ".github/actions/setup-go-cache/action.yml"
    assert action_path.is_file(), "missing workload-scoped Go cache action"
    action = action_path.read_text()
    assert_workload_cache_action(action)
    assert_go_toolchain_workflows([(action_path, action)])
    for token in ("runner.os", "runner.arch", "steps.go.outputs.go-version", "inputs.workload", "'go.sum'", "github.sha"):
        try:
            assert_workload_cache_action(action.replace(token, "removed"))
        except AssertionError:
            continue
        raise AssertionError(f"cache accepts missing identity {token}")
    for path, test_job in ((PR_WORKFLOW, "unit-test-shards"), (MAIN_WORKFLOW, "test-shards")):
        workflow = path.read_text()
        for job, workload in (("quality-checks", "quality"), ("lint", "lint"), ("vet-platforms", "vet-${{ matrix.goos }}"), (test_job, "test-${{ matrix.name }}"), ("build-platforms", "build-${{ matrix.name }}")):
            block = job_block(workflow, job)
            assert "uses: ./.github/actions/setup-go-cache" in block, f"{path}: {job} must own its compiled cache"
            assert "workload: " + workload in block
        quality = job_block(workflow, "quality-checks")
        assert "- parallel:" not in quality, f"{path}: Go producers must not contend on one quality runner"
        shards = job_block(workflow, test_job)
        assert 'ASC_BYPASS_KEYCHAIN: "1"' in shards, f"{path}: bypass must cover every command in a shard"
        assert "run: ${{ matrix.command }}" in shards
        assert "cache-hit" not in shards, f"{path}: cache restoration must never skip tests"


def main() -> None:
    assert_ci_workload_caches()
    assert_lint_analysis_cache()
    assert_test_result_cache_disabled()
    assert_go_tool_cache_identity()
    assert_lint_timeout_budget()
    assert_go_toolchain_source_accepts_normalized_scalars()
    assert_go_toolchain_source_rejects_step_local_violations()
    assert_go_toolchain_source()
    assert_govulncheck_version_source()
    assert_optimized_workflow(PR_WORKFLOW, "unit-test-shards")
    assert_optimized_workflow(MAIN_WORKFLOW, "test-shards")
    assert_optimized_workflow_rejects_weakened_checks()

    pr = PR_WORKFLOW.read_text()
    for required_job in ("format-and-lint", "unit-tests", "build"):
        assert "if: always()" in job_block(pr, required_job), f"required job {required_job} must always resolve"
    quality_gate = job_block(pr, "format-and-lint")
    assert "needs.website-checks.result" in quality_gate

    website = WEBSITE_WORKFLOW.read_text()
    assert "workflow_call:" in website
    assert "runs-on: ubuntu-latest" in job_block(website, "website")
    assert "make check-website-docs" in website
    assert "uses: ./.github/actions/setup-go-cache" in website and "workload: website" in website
    assert "cache: true" not in website, "website must not freeze a dependency-only setup-go cache"

    main = MAIN_WORKFLOW.read_text()
    assert "git diff-tree --no-commit-id --name-only --no-renames -r" in main
    main_windows = job_block(main, "telemetry-windows-tests")
    assert "[\"telemetry\", \"full\"]" in main_windows
    assert "needs.telemetry-windows-tests.result" in job_block(main, "test")

    release = RELEASE_WORKFLOW.read_text()
    assert "actions/upload-artifact" in release, "release workflow must retain official artifact publication"
    assert_winget_submission_retries_transient_failures()
    assert_winget_retry_guard_rejects_unguarded_calls()
    assert_winget_retry_helper_behavior()

    subprocess.run(
        ["go", "test", ".", "-run", "^TestReleaseRehearsalWorkflowContract", "-count=1"],
        cwd=ROOT,
        check=True,
    )
    subprocess.run([sys.executable, str(ROOT / "scripts/test_release_rehearsal.py")], check=True)

    dependabot = DEPENDABOT_CONFIG.read_text()
    assert dependabot == """version: 2

updates:
  - package-ecosystem: \"gomod\"
    directory: \"/\"
    schedule:
      interval: \"weekly\"

  - package-ecosystem: \"github-actions\"
    directory: \"/\"
    schedule:
      interval: \"weekly\"
""", "Dependabot must monitor root Go modules and GitHub Actions weekly"

    assert_security_target_contract()

    print("CI workflow contracts passed")


if __name__ == "__main__":
    main()
