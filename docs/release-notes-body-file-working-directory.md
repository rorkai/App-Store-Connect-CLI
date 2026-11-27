# Message body file locations: deprecation transition

Changed after 7.6.1. These inputs continue to accept previously supported
regular files outside the working directory, with a deprecation warning on
stderr:

- `asc web review reply --body-file`;
- `asc web review drafts create --body-file` and `drafts update --body-file`;
- `asc notify slack --blocks-file` and `--payload-file`;
- `asc reviews respond-batch --file`.

Slack and batch review replies also preserve their existing support for
symlink inputs that resolve to regular files, with the same warning. Web review
commands retain their existing symlink restrictions. All inputs reject
directories, FIFOs, and other non-regular files before reading their content.

Copy deprecated input files into the project directory as regular files before
a future release refuses those paths. Web review also accepts inline text with
`--message`; Slack accepts `--blocks-json` and `--payload-json`. Inline input is
unchanged. Ordinary relative and absolute paths inside the working directory
remain accepted without a warning, including Windows drive-relative paths and
macOS aliases for the working directory.

For example, a previously supported temporary reply file still works and emits:

```text
Warning: --body-file outside the working directory or through a symlink is deprecated and will be refused in a future release; use --message, or copy the file into the project directory
```

Size, UTF-8 and character-length limits are unchanged. An oversize `--body-file`
is now a usage error (exit code `2`). Missing or unreadable web body files retain
their existing read-error classification (exit code `1`).
