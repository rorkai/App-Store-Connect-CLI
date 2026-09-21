---
name: asc-workflow
description: Define, validate, run, resume, and audit repo-local multi-step automations with current `asc workflow` and `.asc/workflow.json`, including step outputs and safe release/TestFlight workflows.
---

# asc workflow

Use this skill when you need lane-style automation inside the CLI using:

- `asc workflow validate`
- `asc workflow list`
- `asc workflow run`

Workflows are repo-local automation files. They run trusted shell commands, stream step output to stderr, and keep stdout as machine-readable JSON.

## Command discovery

Always verify flags with:

```bash
asc workflow --help
asc workflow validate --help
asc workflow list --help
asc workflow run --help
```

## End-to-end flow

1. Author `.asc/workflow.json`.
2. Validate structure and references:

```bash
asc workflow validate
```

3. Discover public workflows:

```bash
asc workflow list
asc workflow list --all
```

4. Preview execution:

```bash
asc workflow run --dry-run beta BUILD_ID:123456789 GROUP_ID:abcdef
```

5. Execute:

```bash
asc workflow run beta BUILD_ID:123456789 GROUP_ID:abcdef
```

6. If a recoverable run fails, resume with the run ID from the JSON result:

```bash
asc workflow run release --resume "release-20260312T120000Z-deadbeef"
```

Do not pass extra `KEY:VALUE` params with `--resume`; the saved workflow file, params, and persisted outputs are reused.

## File location and format

- Default path: `.asc/workflow.json`
- Override path: `asc workflow run --file ./path/to/workflow.json <name>`
- JSONC comments are supported.
- Top-level hooks: `before_all`, `after_all`, `error`
- Workflow keys: `description`, `private`, `env`, `steps`
- Step forms:
  - string shorthand: `"echo hello"`
  - `run` shell command
  - `workflow` sub-workflow call
  - `name` label
  - `if` conditional var name
  - `with` env overrides for workflow-call steps
  - `outputs` map for JSON stdout extraction from named run steps
  - `retry` fixed-delay retry policy for a `run` step
  - `timeout` positive per-attempt duration for a `run` step

## Outputs

Run steps can declare outputs. The command must emit JSON on stdout, so pass `--output json` for `asc` commands that produce outputs.

Output references use:

```text
${steps.step_name.OUTPUT_NAME}
```

Rules:

- A step that declares `outputs` must have a reference-safe `name`.
- Outputs are allowed on `run` steps, not workflow-call steps.
- Output-producing names must be unique across workflows that can execute together in the same run graph.
- Persisted outputs are stored in workflow run state, so do not map secrets into outputs.

## Bounded retry and timeout

Add retry only to commands you have determined are safe to repeat. The runner does not classify commands or retry mutations on its own.

```json
{
  "name": "resolve_build",
  "run": "asc builds info --app $APP_ID --latest --platform IOS --output json",
  "retry": {
    "max_attempts": 3,
    "delay": "5s"
  },
  "timeout": "2m"
}
```

`retry.max_attempts` counts the first attempt and must be between 2 and 100. Retry delays and timeouts must be positive durations no longer than 24 hours. Both fields work only on `run` steps, not workflow calls or lifecycle hooks.

A timeout without retry is terminal because the command may have completed remotely. Pairing retry and timeout is the explicit signal that another attempt is safe. Resume requires either a successful checkpoint or a retry-enabled failed step; output-extraction failures cannot be resumed.

## Runtime params

`asc workflow run <name> [KEY:VALUE ...]` supports both separators:

```bash
asc workflow run beta VERSION:2.1.0
asc workflow run beta VERSION=2.1.0
```

Repeated keys are last-write-wins. In shell commands, reference params through shell expansion like `$VERSION`.

## Env precedence

Main workflow run:

```text
definition.env < workflow.env < CLI params
```

Sub-workflow call with `with`:

```text
sub-workflow env < caller env and params < step with
```

## Conditionals

Add `"if": "VAR_NAME"` to a step. Truthy values are `1`, `true`, `yes`, `y`, and `on`, case-insensitive. Lookup checks merged workflow env/params first, then process environment.

## Example workflow

```json
{
  "env": {
    "APP_ID": "123456789",
    "VERSION": "1.0.0",
    "GROUP_ID": ""
  },
  "before_all": "asc auth status",
  "after_all": "echo workflow_done",
  "error": "echo workflow_failed",
  "workflows": {
    "beta": {
      "description": "Resolve the latest build and distribute it to TestFlight",
      "steps": [
        {
          "name": "resolve_build",
          "run": "asc builds info --app $APP_ID --latest --platform IOS --output json",
          "outputs": {
            "BUILD_ID": "$.data.id"
          }
        },
        {
          "name": "list_groups",
          "run": "asc testflight groups list --app $APP_ID --limit 20 --output json"
        },
        {
          "name": "add_build_to_group",
          "if": "GROUP_ID",
          "run": "asc builds add-groups --build-id ${steps.resolve_build.BUILD_ID} --group $GROUP_ID"
        }
      ]
    },
    "release": {
      "description": "Validate, stage, and submit an App Store version",
      "steps": [
        {
          "name": "validate",
          "run": "asc validate --app $APP_ID --version $VERSION --platform IOS --output json"
        },
        {
          "name": "stage",
          "run": "asc release stage --app $APP_ID --version $VERSION --build-id $BUILD_ID --metadata-dir ./metadata/version/$VERSION --confirm --output json"
        },
        {
          "name": "submit",
          "if": "SUBMIT_FOR_REVIEW",
          "run": "asc review submit --app $APP_ID --version $VERSION --build-id $BUILD_ID --confirm --output json"
        }
      ]
    },
    "publish-appstore": {
      "description": "High-level upload plus App Store review submission",
      "steps": [
        {
          "name": "publish",
          "run": "asc publish appstore --app $APP_ID --ipa ./build/MyApp.ipa --version $VERSION --wait --submit --confirm --output json"
        }
      ]
    }
  }
}
```

## Useful invocations

```bash
asc workflow validate | jq -e '.valid == true'
asc workflow list --pretty
asc workflow list --all --pretty
asc workflow run --dry-run beta BUILD_ID:123 GROUP_ID:grp_abc
asc workflow run beta BUILD_ID:123 GROUP_ID:grp_abc | jq -e '.status == "ok"'
asc workflow run release BUILD_ID:123 SUBMIT_FOR_REVIEW:true
asc workflow run release --resume "release-20260312T120000Z-deadbeef"
```

## Safety rules

- Treat `.asc/workflow.json` like code; only run trusted workflow files.
- Avoid running workflows from untrusted PRs with secrets.
- Keep workflow files in version control.
- Validate first, dry-run next, then run.
- Use explicit IDs and `--confirm` for mutating steps.
- Use `asc validate`, `asc release stage`, `asc review submit`, and `asc publish appstore`; do not use removed submission commands.
