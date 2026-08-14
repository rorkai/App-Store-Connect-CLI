# Workflow Patterns

Use the high-level workflow surfaces deliberately:

- `asc publish appstore`: canonical App Store shipping path
- `asc publish testflight`: canonical high-level TestFlight publish path
- `asc workflow`: user-defined orchestration for repo-specific pipelines

`asc workflow` lets you compose existing `asc` commands and shell commands into
repeatable release pipelines once you know which top-level path you want.

## Verified local Xcode -> TestFlight workflow

This pattern was validated against a real app using:

- `asc builds next-build-number` to choose the next build number for a version
- `asc xcode inject` to materialize deployment metadata into generated Xcode
  plist/config files and asset paths before archiving
- `asc xcode build` to compile a scheme for an explicit simulator or device
  destination before the archive/export path
- `asc xcode archive` to create a deterministic `.xcarchive`
- `asc xcode export` to create a deterministic `.ipa`
- `asc publish testflight --group ... --wait` to upload, wait for processing,
  and add the build to a TestFlight group
- `--submit --confirm` on `asc publish testflight` when the target is an
  external group that should trigger beta app review submission

For a simulator compile check without modifying project files, use the typed
destination and explicit no-signing flag. For that invocation,
`--no-code-signing` overrides signing with `CODE_SIGNING_ALLOWED=NO`. Derived
data defaults to a stable asc cache path outside the checkout; use
`--derived-data-path` when the workflow needs a specific artifact directory.
Use `--result-bundle-path` to produce a new `.xcresult` bundle at an explicit
path; the destination must not already exist and asc does not overwrite it.

Replace the example destination below with a simulator installed on the host.

```bash
asc xcode build \
  --project App.xcodeproj \
  --scheme App \
  --configuration Debug \
  --destination 'platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0' \
  --no-code-signing \
  --output json
```

Device builds retain Xcode's signing behavior unless `--no-code-signing` is
provided explicitly.

`asc xcode export` generates archive-specific App Store Connect export options
automatically when `--export-options` is omitted. It chooses a unique
archive-adjacent path and never overwrites an existing file. The command result
reports the exact path used. To persist a plist at a deterministic path for
inspection or reuse, generate it after the archive:

```bash
asc xcode export-options generate \
  --archive-path .asc/artifacts/App.xcarchive \
  --output-path .asc/export-options-app-store.plist \
  --overwrite
```

The standalone command defaults to `.asc/export-options-app-store.plist` and
requires `--overwrite` if that file already exists. Automatic generation is
cross-platform and only reads archive metadata. Manual signing resolution is
Darwin-only because it inspects local Xcode signing identities and provisioning
profiles.

To export an IPA for registered devices, use the experimental modern Xcode
method name `release-testing` (the older `ad-hoc` spelling is deprecated):

```bash
asc xcode export \
  --archive-path .asc/artifacts/App.xcarchive \
  --ipa-path .asc/artifacts/App.ipa \
  --method release-testing \
  --signing-style manual \
  --team-id TEAM_ID
```

The default remains `app-store-connect`; release-testing always produces a
local export and cannot be combined with `--wait`.

For a PCC-capable app, or another multi-target app that needs manual signing,
pass the signing policy directly to export. ASC reads the archive and matches
installed profiles for the app and its embedded targets, so the command does
not need profile UUID flags or a checked-in plist:

```bash
asc xcode export \
  --archive-path .asc/artifacts/App.xcarchive \
  --ipa-path .asc/artifacts/App.ipa \
  --signing-style manual \
  --team-id TEAM_ID
```

The same flags work when the archive and export are owned by a local publish
flow:

```bash
asc publish testflight \
  --app APP_ID \
  --workspace App.xcworkspace \
  --scheme App \
  --version 1.2.3 \
  --group GROUP_ID \
  --signing-style manual \
  --team-id TEAM_ID
```

An explicit `--export-options` plist cannot be combined with `--method`,
`--signing-style`, or `--team-id`. When supplied without those flags, the
plist is authoritative.

Create `.asc/deployment.json`:

```json
{
  "values": {
    "bundle_id": "com.example.app",
    "app_name": "Example",
    "version": "",
    "build_number": ""
  },
  "outputs": [
    {
      "type": "plist",
      "path": "../Generated/Info.generated.plist",
      "values": {
        "CFBundleIdentifier": "${bundle_id}",
        "CFBundleDisplayName": "${app_name}",
        "CFBundleShortVersionString": "${version}",
        "CFBundleVersion": "${build_number}"
      }
    },
    {
      "type": "text",
      "path": "../Generated/Deployment.xcconfig",
      "contents": "PRODUCT_BUNDLE_IDENTIFIER = ${bundle_id}\nMARKETING_VERSION = ${version}\nCURRENT_PROJECT_VERSION = ${build_number}\n"
    },
    {
      "type": "copy",
      "source": "../Assets/AppIcon.appiconset/Contents.json",
      "path": "../Generated/Assets.xcassets/AppIcon.appiconset/Contents.json"
    }
  ]
}
```

Point your Xcode project at generated files such as `Generated/Info.generated.plist`
or include `Generated/Deployment.xcconfig` from the build configuration. Then run
`asc xcode inject` before archive time to fill in the release-specific values
that previously came from Fastlane scripts.

Create `.asc/workflow.json`:

```json
{
  "env": {
    "APP_ID": "1234567890",
    "PROJECT_PATH": "App.xcodeproj",
    "SCHEME": "App",
    "CONFIGURATION": "Release",
    "TESTFLIGHT_GROUP": "Beta",
    "VERSION": ""
  },
  "workflows": {
    "testflight_beta": {
      "description": "Archive, export, upload, and distribute an app to a TestFlight group.",
      "steps": [
        {
          "name": "validate_version",
          "run": "if [ -z \"$VERSION\" ]; then echo \"VERSION is required\" >&2; exit 1; fi"
        },
        {
          "name": "resolve_next_build",
          "run": "asc builds next-build-number --app \"$APP_ID\" --version \"$VERSION\" --platform IOS --initial-build-number 1 --output json",
          "outputs": {
            "BUILD_NUMBER": "$.nextBuildNumber"
          }
        },
        {
          "name": "inject_metadata",
          "run": "asc xcode inject --manifest .asc/deployment.json --set version=\"$VERSION\" --set build_number=${steps.resolve_next_build.BUILD_NUMBER} --overwrite --output json",
          "outputs": {
            "GENERATED_FILES": "$.outputs"
          }
        },
        {
          "name": "archive",
          "run": "asc xcode archive --project \"$PROJECT_PATH\" --scheme \"$SCHEME\" --configuration \"$CONFIGURATION\" --archive-path \".asc/artifacts/App-$VERSION-${steps.resolve_next_build.BUILD_NUMBER}.xcarchive\" --clean --overwrite --xcodebuild-flag=-destination --xcodebuild-flag=generic/platform=iOS --xcodebuild-flag=-allowProvisioningUpdates --xcodebuild-flag=MARKETING_VERSION=$VERSION --xcodebuild-flag=CURRENT_PROJECT_VERSION=${steps.resolve_next_build.BUILD_NUMBER} --output json",
          "outputs": {
            "ARCHIVE_PATH": "$.archive_path",
            "VERSION": "$.version",
            "BUILD_NUMBER": "$.build_number"
          }
        },
        {
          "name": "export",
          "run": "asc xcode export --archive-path ${steps.archive.ARCHIVE_PATH} --ipa-path \".asc/artifacts/App-$VERSION-${steps.archive.BUILD_NUMBER}.ipa\" --overwrite --timeout 10m --xcodebuild-flag=-allowProvisioningUpdates --output json",
          "outputs": {
            "IPA_PATH": "$.ipa_path",
            "VERSION": "$.version",
            "BUILD_NUMBER": "$.build_number"
          }
        },
        {
          "name": "publish",
          "run": "asc publish testflight --app \"$APP_ID\" --ipa ${steps.export.IPA_PATH} --group \"$TESTFLIGHT_GROUP\" --wait --poll-interval 10s --output json",
          "outputs": {
            "BUILD_ID": "$.buildId",
            "BUILD_NUMBER": "$.buildNumber"
          }
        }
      ]
    }
  }
}
```

Run it:

```bash
asc workflow validate --output json
asc workflow run --dry-run testflight_beta VERSION:1.2.3
asc workflow run testflight_beta VERSION:1.2.3
```

### Resumable upload and distribution steps

When upload and external distribution need separate retry boundaries, the
experimental upload-only flag can make the upload its own output-producing
step. The successful upload step is persisted with `BUILD_ID`; if the later
processing wait or distribution step fails, `--resume` skips the upload and
reuses that exact build ID.

```json
{
  "env": {
    "APP_ID": "1234567890",
    "IPA_PATH": ".asc/artifacts/App.ipa",
    "TESTFLIGHT_GROUP": "External Testers"
  },
  "workflows": {
    "testflight_external": {
      "steps": [
        {
          "name": "upload",
          "run": "asc publish testflight --app \"$APP_ID\" --ipa \"$IPA_PATH\" --upload-only --output json",
          "outputs": {
            "BUILD_ID": "$.buildId",
            "BUILD_VERSION": "$.buildVersion",
            "BUILD_NUMBER": "$.buildNumber"
          }
        },
        {
          "name": "wait",
          "run": "asc builds wait --build-id ${steps.upload.BUILD_ID} --fail-on-invalid --output json"
        },
        {
          "name": "distribute",
          "run": "asc builds add-groups --build-id ${steps.upload.BUILD_ID} --group \"$TESTFLIGHT_GROUP\" --submit --confirm --output json"
        }
      ]
    }
  }
}
```

After a failed wait or distribution step, use the run ID printed by
`asc workflow`:

```bash
asc workflow run testflight_external --resume RUN_ID
```

The upload step is not executed again because its declared outputs were already
persisted in the run-state file.

Notes:

- `VERSION` must be a valid next marketing version for your app. If the latest
  App Store version is already `READY_FOR_DISTRIBUTION`, reusing that same
  version can cause App Store Connect to reject the upload.
- `TESTFLIGHT_GROUP` accepts either a beta group name or group ID.
- Add `"ASC_BYPASS_KEYCHAIN": "1"` to the top-level `env` block if you want the
  workflow to resolve credentials from environment variables or config instead
  of the macOS keychain.
- Output-producing step names only need to stay unique within workflows that
  can execute together in the same run graph. Independent workflows can reuse
  names like `archive` or `publish`.
- Declared outputs keep the exact JSON value the command printed. A numeric
  `$.nextBuildNumber` of `42` is stored and interpolated as `42`, so it can be
  passed straight to `CURRENT_PROJECT_VERSION` or another build-number flag.

## Bounded retry and timeout

Long-form `run` steps can opt into a fixed-delay retry policy and a per-attempt
timeout. This replaces shell retry loops for operations such as Apple's
eventually consistent build-to-beta-group relationship:

```jsonc
{
  "name": "add_build_to_group",
  "run": "asc builds add-groups --build-id $BUILD_ID --group $GROUP_ID",
  "retry": {
    "max_attempts": 6,
    "delay": "10s"
  },
  "timeout": "2m"
}
```

`max_attempts` includes the initial execution and must be between 2 and 100.
`delay` and `timeout` use positive Go duration strings up to `24h`; examples
include `250ms`, `10s`, and `2m`. The delay is fixed and has no jitter. Timeout
applies separately to each attempt. With the example above, the total policy is
bounded by six two-minute attempts plus five ten-second delays.

Retry and timeout are supported only on `run` steps. Workflow-call steps and the
`before_all`, `after_all`, and `error` string hooks remain single-execution.
Caller cancellation stops a running process tree or retry delay immediately.
The error hook runs only after the final attempt fails; `after_all` still runs
only after success.

Use retry only when you have explicitly decided the command is safe to repeat.
The runner does not infer whether a shell command is read-only, idempotent, or a
mutation. A successful command with invalid declared output is not retried,
because its side effect may already have happened. That failure is terminal:
the structured result sets `terminal: true`, the run state records the terminal
reason, and `--resume` rejects the run instead of executing the command again.
A timeout-only failure is terminal for the same replay-safety reason: local
termination cannot prove that a remote mutation was not accepted. A step that
configures both `retry` and `timeout` remains resumable because `retry` is the
explicit repeat-safety opt-in.
Failed-attempt diagnostics from `timeout` alone do not create a resume checkpoint.
Recovery requires a previously successful step or hook, or a retry-enabled
failed step. Omit `retry` or `timeout` to disable it; explicit `null` is invalid.

Attempt numbers and retry delays are written to stderr. stdout remains the one
machine-readable workflow result. Each attempt gets a fresh output buffer, and
declared outputs are extracted and persisted only from the successful attempt.
Persisted attempt diagnostics remain available across `--resume`; already
successful steps are never rerun.

Example:

```json
{
  "workflows": {
    "testflight_beta": {
      "steps": [
        {
          "name": "archive",
          "run": "printf '{\"buildId\":\"beta\"}'",
          "outputs": {
            "BUILD_ID": "$.buildId"
          }
        }
      ]
    },
    "appstore_release": {
      "steps": [
        {
          "name": "archive",
          "run": "printf '{\"buildId\":\"release\"}'",
          "outputs": {
            "BUILD_ID": "$.buildId"
          }
        }
      ]
    }
  }
}
```

This is valid because those workflows are independent. If a third workflow can
call both of them in the same run, the duplicate `archive` producers still need
to be renamed.
