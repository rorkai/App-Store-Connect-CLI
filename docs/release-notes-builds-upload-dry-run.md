# Build upload reservation and dry-run transition

7.4.0 adds `--reserve-only` to create a build upload in App Store Connect
(`POST /v1/buildUploads` and `POST /v1/buildUploadFiles`) and print its upload
operations without sending the file. This is the behavior of `--dry-run`
through 7.4.0.

```bash
asc builds upload --app "123456789" --ipa "app.ipa" --reserve-only
```

`--dry-run` emits the following warning on stderr before the first request.

```text
Warning: builds upload --dry-run reserves an upload in App Store Connect (POST /v1/buildUploads and /v1/buildUploadFiles). In a future release, --dry-run will check the artifact locally without contacting App Store Connect. Use --reserve-only to keep reserving uploads.
```

stdout and exit codes are unchanged. `--dry-run` and `--reserve-only` together
exit with usage code `2`, and `--reserve-only` rejects `--concurrency`,
`--checksum`, `--wait`, `--test-notes` and `--verify-timeout` the way
`--dry-run` does.

## Migration

- If a script uses `--dry-run` to get an upload ID or upload operations, switch
  to `--reserve-only` now. Its output is the same.
- If a script uses `--dry-run` as a preflight check, it still creates an
  upload reservation in 7.4.0. A future release will make the check local.
