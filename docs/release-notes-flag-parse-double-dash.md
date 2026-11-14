# Flag-parse errors name flags with two dashes

Changed in 7.0.0. When a flag value cannot be parsed, or a flag is given
without its value, the error now names the flag the way the rest of the CLI
and its help do, with two dashes:

```text
$ asc apps list --limit abc
Error: invalid value "abc" for flag --limit: parse error
For help:
  asc apps list --help
```

| Mistake | Before | After |
| --- | --- | --- |
| Value that does not parse | `invalid value "abc" for flag -limit: parse error` | `invalid value "abc" for flag --limit: parse error` |
| Boolean value that does not parse | `invalid boolean value "x" for -paginate: parse error` | `invalid boolean value "x" for --paginate: parse error` |
| Flag without its value | `flag needs an argument: -limit` | `flag needs an argument: --limit` |
| Unknown flag after a positional argument (`asc schema`, `asc api`) | `flag provided but not defined: -x` | `flag provided but not defined: --x` |
| `--app` given a self-link of another type (`asc app-setup info set`, `asc app-setup categories set`, `asc categories set`) | `invalid value "…" for flag -app: …` | `invalid value "…" for flag --app: …` |

This applies to every command and to the root flags. Only the dash before
the flag name changes. The value is printed as before, even when it contains
text such as `-x`, and so is the reason after the flag name. The exit code
stays `2`, and the `For help:` lines are unchanged.

A script that matched the old text, such as `for flag -limit`, should match
`for flag --limit`, or check for exit code `2`.
