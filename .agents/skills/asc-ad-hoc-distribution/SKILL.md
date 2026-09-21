---
name: asc-ad-hoc-distribution
description: Prepare, publish, resume, and verify private iOS release-testing installs with asc distribute. Use when distributing an IPA to registered devices outside TestFlight, reconciling ad hoc profiles, publishing through caller-owned S3-compatible storage, or diagnosing a resumable private distribution run.
---

# ASC ad hoc distribution

Use the experimental `asc distribute` workflow to turn an existing iOS archive
into a private, verified install link for registered devices. Use TestFlight or
the App Store release skills instead when the build should go through Apple-hosted
distribution.

Confirm the installed contract before acting:

```bash
asc distribute --help
asc distribute plan --help
asc distribute apply --help
asc distribute resume --help
asc distribute status --help
asc distribute verify --help
```

## Choose the workflow

- Use `plan` -> `apply` -> `resume`/`status` -> `verify` for an end-to-end,
  hash-authorized run starting from an `.xcarchive`.
- Use `inspect` -> `prepare` -> `publish` only when the caller already owns the
  ad hoc IPA and wants to operate the lower-level boundaries separately.

## Guardrails

- Treat the distribution spec, devices file, PKCS#12 identity, password file,
  run state, and exact install-link artifact as private. Keep them out of Git and
  require owner-only permissions.
- Never put S3 credentials or presigned URLs in the distribution spec, command
  output, logs, issues, or chat. Use `ASC_S3_ACCESS_KEY_ID`,
  `ASC_S3_SECRET_ACCESS_KEY`, and optional `ASC_S3_SESSION_TOKEN`, or the
  standard AWS SDK credential chain.
- `plan` is read-only and may exit successfully with `ready: false`. Inspect the
  typed blockers and effects before continuing.
- `apply` can register missing devices, create safe App IDs and successor ad hoc
  profiles, write local artifacts, and publish immutable objects. Run it only
  after the user authorizes the exact plan hash and effect inventory.
- The orchestrated workflow supports private access to an existing
  S3-compatible bucket. It does not create buckets, change policies, delete old
  builds, install the app, or launch it.
- Input drift, an immutable-object conflict, expired signing material, or an
  expired private link requires a new plan. Do not force the old run forward.

## Preconditions

- An existing iOS `.xcarchive` with one main app target. Embedded apps,
  extensions, Watch apps, and App Clips make the v1 plan not ready.
- A local iOS distribution PKCS#12 identity and optional protected password file.
- A protected strict-v1 devices file. For example:

```json
{"schemaVersion":1,"devices":[{"name":"Test iPhone","udid":"DEVICE_UDID","platform":"IOS"}]}
```

- App Store Connect authentication with access to devices, Bundle IDs,
  certificates, and profiles.
- An existing S3-compatible bucket and valid credentials.

## 1. Create the private distribution spec

Relative paths resolve from the spec directory. The spec does not interpolate
environment variables or accept credentials. A representative private config is:

```json
{
  "schemaVersion": 1,
  "devicesFile": "devices.json",
  "signing": {
    "identity": {
      "format": "pkcs12",
      "path": "../signing/distribution.p12",
      "passwordFile": "../secrets/distribution-p12-password"
    },
    "minimumValidityDays": 7,
    "maxMutations": 32
  },
  "publication": {
    "endpoint": "https://objects.example.com",
    "downloadEndpoint": "https://downloads.example.com",
    "region": "auto",
    "bucket": "ios-builds",
    "prefix": "team/app",
    "addressingStyle": "path",
    "urlTtl": "24h",
    "downloadGrace": "1h",
    "verifyTimeout": "30s"
  },
  "metadata": {
    "title": "App",
    "channel": "pull-request-42",
    "sourceRevision": "abc123",
    "sourceUrl": "https://example.com/team/app/commit/abc123"
  }
}
```

`passwordFile`, `certificateSha256`, `downloadEndpoint`, and every metadata field
are optional. An omitted password file means the PKCS#12 must use an empty
password. Protect the config and secret inputs before planning:

```bash
chmod 600 ".asc/distribution/config.json" ".asc/distribution/devices.json"
chmod 600 ".asc/signing/distribution.p12" ".asc/secrets/distribution-p12-password"
```

## 2. Plan without mutation

```bash
asc distribute plan \
  --archive-path ".asc/artifacts/App.xcarchive" \
  --config ".asc/distribution/config.json" \
  --plan ".asc/distribution/plan.json" \
  --state-dir ".asc/distribution/runs" \
  --output json
```

Inspect `ready`, `planHash`, signing validity, destination, and the complete
ordered `effects` inventory. Resolve blockers and create a new plan when
`ready` is false. Do not infer readiness from exit code alone.

## 3. Apply the exact authorized plan

After approval of the exact effects, pass the full 64-character hash:

```bash
PLAN_HASH="$(jq -er '.planHash' ".asc/distribution/plan.json")"
asc distribute apply \
  --plan ".asc/distribution/plan.json" \
  --confirm "$PLAN_HASH" \
  --output json
```

Missing, malformed, or unequal confirmation is rejected before side effects.
Success means publication and live fetch verification completed; it does not
mean a device installed or launched the app.

## 4. Inspect, resume, and verify

Use the returned `runId`:

```bash
asc distribute status --run "RUN_ID" --state-dir ".asc/distribution/runs" --output json
asc distribute resume --run "RUN_ID" --state-dir ".asc/distribution/runs" --output json
asc distribute verify --run "RUN_ID" --state-dir ".asc/distribution/runs" --timeout 30s --output json
```

`status` is local-only and succeeds for `running`, `recoverable`, and `blocked`
runs; branch on typed fields rather than prose. `resume` revalidates durable
evidence before retrying and never blindly repeats a remote write. `verify` is
read-only but performs live fetches. Add `--device "DEVICE_SELECTOR"` only when
the user asks to observe the matching installed bundle, version, and build on a
connected device; this observation does not prove IPA byte identity.

The exact private install URL is a bearer credential stored only in the
owner-private link artifact reported by the completed run. Share it only with
the intended tester through an approved private channel.

## Lower-level IPA workflow

Use this lane when signing and export are already complete. Pass the exact
`bundleDir` returned by `prepare` to `publish`:

```bash
asc distribute inspect --ipa ".asc/artifacts/App.ipa" --output json
asc distribute prepare --ipa ".asc/artifacts/App.ipa" --channel "pull-request-42" --output json
asc distribute publish \
  --bundle-dir ".asc/distribution/com.example.app/1.2-42-IPA_SHA_PREFIX" \
  --endpoint "https://objects.example.com" \
  --region "auto" \
  --bucket "ios-builds" \
  --prefix "team/app" \
  --receipt ".asc/publishes/app-1.2-42.json" \
  --link-path ".asc/publishes/app-1.2-42-link.json" \
  --output json
```

`inspect` omits raw device UDIDs unless `--include-devices` is explicitly
needed. `prepare` never overwrites a bundle and reuses only an exact equivalent.
Private `publish` is the default and writes exact presigned links only to the
mode-0600 link artifact. Public publication is a separate explicit lane using
`--access public --public-base-url`; it assumes anonymous reads are already
configured and never changes storage policy.
