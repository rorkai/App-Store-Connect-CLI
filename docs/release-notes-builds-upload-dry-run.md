# `asc builds upload --reserve-only`; `--dry-run` stops reserving in 7.4.0

Changed in 7.3.2. `asc builds upload --dry-run` has always created a build
upload in App Store Connect (`POST /v1/buildUploads` and
`POST /v1/buildUploadFiles`) and printed its upload operations without sending
the file. That is a write to your account, not a dry run.

7.3.2 adds `--reserve-only`, which does exactly what `--dry-run` did, with the
same output and exit codes:

```bash
asc builds upload --app "123456789" --ipa "app.ipa" --reserve-only
```

`--dry-run` still reserves the upload in 7.3.2, and prints this warning on
stderr before the first request:

```text
Warning: builds upload --dry-run reserves an upload in App Store Connect (POST /v1/buildUploads and /v1/buildUploadFiles). From 7.4.0, --dry-run sends nothing to App Store Connect and only checks the artifact locally; use --reserve-only to keep reserving the upload.
```

stdout and exit codes are unchanged. `--dry-run` and `--reserve-only` together
exit with usage code `2`, and `--reserve-only` rejects `--concurrency`,
`--checksum`, `--wait`, `--test-notes` and `--verify-timeout` the way
`--dry-run` does.

## What changes in 7.4.0

From 7.4.0, `asc builds upload --dry-run` sends nothing to App Store Connect. It
checks the artifact, version, build number and credentials locally and prints
what it would upload, without `uploadId`, `fileId` or `operations`.

## Migration

- If a script uses `--dry-run` to get an upload ID or upload operations, switch
  to `--reserve-only` now. Its output is the same.
- If a script uses `--dry-run` as a preflight check, keep it. From 7.4.0 it no
  longer creates a build upload in your account.
