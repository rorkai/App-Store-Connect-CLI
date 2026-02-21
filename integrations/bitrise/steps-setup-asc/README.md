# Setup asc CLI (Bitrise Step Prototype)

This folder contains a first-pass implementation for issue `#712`:
https://github.com/rudrankriyam/App-Store-Connect-CLI/issues/712

Recommended Step naming from cross-issue alignment:

- Step repository: `rudrankriyam/steps-setup-asc`
- Step ID: `setup-asc`
- Step title: `Setup asc CLI`

## What v1 does

- `mode=install`: installs `asc` from GitHub Releases and exports:
  - `ASC_CLI_PATH`
  - `ASC_CLI_VERSION`
- `mode=run`: installs `asc`, sets optional `ASC_*` auth environment variables, runs a command, and exports:
  - `ASC_COMMAND_EXIT_CODE`

## Example usage in a Bitrise workflow

```yaml
workflows:
  primary:
    steps:
    - setup-asc:
        inputs:
        - mode: install
        - version: latest
    - script:
        inputs:
        - content: |-
            #!/usr/bin/env bash
            set -euo pipefail
            "${ASC_CLI_PATH}" --help
```

Install + run in one step:

```yaml
workflows:
  primary:
    steps:
    - setup-asc:
        inputs:
        - mode: run
        - version: latest
        - command: asc --help
```

## Local validation

From this directory:

```bash
bitrise run audit-this-step
bitrise run test-install
bitrise run test-run-help
```
