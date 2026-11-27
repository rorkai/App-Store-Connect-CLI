# Message body files must be inside the working directory

Changed after 7.6.1. `asc web review reply --body-file`,
`asc web review drafts create --body-file`, `asc web review drafts update
--body-file`, `asc notify slack --blocks-file`, `asc notify slack
--payload-file` and `asc reviews respond-batch --file` now read only a regular
file inside the working directory. Each of these sends the file's content to a
recipient outside the machine: App Review, Slack, or App Store customers.

A path is refused with a usage error (exit code `2`) before anything is sent,
and the command makes no request:

- an absolute path outside the working directory;
- a relative path that goes up with `..` out of it;
- a symlink at any component below the working directory;
- a directory, a FIFO or another non-regular file.

An absolute path inside the working directory, and a relative path, still work.
Size, UTF-8 and character-length limits are unchanged. An oversize
`--body-file` is now a usage error, so it also exits with `2`.

```text
$ asc web review reply --thread-id THREAD_ID --body-file ~/.ssh/config --confirm
Error: --body-file must be a regular file inside the working directory; use --message, or copy the file into the project directory
```

To send a body from elsewhere, copy the file into the project directory, or pass
the text with `--message` (or `--blocks-json` and `--payload-json` for Slack).
`--message` and inline JSON are not changed.
