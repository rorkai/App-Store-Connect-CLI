# Build upload reservation and the dry-run warning

7.4.0 adds `--reserve-only` to create a build upload in App Store Connect
(`POST /v1/buildUploads` and `POST /v1/buildUploadFiles`) and print its upload
operations without sending the file. `--dry-run` has always done this, and
still does.

```bash
asc builds upload --app "123456789" --ipa "app.ipa" --reserve-only
```

`--dry-run` emits the following warning on stderr before the first request.

```text
Warning: builds upload --dry-run reserves an upload in App Store Connect (POST /v1/buildUploads and /v1/buildUploadFiles) without sending the file. Use --reserve-only to reserve an upload without this warning.
```

stdout and exit codes are unchanged. `--dry-run` and `--reserve-only` together
exit with usage code `2`, and `--reserve-only` rejects `--concurrency`,
`--checksum`, `--wait`, `--test-notes` and `--verify-timeout` the way
`--dry-run` does.

## Migration

- If a script uses `--dry-run` to get an upload ID or upload operations, switch
  to `--reserve-only`. Its output is the same and it prints no warning.
- If a script uses `--dry-run` as a preflight check, it creates an upload
  reservation in App Store Connect each time. The reservation carries the app,
  platform, version, build number, file name and file size, never the file.
