---
name: asc-wall-submit
description: Submit or update a Wall of Apps entry in the App-Store-Connect-CLI repository using `asc apps wall submit`. Use when the user says "submit to wall of apps", "add my app to the wall", or "wall-of-apps".
---

# asc wall submit

Use this skill to add or update a Wall of Apps entry with the built-in CLI flow.

## When to use

- User wants to submit an app to the Wall of Apps
- User wants to update an existing Wall of Apps entry
- User asks for the exact Wall submission flow

## Required inputs

Use one of these input paths:

- Standard App Store flow: `app` ID
- Manual/pre-release flow: `link` plus `name`

## Submission workflow

1. Run commands from the `App-Store-Connect-CLI` repository root.
2. Preview first:
   - `asc apps wall submit --app "1234567890" --dry-run`
   - or `asc apps wall submit --link "https://testflight.apple.com/join/ABCDEFG" --name "My Beta App" --dry-run`
3. Apply with confirmation:
   - `asc apps wall submit --app "1234567890" --confirm`
   - or `asc apps wall submit --link "https://testflight.apple.com/join/ABCDEFG" --name "My Beta App" --confirm`
4. Review the generated PR plan and resulting change to `docs/wall-of-apps.json`.

## Guardrails

- Do not modify unrelated entries in `docs/wall-of-apps.json`.
- If submission fails due to invalid input, fix the inputs and rerun the CLI command.
- Keep submission path PR-based unless maintainers define an issue-based intake flow.

## Examples

Add new app:

`asc apps wall submit --app "1234567890" --confirm`

Submit a non-App-Store/TestFlight entry:

`asc apps wall submit --link "https://testflight.apple.com/join/ABCDEFG" --name "My Beta App" --confirm`
