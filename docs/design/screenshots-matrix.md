# Screenshot matrix capture and review

## Scope

Issue #2230 adds the `asc screenshots matrix` command. It runs an
existing local screenshot plan over a bounded device, locale, appearance, and
content-variant matrix, writes one isolated artifact directory per cell, and
creates a report that contains both successful and unsuccessful cells.

The command is local-only. It does not call the App Store Connect API, upload
artifacts, or create, boot, install, clone, or delete simulators. Target
simulators must already exist and be booted. Existing `screenshots run`,
`capture`, `frame`, and review commands retain their current behavior.

## Command placement and invocation

`ScreenshotsCommand` registers a new `shots.ShotsMatrixCommand` subcommand:

```text
asc screenshots matrix --plan .asc/screenshots-matrix.json [flags]
```

The command uses `shared.DefaultUsageFunc` and `d.BindOutputFlags`.
Besides the standard `--output` and `--pretty` flags, it accepts `--plan`,
`--max-concurrency`, `--max-attempts`, and `--retry-backoff`. Explicit runtime
flags override the corresponding plan values. Usage validation returns
`flag.ErrHelp`/the existing usage diagnostic path and therefore exit code 2.

## Matrix plan

The matrix plan is a separate JSON/JSONC document. `version` is currently 1 and
must be present. Its `base_plan` is a literal relative filename rooted at the
directory containing the matrix plan; absolute paths, escaping `..` paths,
symlinks, non-regular files, and oversized files are rejected. The base plan
remains the source of the bundle ID and ordered interaction steps. Matrix field
names must use the documented exact snake_case spelling; duplicate fields are
rejected rather than silently overwritten.

```jsonc
{
  "version": 1,
  "base_plan": "screenshots.json",
  "devices": [
    { "id": "iphone-17-pro", "udid": "SIMULATOR_UDID" },
    { "id": "ipad-pro-13", "udid": "ANOTHER_SIMULATOR_UDID" }
  ],
  "locales": ["en-US", "ja-JP"],
  "appearances": ["light", "dark"],
  "content_variants": [
    { "id": "default" },
    { "id": "empty", "launch_arguments": ["--fixture", "empty"] }
  ],
  "execution": {
    "max_concurrency": 2,
    "max_attempts": 2,
    "retry_backoff_ms": 500
  },
  "output": {
    "raw_dir": "./screenshots/matrix/raw",
    "framed_dir": "./screenshots/matrix/framed",
    "review_dir": "./screenshots/matrix/review",
    "frame": {
      "enabled": false,
      "device_by_matrix_device": {}
    }
  }
}
```

The example deliberately disables framing because matrix device labels must
never be mapped to a frame for another device family. When framing is enabled,
each matrix device needs an explicit supported mapping accepted by the existing
frame-device parser and checked against the actual simulator family during
inventory preflight.

The product of the four axis lengths must be non-zero and no more than 256
cells. Device IDs, content-variant IDs, and screenshot names must be unique
(case-insensitively) and safe path components. Device UDIDs must be present and
unique across device declarations after whitespace/case normalization.
Appearance is case-insensitively normalized to `light` or `dark`.
Locales are non-empty and use the existing locale normalization helper.

## Execution model

Expansion order is device declaration order, locale declaration order,
appearance declaration order, then content-variant declaration order. Each
cell has the stable ID:

```text
<device-id>|<locale>|<appearance>|<content-variant-id>
```

The executor reuses the validated base `Plan` and overrides its simulator UDID
and output directory in memory. It passes locale launch overrides and literal
content-variant arguments to each `launch` step. Locale overrides are:

```text
-AppleLanguages (<language-or-language-script>)
-AppleLocale <locale-with-underscore>
```

Script-specific locales retain the normalized script in `AppleLanguages`
(for example, `zh-Hans-CN` uses `(zh-Hans)`), while region-only locales keep
the language-only behavior (`pt-BR` uses `(pt)`).

Content arguments are appended without shell interpolation. A content variant
that tries to override Apple language or locale arguments is rejected during
validation.

The worker pool has a hard maximum of eight workers and a default of one. A
per-UDID mutex serializes cells targeting the same simulator so appearance
changes cannot race. Before a cell, the executor reads the simulator's
appearance with `xcrun simctl ui <device> appearance`, applies the requested
state with the same supported `simctl ui` interface, executes the plan, and
restores the original state in a deferred cleanup path. A restore failure is
surfaced as `cleanup_failed` and prevents later cells on that simulator.

Simulator inventory preflight has a bounded 30-second deadline. Capture and
framing operations honor the caller context. Appearance restoration runs on a
detached context with the same 30-second deadline so cleanup can complete after
caller cancellation. This slice does not add separate per-stage deadlines or
attempt pair recovery after an external process crash.

`max_attempts` is the total number of attempts and defaults to one, with a hard
maximum of three. Execution and framing failures retry the complete cell after
the configured backoff. Validation, cancellation, and cleanup failures do not
retry. Independent cells continue after a failure. Caller context cancellation
stops new work, cancels external commands, records unfinished cells as canceled,
and writes the partial report. A deadline reached only by bounded inventory
preflight is reported as a preflight `simulator_not_ready` failure instead of
cancellation.

## Artifact and report contract

For a base screenshot step named `home`, cell artifacts use:

```text
raw/<locale>/<device-id>/<appearance>/<content-variant-id>/home.png
framed/<locale>/<device-id>/<appearance>/<content-variant-id>/home.png
```

Each attempt writes to a temporary attempt path and promotes only validated
successes to the final path. The command does not recursively delete prior
outputs; stale files are excluded from the explicit current-run manifest.

Every validated invocation writes `review/manifest.json` and
`review/index.html`, even when one or more cells fail. The matrix manifest has
one entry per planned cell and contains the logical device label rather than the
simulator UDID. Entries include cell axes, status, attempts, duration, step
results, raw/framed paths, dimensions, and sanitized failure stage/code. Launch
arguments, raw command output, environment values, credentials, keychain paths,
and simulator UDIDs are not persisted.

Publication is serialized per review directory across processes. The manifest
is written last as the commit marker and records the SHA-256 digest of the exact
HTML bytes. Matrix manifest loading and the default review opener reject a torn
or externally mixed HTML/manifest generation instead of displaying it.

The HTML report is self-contained and network-free. It displays all cells,
including failures and cancellations, and links only to local raw/framed
artifacts. Plan-provided labels are escaped. When the default review opener
launches a browser for a digest-bound matrix report, it takes fresh, bounded
open-time copies of the validated HTML and its referenced raw/framed bytes
from anchored roots into an owner-only private snapshot directory and rewrites
links to those copies. The HTML/manifest digest still binds the report pair;
asset bytes are fresh open-time snapshots and are not persisted as a separate
per-asset digest contract. Successful snapshots are retained for asynchronous
browser consumption and reclaimed opportunistically when older than 24 hours;
a failed browser launch removes its snapshot immediately. Legacy/custom HTML
without a valid matrix binding keeps the historical direct-open behavior.
On Windows, private staging and browser-snapshot objects receive a protected
owner-only DACL at creation; the rooted parent/child locks remain in force
while external capture or framing tools run.
Raw-only reports are valid when framing is disabled. Approval and App Store
upload integration are explicitly out of scope.

The command prints the structured result through the existing output helpers.
Computed result fields use the CLI's governed camelCase contract: `planPath`,
`bundleId`, `rawDir`, `framedDir`, `reviewDir`, `status`, `totalCells`,
`succeeded`, `failed`, `canceled`, `retried`, `cleanupFailed`, `cells`, and
`review` (with `manifestPath` and `htmlPath`). Each cell error is an object
containing only a sanitized `stage`, `code`, and `message`; step errors are
sanitized as well. The on-disk review manifest uses the same camelCase result
contract.
All cells succeeding returns nil and exit code 0. Partial or failed execution
writes its result/artifacts and then returns a runtime error for exit code 1.
Invalid flags/plans return usage exit code 2 before side effects.

## Implementation locations

- `internal/screenshots/matrix.go`: plan types, validation, expansion,
  scheduling, cell execution, retry, and result types.
- `internal/screenshots/matrix_review.go`: explicit matrix manifest and HTML
  rendering, reusing safe review rendering helpers where appropriate.
- `internal/asc/output_screenshots.go`: exported camelCase result and review
  manifest contracts plus registered table/Markdown renderers.
- `internal/cli/shots/shots_matrix.go`: flags, command execution, and output
  selection.
- `internal/cli/shots/shots_matrix_output.go`: conversion from the local
  executor model to the governed public output model.
- `internal/cli/screenshots/screenshots.go`: register the subcommand.
- `internal/cli/cmdtest/shots_matrix_test.go` and package tests: CLI and unit
  coverage with injected command/frame/state runners.
- `docs/COMMANDS.md`, README or local workflow docs: generated help and usage
  example.

Prefer a small injectable process runner and appearance-state interface over
tests that require Xcode. Reuse `Plan` parsing/validation, step semantics,
`Capture`, `Frame`, and existing output conventions without changing their
public behavior.

## Verification

RED tests should cover command registration, required/invalid flags, plan
validation, deterministic expansion, path safety, literal argument forwarding,
per-UDID serialization, concurrency bounds, state restoration, retry behavior,
partial results, report escaping, and output streams/exit codes. Fake `xcrun`,
`axe`, and framing runners should prove execution without a simulator.

Run focused package tests, then:

```bash
make build
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```

On macOS, perform one opt-in smoke test with two already-booted simulators and
an installed sample app. Confirm that cells are isolated, same-device cells do
not overlap, appearance is restored, review artifacts contain failures, and no
App Store Connect request occurs.

The implementation follows the repository handoff contract: additive commits
are pushed for review and must remain unmerged until explicit merge
authorization. Unresolved risks are the absence of a live Simulator smoke test
on this host and dependence on the installed Xcode `simctl ui` appearance
interface and selected framing profiles for any opt-in framed run.

## Alternatives rejected

Extending the existing Plan v1 schema would make a single-device sequence
carry matrix-only fields and risk changing `screenshots run` behavior. A
separate matrix document keeps the existing contract stable while reusing its
steps. Creating simulators or installing builds would make this slice depend on
build/signing lifecycle and introduce cleanup risks; prebooted explicit UDIDs
keep the first implementation deterministic and local.
