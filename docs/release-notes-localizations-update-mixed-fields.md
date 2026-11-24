# `asc localizations update` refuses fields of the other localization type

Changed in 7.4.0. `asc localizations update` writes one kind of localization
per call: a version localization (`--type version`, the default) or an app-info
localization (`--type app-info`). Each kind has its own field flags.

| Type | Field flags |
|---|---|
| `version` | `--description`, `--keywords`, `--whats-new`, `--promotional-text`, `--clear-promotional-text`, `--support-url`, `--marketing-url` |
| `app-info` | `--name`, `--subtitle`, `--clear-subtitle`, `--privacy-policy-url`, `--clear-privacy-policy-url`, `--privacy-choices-url`, `--privacy-policy-text` |

Through 7.3.1, a call that passed field flags of both kinds was accepted. The
flags of the selected type were sent, and the flags of the other type were
dropped without a message:

```bash
# 7.3.1: updated the description; --name was ignored
asc localizations update --id "LOCALIZATION_ID" --name "New name" --description "New description"
```

7.4.0 stops before any request, with exit code `2`:

```text
$ asc localizations update --id "LOCALIZATION_ID" --name "New name" --description "New description"
Error: localizations update: version fields (--description) and app-info fields (--name) cannot be combined in one command; update version fields with --type version and app-info fields with --type app-info in separate commands
```

The error lists the flags of each kind that the call passed. A field flag
counts when it is passed, even with an empty value.

A call whose field flags all belong to the other type also exits `2`, as it
did before, with a line that names the type to select:

| Call | 7.3.1 | 7.4.0 |
|---|---|---|
| `--id ID --name X` (app-info flag, type `version`) | `Error: at least one update flag is required (--description, ...)` | `Error: --type app-info is required for app-info update flags` |
| `--type app-info --id ID --description X` (version flag, type `app-info`) | `Error: at least one update flag is required (--name, ...)` | `Error: --type version is required for version update flags` |

## Migration

Run one command per localization type. The two kinds are different resources,
so each command needs its own localization ID, or its own selector:

```bash
asc localizations update --version "VERSION_ID" --locale "en-US" --description "New description"
asc localizations update --type app-info --app "APP_ID" --locale "en-US" --name "New name"
```

A script that passed both kinds in one call should check what it expected to
change: under 7.3.1 only the fields of the selected type were written.
