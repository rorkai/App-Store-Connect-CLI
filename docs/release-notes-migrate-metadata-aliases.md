# Deprecated `asc migrate metadata` aliases

`asc migrate metadata pull`, `asc migrate metadata push`, and
`asc migrate metadata validate` are deprecated and will be removed in 6.0.0.
They still forward to `asc metadata pull`, `asc metadata push`, and
`asc metadata validate` with identical flags, stdout, and exit codes, and now
print one warning line to stderr first:

```text
Warning: `asc migrate metadata pull` is deprecated and will be removed in 6.0.0. Use `asc metadata pull`.
```

Help marks the group and each alias as `DEPRECATED:`, and `asc migrate --help`,
shell completion, and `asc search` no longer list them.

## Migration

Replace the `migrate metadata` prefix with `metadata`; flags do not change:

```bash
# Before
asc migrate metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
asc migrate metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
asc migrate metadata validate --dir "./metadata"

# After
asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
asc metadata validate --dir "./metadata"
```

`asc migrate import`, `asc migrate export`, and `asc migrate validate` are not
affected.
