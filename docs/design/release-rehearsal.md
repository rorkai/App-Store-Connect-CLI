# Hosted Release Rehearsal

## Placement and invocation

The repository gets a manually dispatched `Release rehearsal` GitHub Actions
workflow. It accepts a required candidate semantic version and a required Git
ref or commit SHA. The workflow resolves that input through `actions/checkout`,
records the exact checked-out SHA, and passes both values to the repository's
release rehearsal script. Its guardrails run on `macos-latest`, matching the
publishing workflow so Darwin-only release tests are exercised.

The rehearsal uses untagged `make build-all` cross-builds. Self-contained macOS
release packages are qualified separately by the non-publishing
`Native macOS release packages` workflow, which runs the actual release builder.
The rehearsal keeps its existing runner-local output and read-only contract.

The local invocation uses the same entrypoint:

```bash
python3 scripts/release_rehearsal.py \
  --version 4.5.0 \
  --expected-sha "$(git rev-parse HEAD)"
```

## Behavior and safety

The script validates the candidate version against every semantic-version tag
in the repository, then validates the exact commit and a clean tracked and
untracked source tree before it invokes Make. It then runs release guardrails,
builds the supported release binaries, and verifies the source tree again
before it checks artifact names, creates checksums, and generates a local
Markdown preview from commits since the latest semantic-version tag merged into
the candidate. Only the configured release output directory is excluded from
the cleanliness checks. Diagnostics and the tested SHA go to the job log and
GitHub job summary.

A custom release output directory is passed through to the build, but it must
not contain tracked files, overlap Git metadata, or contain the repository. The
rehearsal safely quotes whitespace in valid paths and rejects Make or shell
metacharacters before Make can run its cleaning prerequisite. Non-default
custom output paths must be absent or empty, so cleanup cannot erase unrelated
existing content.

Guardrails and release builds run with `GOWORK=off`, preventing an ignored
local `go.work` or `go.work.sum` from replacing the dependencies represented by
the tested commit.

The workflow has read-only repository permissions and disables interactive Git
and GitHub CLI prompts. It does not import signing certificates, sign binaries,
create or push tags, create or update GitHub releases, upload artifacts, update
Homebrew, or submit WinGet changes. No App Store Connect API is involved.

The requested candidate ref must contain this workflow's rehearsal script and
`release-guardrails` target. Refs predating the rehearsal feature fail early
with an explicit diagnostic. Supporting those refs would require running newer
orchestration code against older source, which would weaken the exact-source
contract and is intentionally outside this pre-tag workflow.

## Compatibility and failure behavior

This adds an opt-in workflow and script without changing the publishing
workflow or CLI surface. Missing inputs, an invalid version, a mismatched SHA,
an existing candidate tag, no commits after the latest release, missing build
artifacts, a dirty source tree before or after the build, or a failed release
guardrail produce a nonzero exit. Unsafe release output paths fail before Make.
The preview and artifacts remain runner-local and expire with the job.

## Verification

Workflow-contract tests establish RED before the workflow exists, then assert
the parsed dispatch inputs, checkout binding, release-platform runner, prompt
guards, every permissions block, script entrypoint, persisted SHA flow,
generated summary content, and absence of publishing steps or commands. The
workflow invokes one Python entrypoint; its dynamic orchestration test proves
that `make release-guardrails` runs before `build-all`, avoiding a second
YAML-owned orchestration path. Script tests cover valid and invalid versions, SHA
mismatches, repository-wide and candidate-ancestor tag boundaries, dirty
tracked and untracked inputs, repository-root output rejection, post-build
source mutation, release-output exclusion, and artifact validation. A local
safe rehearsal verifies the end-to-end script entrypoint without credentials.

Keeping this as a separate non-publishing workflow is preferable to adding a
dry-run flag to the release workflow: publishing steps and secrets are absent
by construction. A shell-only workflow would be smaller, but would duplicate
release rules and make the contract harder to exercise locally.
