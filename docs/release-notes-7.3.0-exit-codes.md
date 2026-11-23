# Exit-code and error-output changes in 7.3.0

Changed in 7.3.0. Scripts that branch on these exit codes need a look.

| Command and case | 7.2.0 | 7.3.0 |
|---|---|---|
| `asc builds beta-app-review-submission view` for a build with no beta review submission | `4` (not found) | `0`, prints Apple's empty envelope and a note on stderr |
| `asc subscriptions offers introductory create` when some territories fail | `1` | the exit code of the first failure (for example `5` for a conflict) |
| `asc auth status` when the keychain can't be read | `1` | `3` (auth) |

Error output changes, exit code unchanged:

- `asc auth login` (and `asc ads auth login`, `asc storekit auth login`) with a
  refused private key file or `--key-json` file prints the error and its fix
  without the command's usage page after it. The exit code stays `2`.
