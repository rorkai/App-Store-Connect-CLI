# `asc web apps create --password` removed

Removed in 6.0.0. `asc web apps create` no longer accepts `--password`. The
flag was deprecated with a stderr warning before this release. Passing it now
fails with usage exit code `2` before any web session lookup or app creation:

```text
Error: `--password` was removed in 6.0.0; run `asc web auth login --apple-id EMAIL` once and rerun without it to reuse the cached web session, or set `ASC_WEB_PASSWORD` for unattended sign-in
```

## Migration

Sign in once in a terminal, then create apps from the cached web session:

```bash
asc web auth login --apple-id "user@example.com"
asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123"
```

For unattended runs where no session is cached, provide the password through
the `ASC_WEB_PASSWORD` environment variable and the two-factor code through
`--two-factor-code-command` or `ASC_WEB_2FA_CODE_COMMAND`. A session exported
from another machine can also be loaded with
`asc web auth import --file FILE`.

The interactive password prompt and the environment variable are unchanged.
