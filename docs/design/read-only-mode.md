# Global read-only mode

Tracking issue: rorkai/App-Store-Connect-CLI#2563. Milestone 5.4.0.

## Problem

Agents drive roughly half of `asc` invocations, and three independent MCP
servers built on top of `asc` each re-implemented a "never mutate" guard:
Heimdall's `--read-only`, zelentsov's asc-mcp read-only mode, and
abd3lraouf's risk tiers. Each one is an allowlist of command names that has to
be updated whenever `asc` adds a command. Per-command `--confirm` flags cannot
provide the guarantee either: many mutations (`update`, `create`, relationship
edits, uploads) do not take `--confirm`, and an agent can always pass it.

## Contract

- `ASC_READ_ONLY` (truthy: `1`, `true`, `yes`, `y`, `on`) enables read-only
  mode for the process. The root flag `--read-only` enables it for one
  invocation. Either source enables; neither disables the other.
- Every `POST`, `PATCH`, `PUT`, and `DELETE` is refused before it is sent.
  `GET`, `HEAD`, and `OPTIONS` pass.
- The refusal is a single stderr line,
  `Error: <source> is set; refusing <METHOD> <target>`, where `<source>` is
  `ASC_READ_ONLY` or `--read-only` and `<target>` is the request path or URL
  without its query string.
- The process exits with `ExitReadOnly` (6). The 0-9 range holds the
  well-known, non-HTTP exit codes, and none of the existing ones fit: 3 is an
  Apple authorization outcome, 2 is a usage error, 5 is an API conflict. A
  refusal is a local policy decision, so it gets its own stable code.
- `--confirm` is not a separate signal. An earlier revision refused any
  command invoked with `--confirm` at validation time, but `--confirm` also
  guards purely local changes (`auth fix`, `auth export-to-config`,
  `notarization staple`, keychain operations), and refusing those contradicts
  the promise that local-only operations keep working. The transport check is
  the single enforcement point, so a remote `--confirm` mutation is refused
  when it issues its write; the preflight reads it may run first are reads.

### Root flag

`--read-only` is a root flag bound in `shared.BindRootFlags` next to
`--profile`, `--output`, and `--api-debug`. Root flags are parsed once before
any subcommand runs, so the flag is as non-invasive as the existing ones: no
command definition changes and the parsed value is visible to every client that
is built afterwards. `BindRootFlags` resets the flag state when a fresh root
flag set is bound and `cmd.Run` clears it on return, so one parse cannot leak
into the next in tests or embedded use.

## Enforcement points

Enforcement lives in the transport, not in commands, so a command that was
never taught about the mode is still covered. `readonly.Check(ctx, method,
target)` runs immediately before the request is built in:

| Client | Choke point |
| --- | --- |
| App Store Connect API | `asc.Client.newRequest` |
| Asset and build upload operations | `asc.executeUploadOperation` |
| Notary API and its S3 uploads | `asc.Client.newNotaryRequest`, `asc.newS3Request`, `asc.uploadSinglePartToS3` |
| App Store Connect web session | `web.Client.doRequestBaseWithHTTPClient` |
| Developer Portal | `web.Client.doDeveloperPortalHTTP` |
| Apple Ads | `appleads.Client.requestOnce`, `appleads.Client.UploadPlatformAsset` |
| StoreKit retention messaging | `storekit.Client.request` |
| Ad hoc distribution object store | `distribution.S3Store.Ensure` and `ReplaceCorrupt` |
| Slack notifications | `asc notify slack` |
| GitHub | `asc snitch` issue creation, Wall of Apps submission |
| Xcode Cloud usage alert webhook | `asc web xcode-cloud usage alert` |

### Subprocesses

`asc workflow run` runs each step as a shell command, and the root flag is
process-local, so `workflow.buildEnvSlice` forces `ASC_READ_ONLY=1` into every
step environment whenever the mode is on, applied after the step's declared
`env` overrides. A nested `asc` call is then refused by its own transport
checks.

This is inheritance, not containment, and the docs say so. A step's shell
command can clear the variable for something it starts
(`ASC_READ_ONLY=0 asc ...`), steps that run other tools (`curl`, vendor CLIs)
never reach these guards, and the subprocesses the CLI shells out to for local
work (`xcodebuild`, `codesign`, `git`) are outside the mode by design: it is a
statement about the requests `asc` itself makes. Refusing `workflow run`
outright under the mode was considered and rejected, because reviewing your own
workflow against production credentials is one of the main reasons to turn the
mode on; a workflow file you do not trust needs a credential that cannot
write, not a local flag.

### Reads transported as writes

Apple sends several reads as `POST`. Those call sites wrap their context with
`readonly.WithReadIntent` so the transport check passes:

- Analytics v1 and v2 queries (`web.doAnalyticsRequest`, `doAnalyticsV2Request`).
- Apple Ads selector lookups (`/find`, `/query`), report endpoints
  (`v5/reports/`), and search (`v5/search/`), classified by
  `EndpointSpec.ReadOnlyRequest`. The raw `asc ads api` passthrough stays
  method-based because the CLI cannot know a caller's intent.
- Developer Portal listings that send `X-HTTP-Method-Override: GET`, the team
  list used to establish Portal context, agreement history, app group listings,
  and website push ID listings.

Read intent is a context value rather than a client field so it applies to one
request and cannot be left on by mistake.

### Deliberately allowed

- JWT minting is local. Apple Ads OAuth token exchange, web sign-in and
  two-factor calls, and team (provider) selection only establish a session.
- Anonymous usage telemetry has its own opt-out and never touches account
  state.
- Local-only commands never reach a transport.

## Tests

- `internal/readonly`: source precedence, method classification, message
  format, query stripping.
- `httptest` in `internal/asc`, `internal/web`, `internal/appleads`,
  `internal/storekit`, and `internal/distribution`: mutating requests never
  leave the client; reads and POST-shaped reads still reach the server.
- `internal/cli/cmdtest`: stderr line and exit code 6 for a public API mutation
  under both sources, for a web mutation after its preflight reads, and for a
  confirmed delete that never reaches the transport; reads still succeed under
  `ASC_READ_ONLY`; the root flag does not leak between runs.
- `internal/workflow`: step environments carry `ASC_READ_ONLY=1` exactly once
  while the mode is on, including when a step's `env` block tries to override
  it, and carry nothing when the mode is off.

## Non-goals

- Reclassifying commands. The mode is a transport guard; it does not maintain a
  list of safe commands.
- Replacing API key roles. A read-only key remains the server-side guarantee.
