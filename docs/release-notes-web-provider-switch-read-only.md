# `asc web` no longer switches providers under read-only mode or `--dry-run`

Changed in 7.4.0. When `--provider-id` or `--public-provider-id` names a
provider other than the one the Apple web session is on, `asc web` switches the
session with `POST /olympus/v1/session` and caches it on the new provider. Every
later `asc web` command that uses the cache then runs on that provider.

That switch is now refused under read-only mode (`--read-only` or
`ASC_READ_ONLY`) and under a web command's `--dry-run`. The command reads the
session, sends no switch request, keeps the session cached on its current
provider, and stops:

```text
$ ASC_READ_ONLY=1 asc web apps declarations list --app APP_ID --provider-id 222222
Error: ASC_READ_ONLY is set; refusing to switch the web session from provider 111111 / AAAA111111 / Team A to provider 222222 / BBBB222222 / Team B
Hint: The session stays on provider 111111 / AAAA111111 / Team A. To switch, run 'asc web auth login --provider-id 222222' with neither --read-only nor ASC_READ_ONLY set; later commands then run on provider 222222.
```

| Case | Exit code |
|---|---|
| read-only mode, other provider | `6`, like every read-only refusal |
| `--dry-run`, other provider | `2` (usage), without the usage page; the hint says to switch first with `asc web auth login --provider-id ID` or to run without `--dry-run` |
| the provider the session is already on | unchanged: no request, the command runs |

Unchanged:

- Signing in (password, two-factor code, trusting the browser, resuming a
  cached session) still works under read-only mode and `--dry-run`.
- After an automatic re-sign-in, the session is still returned to the provider
  its expired cached session was on. That restores the session's provider rather
  than choosing a new one.
- Brokered web sessions keep their own refusal.

## Migration

Switch providers once, outside read-only mode and without `--dry-run`:

```bash
asc web auth login --provider-id 222222
```

Later read-only and dry-run commands then run on that provider with no switch.
A read-only job that reads several teams needs a step that switches providers
without read-only mode before each team's reads.
