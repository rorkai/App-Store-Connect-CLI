# Usage errors are printed once

Changed in 7.0.0. A usage error prints one `Error:` line, then the command's
usage page, and exits with code `2`. Five commands printed the `Error:` line
twice for some mistakes, and `asc web xcode-cloud settings version-aliases
update` also printed two different `Error:` lines for one mistake. Each now
prints the line once:

```text
$ asc web xcode-cloud settings next-build-number set --product-id PRODUCT_ID --value 0
Error: --value must be greater than 0
DESCRIPTION
...
```

| Command | Mistake |
| --- | --- |
| `asc profiles create` | `--name` longer than 64 characters |
| `asc signing fetch` | `--create-missing-certificate` without `--create-missing`, or with `--match-extensions` |
| `asc web xcode-cloud settings next-build-number set` | `--value` of 0 or less |
| `asc web xcode-cloud settings version-aliases create` | `--type` missing or not `macos_version` or `xcode_version` |
| `asc web xcode-cloud settings version-aliases update` | `--type` empty or not `macos_version` or `xcode_version` |
| `asc web xcode-cloud settings version-aliases update` | none of `--name`, `--type`, `--build` or `--locked` given |

For the last row, the second line, `Error: at least one version alias field is
required`, is gone; the first line, `Error: at least one of --name, --type,
--build, or --locked is required`, stays.

The first line of stderr, the usage page and the exit code are unchanged.
