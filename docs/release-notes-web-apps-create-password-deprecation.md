# `asc web apps create --password` is deprecated

`asc web apps create --password` now prints a deprecation warning on stderr.
The flag still works during the compatibility window and will be removed in
the next major release.

```text
Warning: `asc web apps create --password` is deprecated and will be removed in the next major release. Run `asc web auth login --apple-id EMAIL` once and rerun without it to reuse the cached web session; unattended runs can set ASC_WEB_PASSWORD instead.
```

Migrate to the cached web session flow. Sign in once in a terminal, then run
`asc web apps create` without a password:

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
