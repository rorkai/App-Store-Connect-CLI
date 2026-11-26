# `asc users invite` requires `--confirm`

Changed after 7.6.0. `asc users invite` sends an invitation that gives the
person a role on your App Store Connect team once they accept it. It now
requires `--confirm`, like the other commands that grant access. Without it the
command sends nothing:

```text
$ asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" --roles "DEVELOPER" --all-apps
Error: --confirm is required: users invite invites the person to join your App Store Connect team
...usage page...
$ echo $?
2
```

Exit code `2` is the usage code. Stdout is empty, and no request reaches Apple.
The check runs after the other required flags (`--email`, `--first-name`,
`--last-name`, `--roles`, and `--all-apps` or `--visible-app`), so a missing
flag is still reported first.

Scripts that invite users must add `--confirm`:

```bash
asc users invite --email "user@example.com" --first-name "Jane" --last-name "Doe" \
  --roles "DEVELOPER" --visible-app "APP_ID" --confirm
```

If a script parses stderr, match `Error: --confirm is required` at the start of
the line. The warning from 7.3.1 (`Warning: users invite sends an invitation ...
will require --confirm in a future release`) is gone. For a successful invite,
stdout, the JSON output and the exit codes are unchanged.

## TestFlight hint (7.3.1)

When `asc testflight testers add` cannot put an email outside your team into an
internal group, its hint no longer prints a ready-to-run
`asc users invite ... --roles DEVELOPER` command. It says that internal groups
take only App Store Connect team users who can see the app, points to an
external group, and says that inviting the person to the team is a separate
decision for the Account Holder or an Admin.
