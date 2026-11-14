# User invitation reads expose their query parameters as flags

Added in 7.0.0. `asc users invites list` and `asc users invites view` accept
a flag for every query parameter App Store Connect defines for
`GET /v1/userInvitations` and `GET /v1/userInvitations/{id}`. Help lists them
under a new `QUERY FLAGS` heading, and each help line names the parameter the
flag sends:

| Flag | Parameter | Commands |
| --- | --- | --- |
| `--email` | `filter[email]` | list |
| `--role` | `filter[roles]` | list |
| `--visible-app` | `filter[visibleApps]` | list |
| `--sort` | `sort` | list |
| `--fields` | `fields[userInvitations]` | list, view |
| `--app-fields` | `fields[apps]` | list, view |
| `--include` | `include` | list, view |
| `--visible-apps-limit` | `limit[visibleApps]` | list, view |

```bash
asc users invites list --role DEVELOPER --sort -lastName
asc users invites view --id "INVITE_ID" --include visibleApps --app-fields name
```

The names follow the rule the CLI already uses for these parameters, so they
match `asc users list`: `filter[x]` is `--x` with the last word singular,
`fields[x]` is `--fields` for the listed type and `--<x singular>-fields`
otherwise, `limit[x]` is `--x-limit`.

Values are checked before any request: a value outside the parameter's
allowed values exits with code `2` and lists them, `--visible-apps-limit`
must be between 1 and 50 and needs `--include visibleApps`, and
`users invites list --next` cannot be combined with these flags because a
`links.next` URL already carries its query. Enumerated values match without
regard to case (`--role developer` sends `DEVELOPER`).

Invocations without the new flags send the same requests as before.
