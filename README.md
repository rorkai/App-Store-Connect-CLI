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
asc apps list --output table
```

<!-- WALL-OF-APPS:START -->
## Wall of Apps

**36 apps ship with asc.** [See the Wall of Apps →](https://asccli.sh/#wall-of-apps)

Want to add yours? [Open a PR](https://github.com/rudrankriyam/App-Store-Connect-CLI/pulls).
<!-- WALL-OF-APPS:END -->

### Add Your App to the Wall

Use:
`make generate app APP="Your App Name" LINK="https://apps.apple.com/app/id1234567890" CREATOR="your-github-handle" PLATFORM="iOS,macOS"`

This updates `docs/wall-of-apps.json` and re-syncs the Wall snippet in `README.md`.

## Common Workflows

### TestFlight feedback and crashes

```bash
asc feedback --app "123456789" --paginate
asc crashes --app "123456789" --sort -createdDate --limit 10
```

### Builds and distribution

```bash
asc builds upload --app "123456789" --file "/path/to/MyApp.ipa"
asc testflight builds list --app "123456789" --output table
```

### Validate and submit

```bash
asc validate --app "123456789" --version "1.2.3"
asc submit --app "123456789" --version "1.2.3"
```

### Metadata and localization

```bash
asc localizations list --app "123456789"
asc app-info get --app "123456789" --output json --pretty
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
asc workflow run --file .asc/workflow.json --workflow release
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
- [docs/API_NOTES.md](docs/API_NOTES.md) - API quirks and behaviors
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) - CLI development and testing notes
- [docs/TESTING.md](docs/TESTING.md) - Testing patterns and conventions
- [docs/openapi/README.md](docs/openapi/README.md) - Offline OpenAPI snapshot + update flow
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guide

## Acknowledgements

Local screenshot framing uses Koubou (pinned to `0.13.0`) for deterministic device-frame rendering.
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
