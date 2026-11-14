# A format name given to a file-path `--output` is rejected

Changed in 7.0.0. In almost every command, `--output` chooses the output
format (`json`, `table`, `markdown`). In the 13 commands below it is the path
the command writes to instead. Given `--output json` there, these commands
wrote a file named `json` and exited `0`. They now stop before doing any work,
print one `Error:` line and the usage page, and exit with usage code `2`:

```text
$ asc profiles download --id PROFILE_ID --output json
Error: --output takes a path in this command, not an output format; use --output-format json to choose the format, or --output ./json to write to a path named json
DESCRIPTION
  Download a provisioning profile.
...
$ echo $?
2
```

The check applies to `json`, `table`, `markdown` and the `md` alias, in any
letter case. The error names the command's own format flag. Two commands have
no flag that chooses `json`, `table` or `markdown`, and their error says so.

| Command | Format flag named in the error |
| --- | --- |
| `asc analytics download` | `--output-format` |
| `asc analytics sales` | `--output-format` |
| `asc finance reports` | `--output-format` |
| `asc performance download` | `--output-format` |
| `asc profiles download` | `--output-format` |
| `asc subscriptions offers offer-codes generate` | `--output-format` |
| `asc screenshots download` | `--format` |
| `asc signing fetch` | `--format` |
| `asc signing resign` | `--format` |
| `asc testflight testers export` | `--format` |
| `asc video-previews download` | `--format` |
| `asc subscriptions offers offer-codes values` | none |
| `asc testflight config export` | none |

To write to a file or directory that really is named `json`, `table` or
`markdown`, give it as a path: `--output ./json`. Every other `--output` value
behaves as before.
