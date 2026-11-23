# `asc users invite` takes `--confirm`

Changed in 7.3.1. `asc users invite` sends an invitation that gives the person
a role on your App Store Connect team once they accept it. Other commands that
grant access, such as `asc users update --visible-app` and
`asc web apps distribution users create`, already require `--confirm`.
`asc users invite` now accepts `--confirm` too:

```bash
asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" \
  --roles "DEVELOPER" --visible-app "APP_ID" --confirm
```

In this release `--confirm` is optional. Without it, the invitation is still
sent, and stderr gets one line before the request:

```text
Warning: users invite sends an invitation to join your App Store Connect team and will require --confirm in a future release; pass --confirm to acknowledge.
```

A future release will require `--confirm`: an invite without it will print
`Error: --confirm is required`, exit with usage code `2`, and send nothing.
Add `--confirm` to scripts that invite users now. Stdout, the JSON output and
exit codes are unchanged.

## TestFlight hint

When `asc testflight testers add` cannot put an email outside your team into an
internal group, its hint no longer prints a ready-to-run
`asc users invite ... --roles DEVELOPER` command. It says that internal groups
take only App Store Connect team users who can see the app, points to an
external group, and says that inviting the person to the team is a separate
decision for the Account Holder or an Admin.
