# Contributing

Thanks for your interest in contributing to asc cli!

## Development Setup

Requirements:
- Go 1.26+

Clone and build:
```bash
git clone https://github.com/rudrankriyam/App-Store-Connect-CLI.git
cd App-Store-Connect-CLI
make build
```

Run tests:
```bash
ASC_BYPASS_KEYCHAIN=1 make test
```

Optional tooling:
```bash
make tools   # installs gofumpt + golangci-lint
make lint    # uses golangci-lint if installed, else go vet
make format  # gofmt + gofumpt (requires gofumpt; install with make tools)
```

## Integration Tests (Opt-in)

Integration tests hit the real App Store Connect API and are skipped by default.
Set credentials in your environment and run:

```bash
export ASC_KEY_ID="YOUR_KEY_ID"
export ASC_ISSUER_ID="YOUR_ISSUER_ID"
export ASC_PRIVATE_KEY_PATH="/path/to/AuthKey.p8"
export ASC_APP_ID="YOUR_APP_ID"

make test-integration
```

Headless alternative (write the key to a temp file at runtime):
```bash
export ASC_PRIVATE_KEY_B64="BASE64_KEY"
```

## Local API Testing (Optional)

If you have App Store Connect API credentials, you can run real API calls locally:

```bash
export ASC_KEY_ID="YOUR_KEY_ID"
export ASC_ISSUER_ID="YOUR_ISSUER_ID"
export ASC_PRIVATE_KEY_PATH="/path/to/AuthKey.p8"
export ASC_APP_ID="YOUR_APP_ID"

asc testflight feedback list --app "$ASC_APP_ID"
asc testflight crashes list --app "$ASC_APP_ID"
asc review status --app "$ASC_APP_ID"
asc reviews list --app "$ASC_APP_ID"
```

Credentials are stored in the system keychain when available, with a config fallback at
`~/.asc/config.json` (restricted permissions). A repo-local `./.asc/config.json` is also supported.
Do not commit secrets.

## Local Validation

Run this checklist before opening a PR:

```bash
make tools               # Install gofumpt + golangci-lint
make format              # Format code
make lint                # Lint code
make check-docs          # Verify repository + website docs are up to date
ASC_BYPASS_KEYCHAIN=1 make test  # Run tests (bypasses keychain)
make build               # Build binary
./asc --help             # Smoke-test the binary
```

## Pull Request Guidelines

- Keep PRs small and focused.
- Add or update tests for new behavior.
- When pruning repetitive tests, prefer grouped/table-driven suites, but keep representative high-signal assertions for response decoding and user-facing output formatting.
- Update `README.md` if behavior or scope changes.
- Avoid committing any credentials or `.p8` files.

## Support and Questions

- Use GitHub Discussions for install help, auth setup, workflow questions, and "how do I...?" support
- Use GitHub Issues for reproducible bugs and concrete feature requests
- When reporting a bug, include `asc version`, your OS, install method, exact command, stdout/stderr, whether it reproduces with `ASC_BYPASS_KEYCHAIN=1`, and redacted `ASC_DEBUG=api` output when safe

## Issue Triage Labels

Every newly created GitHub issue should leave initial triage with exactly one label from
each of these buckets:

- Type: `bug`, `enhancement`, or `question`
- Priority: `p0`, `p1`, `p2`, or `p3`
- Difficulty: `easy`, `medium`, or `hard`

Label meanings:

- `bug`: broken behavior, regression, incorrect output, or misleading UX
- `enhancement`: new feature, workflow improvement, or behavior expansion
- `question`: clarification or discussion where the work is not yet well-defined
- `p0`: release-blocking, security-sensitive, data-loss, or core workflow outage
- `p1`: high-impact bug or important near-term work
- `p2`: normal roadmap work or a bug with a reasonable workaround or limited blast radius
- `p3`: longer-horizon, convenience, exploratory, or low-urgency work
- `easy`: small, low-risk, localized change
- `medium`: moderate cross-file change or some product/UX/design work
- `hard`: large, high-risk, or architecture-heavy change

External contributors may not have permission to label issues directly. Maintainers and
agents should add any missing labels during first triage, and new issues should not be left
without a type, priority, and difficulty label set.

## Security

If you find a security issue, please report it responsibly by opening a private issue
or contacting the maintainer directly.

## Wall of Apps

Wall content lives in `docs/wall-of-apps.json`.

To add your app:

1. Authenticate GitHub CLI once:
   `gh auth login`
2. Run:
   `asc apps wall submit --app "1234567890" --confirm`
3. Optional preview:
   `asc apps wall submit --app "1234567890" --dry-run`

The command uses your authenticated `gh` session to fork the repo, create a branch, update `docs/wall-of-apps.json`, and open a pull request automatically.
It resolves the public App Store name, URL, and icon from the app ID automatically. For entries that are not on the public App Store yet, use `--link` with `--name`.
When `docs/wall-of-apps.json` is the only changed file, the local hook and PR/main CI use the Wall-specific fast path and only run `make check-wall-of-apps`.

Format:
```json
{
  "app": "Your App Name",
  "link": "https://apps.apple.com/app/id1234567890"
}
```
