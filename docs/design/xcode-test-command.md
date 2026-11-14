# First-class local Xcode test command

## Placement and scope

`asc xcode test` is a leaf beneath the existing local `asc xcode`
group. It runs an explicitly selected local `xcodebuild` test action and reads
the resulting `.xcresult` bundle through the active Xcode command-line tools.
It never calls App Store Connect, uploads an artifact, or mutates an Xcode
project. Xcode may boot or launch the simulator or device selected by the
destination.

The command supports three Xcode actions through one typed option:

```text
test                    xcodebuild ... test
build-for-testing       xcodebuild ... build-for-testing
test-without-building   xcodebuild ... test-without-building
```

`test` is the default. `test` and `build-for-testing` require exactly one of
`--project` or `--workspace` and a `--scheme`. `test-without-building` requires
an existing `--xctestrun` file and rejects those project-oriented selectors.
Every action requires one or more explicit `--destination` values so a run
does not depend on Xcode's host-specific destination choice.

## Invocation and flags

```bash
asc xcode test \
  --project App.xcodeproj \
  --scheme App \
  --configuration Debug \
  --destination 'platform=iOS Simulator,name=iPhone 17 Pro' \
  --result-bundle-path .asc/artifacts/App-tests.xcresult \
  --output json
```

Typed flags are `--project`, `--workspace`, `--scheme`, `--action`,
`--configuration`, repeatable `--destination`, `--test-plan`, `--xctestrun`,
repeatable `--only-testing`, repeatable `--skip-testing`,
`--derived-data-path`, `--result-bundle-path`, `--clean`,
`--no-code-signing`, `--code-coverage`, repeatable `--xcodebuild-flag`, and the
standard output flags. Empty values are invalid. Positional arguments are invalid.

`--test-plan` and `--xctestrun` are mutually exclusive. `--clean`,
`--no-code-signing`, `--configuration`, and `--derived-data-path` are only
valid for actions that select a project/workspace; `test-without-building`
rejects all four build-only controls. Repeatable destination and test-filter
values reject empty or control-character input but otherwise retain their
literal whitespace.
Raw passthrough arguments cannot override selectors, destinations, paths,
actions, or ASC-managed signing settings. Values are passed as individual argv
entries and retain their user-provided order and whitespace.

For test-executing actions, asc always supplies a new result-bundle path. An
explicit `--result-bundle-path` is resolved to an absolute path; an omitted path
is allocated below the user cache. Existing paths, directories, and symlink
destinations are rejected. `build-for-testing` does not need a result bundle,
but reports the generated `.xctestrun` file when exactly one safe candidate can
be identified under its derived-data directory.

## Result parsing and output

The result bundle is summarized with the active Xcode tooling rather than by
parsing human-readable `xcodebuild` logs. The current structured interface is
two reads: `xcresulttool get test-results summary --path PATH --compact` supplies
aggregate fields such as `totalTestCount`, `passedTests`, `failedTests`,
`skippedTests`, and `testFailures`; `xcresulttool get test-results tests
--path PATH --compact` supplies the recursive `testNodes` tree. The parser
flattens only `Test Case` nodes and takes bounded failure text from structured
failure-message children. It accepts closed `passed`, `failed`, `skipped`, and
expected-failure case statuses; expected failures remain nonfailing while their
count is reported separately. Aggregate counts must be nonnegative and cannot
exceed `totalTestCount`; `expectedFailures` reconciles any remaining aggregate
count when Xcode provides it, and is derived from the remainder when the field
is absent. When the flattened cases represent the same unit as the aggregate,
their status counts are cross-checked; multi-destination or repetition trees
are preserved even when their leaf count differs from the aggregate. Unknown
fields are ignored. Structured output is capped before parsing. Missing
required summary fields, malformed JSON, a missing result bundle, or
unavailable result tooling are explicit post-processing errors; asc must not
invent successful test counts or report success when any test failed.

JSON uses the registered exported output receipt with stable camelCase fields:

```json
{
  "action": "test",
  "project": "App.xcodeproj",
  "scheme": "App",
  "configuration": "Debug",
  "destinations": ["platform=iOS Simulator,name=iPhone 17 Pro"],
  "derivedDataPath": "/path/to/DerivedData",
  "resultBundlePath": "/path/to/App-tests.xcresult",
  "tests": {
    "total": 12,
    "passed": 10,
    "failed": 1,
    "skipped": 1,
    "expectedFailures": 0,
    "durationMs": 4812,
    "failures": [
      {"identifier": "AppTests/LoginTests/testInvalidPassword", "message": "assertion failed"}
    ]
  },
  "success": false,
  "durationMs": 5120,
  "exitStatus": 65
}
```

With `--code-coverage`, test-executing actions add a `coverage` object read with
`xccov view --report --json`: overall `lineCoverage` (0 to 1), `coveredLines`,
`executableLines`, and per-target `targets` sorted by name. If the report cannot
be read, asc prints one warning on stderr, omits `coverage`, and keeps the test
outcome and exit status.

`build-for-testing` omits `tests` and reports an `.xctestrun` path when safely
discovered. Ordinary Xcode process exit statuses are retained. Cancellation,
preflight, and result-post-processing errors do not claim a subprocess exit
status. Table and Markdown render the stable summary fields while test failure
details remain bounded. Live Xcode diagnostics stay on stderr; structured
stdout never contains the complete raw log or environment.

When the global `--report junit --report-file PATH` flags are supplied and a
structured test result exists, the command contributes one JUnit testcase per
parsed test and synthesizes bounded aggregate cases when summary counts are not
fully represented by the flattened tree. A zero-test summary produces no
passing placeholder. Usage and preflight failures retain the existing generic
command-level report. Report files continue to use the repository's
no-overwrite and restricted-permission writer behavior.

## Validation and failure behavior

All deterministic option and output-path validation occurs before creating a
directory or starting a subprocess. The local helper uses the existing
macOS-only Xcode availability and process-group cancellation behavior. A failed
Xcode action may still emit a parsed summary when the result bundle is readable,
but the command returns the original nonzero process error. A successful action
whose result bundle cannot be summarized returns a post-processing error and
keeps the artifact for diagnosis.

The command is visible in help and generated documentation on all platforms but
executes only on macOS with a usable Xcode installation. No App Store Connect
credentials or network access are required.

## Test and live validation plan

RED coverage starts with CLI tests for command discovery, action-specific flag
validation, repeatable destinations and filters, output formats, error streams,
and usage exit code. Core tests cover exact argv generation, result-bundle path
allocation and collision protection, action failures, cancellation, summary
parsing, bounded failure details, and JUnit conversion using injected process
and parser seams.

The full gate is `make build`, `make format`, `make check-docs`, `make lint`, and
`ASC_BYPASS_KEYCHAIN=1 make test`. On a host with full Xcode, live checks cover a
passing run, a failing run, build-for-testing followed by
test-without-building, multiple destinations, test filters, JUnit output, and
unchanged source files.

## Alternatives

Adding a raw `--xcodebuild-flag` escape hatch would expose test actions but
would not provide validation, structured output, artifact protection, or a
stable exit contract. Extending `asc xcode build` to infer a test action would
make the existing build result misleading and break its action invariant. A
dedicated typed command keeps compile and test semantics explicit while sharing
the existing local process and renderer infrastructure.
