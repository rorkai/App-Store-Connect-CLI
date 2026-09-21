---
name: asc-analytics-reports
description: Collect, download, and verify App Store Connect Analytics reports with asc. Use when users need to discover analytics report requests, select report instances by processing date or granularity, download every segment, or verify downloaded files against Apple's size and MD5 metadata before analysis.
---

# asc analytics reports

Collect a complete set of analytics report segments and verify the compressed
files before handing them to a separate analysis workflow. Do not interpret,
aggregate, or present the report contents as part of this skill.

## Guardrails

- Treat report inventories, signed URLs, downloaded segments, and identifiers as
  confidential business data.
- Prefer an existing request. Creating a request changes App Store Connect state
  and requires an Admin-authorized profile; obtain explicit user approval before
  running `asc analytics request`.
- Never delete or replace a request as part of collection.
- Use `asc analytics download`; do not fetch or persist signed segment URLs
  separately.
- Store private files outside the source repository with owner-only permissions.
- Do not claim the collection is complete if any expected segment is missing or
  fails verification.

## 1. Verify the CLI contract

Inspect the installed command before authentication or collection:

```bash
asc analytics view --help
```

Continue only when help lists `--processing-date`, `--granularity`,
`--paginate`, and `--include-segments`. These filters require asc 3.5.0 or
newer. If either filter is absent, ask the user to upgrade. Do not substitute
the deprecated `--date` flag because it uses legacy local matching rather than
Apple's server-side processing-date filter.

## 2. Prepare private storage

Create a private temporary directory outside the repository before capturing
JSON or downloading segments:

```bash
umask 077
ASC_ANALYTICS_DIR="$(mktemp -d "${TMPDIR:-/tmp}/asc-analytics.XXXXXX")"
mkdir -m 700 "$ASC_ANALYTICS_DIR/segments"
```

Redirect command output and errors into this directory. Do not paste raw
inventory JSON or error output into chat, commits, issues, or pull requests.

## 3. Find an existing request

Resolve the app ID and selected asc profile, then list every request:

```bash
asc --profile "$PROFILE" analytics requests \
  --app "$APP_ID" \
  --paginate \
  --output json \
  > "$ASC_ANALYTICS_DIR/requests.json" \
  2> "$ASC_ANALYTICS_DIR/requests.stderr"
```

Inspect the JSON structurally and select an existing usable request. Do not rely
on `--state` when discovery works without it. If no usable request exists, stop
and ask whether the user wants to create `ONGOING` or `ONE_TIME_SNAPSHOT`
access. State the target app and profile before requesting approval. After
approval, prefer `--reuse-existing` to avoid duplicates:

```bash
asc --profile "$APPROVED_PROFILE" analytics request \
  --app "$APP_ID" \
  --access-type "$ACCESS_TYPE" \
  --reuse-existing \
  --output json \
  > "$ASC_ANALYTICS_DIR/request.json" \
  2> "$ASC_ANALYTICS_DIR/request.stderr"
```

Do not run that command without explicit approval. Read the returned request ID
from the private JSON response before continuing:

```bash
REQUEST_ID="$(jq -er '.requestId' "$ASC_ANALYTICS_DIR/request.json")"
```

When a request was created or reused with the approved profile, set
`ANALYTICS_PROFILE="$APPROVED_PROFILE"`. For an existing request discovered
with the read-only profile, set `ANALYTICS_PROFILE="$PROFILE"`. Use that same
profile for every subsequent `analytics view` and `analytics download` call.

## 4. Discover and select report instances

First retrieve report and instance metadata without segment URLs:

```bash
asc --profile "$ANALYTICS_PROFILE" analytics view \
  --request-id "$REQUEST_ID" \
  --paginate \
  --output json \
  > "$ASC_ANALYTICS_DIR/discovery.json" \
  2> "$ASC_ANALYTICS_DIR/discovery.stderr"
```

Select an available `processingDate` and the granularity requested by the user.
Accept `DAILY`, `WEEKLY`, and `MONTHLY`, individually or as a comma-separated
list. Split the input on commas, trim each token, and normalize it to uppercase.
Validate every token against that allowlist, including empty tokens. Report the
invalid input and stop before running `analytics view`; never silently discard
unsupported values or continue with an empty filter. After validation, remove
duplicates and join the remaining values with commas.
Treat `processingDate` as the date Apple processed the report, not necessarily
the period represented by its rows.

Retrieve the filtered inventory, including all segment metadata:

```bash
asc --profile "$ANALYTICS_PROFILE" analytics view \
  --request-id "$REQUEST_ID" \
  --processing-date "$PROCESSING_DATE" \
  --granularity "$GRANULARITY" \
  --paginate \
  --include-segments \
  --output json \
  > "$ASC_ANALYTICS_DIR/inventory.json" \
  2> "$ASC_ANALYTICS_DIR/inventory.stderr"
```

Always use `--paginate`; asc follows Apple-provided report and instance next
links. Do not reconstruct, alter, or follow pagination URLs manually.

## 5. Download every segment

Parse `inventory.json` structurally. For every selected instance, enumerate all
segments and retain each segment's exact ID, `sizeInBytes`, and `checksum` for
verification. Do not print `downloadUrl`.

Download each segment by its request, instance, and segment IDs. Use a filename
derived only from the segment ID and keep the compressed bytes intact. Analytics
reports are tab-delimited text, so use `.txt.gz` rather than implying CSV:

```bash
SEGMENT_FILE="$ASC_ANALYTICS_DIR/segments/$SEGMENT_ID.txt.gz"
asc --profile "$ANALYTICS_PROFILE" analytics download \
  --request-id "$REQUEST_ID" \
  --instance-id "$INSTANCE_ID" \
  --segment-id "$SEGMENT_ID" \
  --output "$SEGMENT_FILE" \
  > /dev/null \
  2>> "$ASC_ANALYTICS_DIR/download.stderr"
```

Do not use `--decompress` before verification. If an instance has multiple
segments, download every one; never treat the first segment as the whole report.

## 6. Verify the downloaded files

For each file, compare the compressed byte count with `sizeInBytes` and the
lowercase MD5 digest with `checksum`. Use local system tools such as:

```bash
actual_size="$(wc -c < "$SEGMENT_FILE" | tr -d ' ')"
actual_md5="$(openssl dgst -md5 -r "$SEGMENT_FILE" | awk '{print tolower($1)}')"
expected_md5="$(printf '%s' "$CHECKSUM" | tr '[:upper:]' '[:lower:]')"
```

Require `actual_size` to equal `sizeInBytes` and `actual_md5` to equal
`expected_md5`. On a mismatch, mark that segment failed, keep the raw error
private, and do not claim a complete collection. `asc analytics download`
refuses to overwrite an existing output, so retry the failed segment once to a
new path such as `$ASC_ANALYTICS_DIR/segments/$SEGMENT_ID.retry-1.txt.gz`. Verify
the retry independently and use it only if both checks pass. Keep the original
failed file unless the user approves its deletion. Do not parse or analyze a
file until verification succeeds.

## 7. Report the result

Return a concise summary containing:

- the selected processing date and granularity values;
- counts of reports, instances, expected segments, downloaded segments, and
  verified segments;
- whether the collection is complete;
- failed or missing segment IDs, if any, without signed URLs or report rows;
- the private output directory when appropriate for the current user session.

Do not include analytics values, signed URLs, profile names, credentials, or raw
rows in a public artifact. Keep the verified files for the user's next workflow.
Ask before deleting the temporary directory or any downloaded evidence.
