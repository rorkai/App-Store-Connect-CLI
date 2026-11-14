# Invalid flag values exit with usage code 2 in every command

Changed in 7.0.0. A flag value that a command rejects before it does any work
is a usage error: the command prints one `Error:` line, then its usage page,
and exits with code `2`. Twenty commands printed the `Error:` line without the
usage page and exited with code `1` instead. They now behave like every other
command:

```text
$ asc merchant-ids list --fields bogus
Error: merchant-ids list: --fields must be one of: name, identifier, certificates
DESCRIPTION
  List merchant IDs.
...
$ echo $?
2
```

The `Error:` line is unchanged. Only the usage page after it and the exit code
are new.

| Command | Flag values that now exit 2 |
| --- | --- |
| `asc app-clips advanced-experiences create` | `--default-language`, `--action`, `--category` |
| `asc app-clips advanced-experiences list` | `--action` |
| `asc app-clips advanced-experiences update` | `--action`, `--category`, `--default-language`, including an empty value |
| `asc app-setup localizations upload` | `--type` |
| `asc apps search-keywords list` | `--platform` |
| `asc background-assets list` | `--archived` |
| `asc background-assets update` | `--archived` |
| `asc certificates export` | `--p12-out` equal to an input path |
| `asc merchant-ids certificates list` | `--fields`, `--include`, `--pass-type-fields` |
| `asc merchant-ids list` | `--fields`, `--certificate-fields`, `--include` |
| `asc merchant-ids view` | `--fields`, `--certificate-fields`, `--include` |
| `asc pass-type-ids certificates list` | `--fields` |
| `asc pass-type-ids list` | `--fields`, `--certificate-fields`, `--include` |
| `asc pass-type-ids view` | `--fields`, `--certificate-fields`, `--include` |
| `asc pre-orders enable` | `--release-date` |
| `asc pre-orders update` | `--release-date` |
| `asc subscriptions offers win-back list` | `--fields`, `--price-fields`, `--include` |
| `asc subscriptions offers win-back prices` | `--fields`, `--territory-fields`, `--price-point-fields`, `--include` |
| `asc versions create` | `--platform` |
| `asc xcode export-options generate` | `--archive-path` that does not end with `.xcarchive` |

A script that treated exit code `1` from one of these commands as "the value
was rejected" should check for `2`, the code every other command already
returns for the same mistake. Errors that depend on files, such as a missing
input file, keep exit code `1`.
