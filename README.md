# Unofficial App Store Connect CLI

<p align="center">
  <a href="https://github.com/rudrankriyam/App-Store-Connect-CLI/releases/latest"><img src="https://img.shields.io/github/v/release/rudrankriyam/App-Store-Connect-CLI?style=for-the-badge&color=blue" alt="Latest Release"></a>
  <a href="https://github.com/rudrankriyam/App-Store-Connect-CLI/stargazers"><img src="https://img.shields.io/github/stars/rudrankriyam/App-Store-Connect-CLI?style=for-the-badge" alt="GitHub Stars"></a>
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Homebrew-compatible-blue?style=for-the-badge" alt="Homebrew">
  <img src="https://img.shields.io/github/downloads/rudrankriyam/App-Store-Connect-CLI/total?style=for-the-badge&color=green" alt="Downloads">
</p>

<p align="center">
  <img src="docs/images/banner.png" alt="asc -- App Store Connect CLI" width="600">
</p>

A fast, lightweight, and scriptable CLI for the App Store Connect API.
Automate iOS, macOS, tvOS, and visionOS release workflows from your terminal, IDE, or CI/CD pipeline.

## Table of Contents

- [asc skills](#asc-skills)
- [Sponsors](#sponsors)
- [Quick Start](#quick-start)
- [Wall of Apps](#wall-of-apps)
- [Common Workflows](#common-workflows)
- [Commands and Reference](#commands-and-reference)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## asc skills

Agent Skills for automating `asc` workflows including builds, TestFlight, metadata sync, submissions, and signing:
https://github.com/rudrankriyam/app-store-connect-cli-skills

## Sponsors

<p align="center">
  <a href="https://rork.com/">
    <img src="docs/images/rork-logo.svg" alt="Rork logo" width="180">
  </a>
</p>

[Rork](https://rork.com/) helps you build real mobile apps by chatting with AI, going from idea to phone in minutes and to the App Store in hours.

## Quick Start

### Install

```bash
# Homebrew (recommended)
brew install asc

# Install script (macOS/Linux)
curl -fsSL https://asccli.sh/install | bash
```

For source builds and contributor setup, see [CONTRIBUTING.md](CONTRIBUTING.md).

### Authenticate

```bash
asc auth login \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8
```

Generate API keys at:
https://appstoreconnect.apple.com/access/integrations/api

### First command

```bash
asc apps list
```

### Output defaults (TTY-aware)

`asc` chooses a default `--output` based on where stdout is connected:

- Interactive terminal (TTY): `table`
- Non-interactive output (pipes/files/CI): `json`

You can still set a global preference:

```bash
export ASC_DEFAULT_OUTPUT=markdown
```

And explicit flags always win:

```bash
asc apps list --output json
```

## Wall of Apps

[See the Wall of Apps →](https://asccli.sh/#wall-of-apps)

Want to add yours?
`asc apps wall submit --app "1234567890" --confirm`

The command uses your authenticated `gh` session to fork the repo and open a pull request that updates `docs/wall-of-apps.json`.
It resolves the public App Store name, URL, and icon from the app ID automatically. For manual entries that are not on the public App Store yet, use `--link` with `--name`.
Use `asc apps wall submit --dry-run` to preview the fork, branch, and PR plan before creating anything.

## Common Workflows

### TestFlight feedback and crashes

```bash
asc testflight feedback list --app "123456789" --paginate
asc testflight crashes list --app "123456789" --sort -createdDate --limit 10
asc testflight crashes log --submission-id "SUBMISSION_ID"
```

### Builds and distribution

```bash
asc builds upload --app "123456789" --ipa "/path/to/MyApp.ipa"
asc builds list --app "123456789" --output table
asc testflight groups list --app "123456789" --output table
```

### Release (high-level: validate + attach + submit)

```bash
# Dry-run first to preview steps
asc release run --app "123456789" --version "1.2.3" --build "BUILD_ID" --metadata-dir "./metadata/version/1.2.3" --dry-run

# Run the full pipeline: ensure version, apply metadata, attach build, validate, submit
asc release run --app "123456789" --version "1.2.3" --build "BUILD_ID" --metadata-dir "./metadata/version/1.2.3" --confirm

# Monitor status after submission
asc status --app "123456789"
```

Lower-level alternatives (for scripting or partial workflows):

```bash
asc validate --app "123456789" --version "1.2.3"
asc submit create --app "123456789" --version "1.2.3" --build "BUILD_ID" --confirm
```

### Metadata and localization

```bash
asc localizations list --app "123456789"
asc apps info view --app "123456789" --output json --pretty
```

### Screenshots and media

```bash
asc screenshots list --app "123456789"
asc video-previews list --app "123456789"
```

### Signing and bundle IDs

```bash
asc certificates list
asc profiles list
asc bundle-ids list
```

### Workflow automation

```bash
asc workflow run release
```

### Verified local Xcode -> TestFlight workflow

See [docs/WORKFLOWS.md](docs/WORKFLOWS.md) for a copyable `.asc/workflow.json`
and `ExportOptions.plist` that use `asc builds latest`, `asc xcode archive`,
`asc xcode export`, and `asc publish testflight --group ... --wait`.

```bash
asc workflow validate
asc workflow run --dry-run testflight_beta VERSION:1.2.3
asc workflow run testflight_beta VERSION:1.2.3
```

### Xcode Cloud workflows and build runs

```bash
# Trigger from a pull request
asc xcode-cloud run --workflow-id "WORKFLOW_ID" --pull-request-id "PR_ID"

# Rerun from an existing build run with a clean build
asc xcode-cloud run --source-run-id "BUILD_RUN_ID" --clean

# Fetch a single build run by ID
asc xcode-cloud build-runs get --id "BUILD_RUN_ID"
```

## Commands and Reference

Use built-in help as the source of truth:

```bash
asc --help
asc <command> --help
asc <command> <subcommand> --help
```

For full command families, flags, and discovery patterns, see:
- [docs/COMMANDS.md](docs/COMMANDS.md)

## Documentation

- [docs/CI_CD.md](docs/CI_CD.md) - CI/CD integration guides (GitHub Actions, GitLab, Bitrise, CircleCI)
- [docs/COMMANDS.md](docs/COMMANDS.md) - Command families and reference navigation
- [docs/WORKFLOWS.md](docs/WORKFLOWS.md) - Reusable workflow patterns, including local Xcode to TestFlight
- [docs/API_NOTES.md](docs/API_NOTES.md) - API quirks and behaviors
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) - CLI development and testing notes
- [docs/TESTING.md](docs/TESTING.md) - Testing patterns and conventions
- [docs/openapi/README.md](docs/openapi/README.md) - Offline OpenAPI snapshot + update flow
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guide

## Acknowledgements

Local screenshot framing uses Koubou (pinned to `0.14.0`) for deterministic device-frame rendering.
GitHub: https://github.com/bitomule/koubou

Simulator UI automation for screenshot capture and interactions uses AXe CLI.
GitHub: https://github.com/cameroncooke/AXe

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

[Rudrank Riyam](https://github.com/rudrankriyam)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=rudrankriyam/App-Store-Connect-CLI&type=Date)](https://star-history.com/#rudrankriyam/App-Store-Connect-CLI&Date)

---

<p align="center">
  <img src="https://cursor.com/marketing-static/icon-192x192.png" alt="Cursor logo" width="24" height="24" />
</p>

<p align="center">
  Built with Cursor
</p>

<p align="center">
  <sub>This project is an independent, unofficial tool and is not affiliated with, endorsed by, or sponsored by Apple Inc. App Store Connect, TestFlight, Xcode Cloud, and Apple are trademarks of Apple Inc., registered in the U.S. and other countries.</sub>
</p>
