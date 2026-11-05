# CI/CD Integrations

## GitHub Actions

Install `asc` using the official setup action:

```yaml
- uses: rudrankriyam/setup-asc@v1
  with:
    version: latest

- run: asc --help
```

For end-to-end examples, see:
https://github.com/rudrankriyam/setup-asc

## GitLab CI/CD Components

Use the official `asc-ci-components` repository:

```yaml
include:
  - component: gitlab.com/rudrankriyam/asc-ci-components/run@main
    inputs:
      stage: deploy
      job_prefix: release
      asc_version: latest
      command: asc --help
```

For install/run templates and self-managed examples:
https://github.com/rudrankriyam/asc-ci-components

## Bitrise

Use the official `setup-asc` Bitrise step repository:

```yaml
workflows:
  primary:
    steps:
    - git::https://github.com/rudrankriyam/steps-setup-asc.git@main:
        inputs:
        - mode: run
        - version: latest
        - command: asc --help
```

## CircleCI

Use the official CircleCI orb repository:
https://github.com/rudrankriyam/asc-orb

## Code signing on a fresh runner

A fresh macOS runner can go from an App Store Connect API key to a signed
archive without hand-written `security` calls. Provide `ASC_KEY_ID`,
`ASC_ISSUER_ID`, and `ASC_PRIVATE_KEY_B64` from the CI secret store, then run
these commands in one shell step:

1. First run only, when the team has no active certificate of the required
   type: `asc signing fetch --create-missing --create-missing-certificate`
   creates the private key, CSR, certificate, profile, and a
   password-protected `.p12`. Push that identity to the encrypted store
   (`asc signing sync push` with `--identity`) before building, because the
   private key exists only on this runner.
2. Every later run: `asc signing sync pull` decrypts the stored identity and
   profiles.
3. `asc signing keychain install --confirm` imports the identity into a new
   dedicated keychain, and `asc profiles local install` installs each profile.
4. `asc xcode signing plan` with `--profile` infers each target's signing
   settings and an `ExportOptions.plist` from the profiles, and
   `asc xcode signing apply --confirm` writes those settings into the project.
5. `asc xcode archive`, then `asc xcode export` with `--export-options`,
   produce the signed archive and IPA.
6. `asc signing keychain delete --confirm` removes the job keychain.

The first-run commands look like this. The full script, including the
password files, cleanup trap, and later-run variant, is in the
[code signing guide](../website/src/content/docs/guides/code-signing.mdx) and the
[signing command reference](../website/src/content/docs/commands/signing.mdx).

```bash
asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --create-missing --create-missing-certificate --identity-password-file "$p12_password_file" --output .asc/signing/bootstrap --format json > .asc/signing/fetch-result.json
asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE --repo git@github.com:team/signing.git --password-file "$sync_password_file" --identity "$p12_path" --identity-password-file "$p12_password_file" --output json
asc signing keychain install --identity "$p12_path" --identity-password-file "$p12_password_file" --keychain "$keychain_path" --keychain-password-file "$keychain_password_file" --add-to-search-list --confirm --output json
asc profiles local install --path "$profile_path" --output json
asc xcode signing plan --project Example.xcodeproj --profile "$profile_path" --configuration Release --export-options-out .asc/artifacts/ExportOptions.plist --output json
asc xcode signing apply --plan .asc/xcode/signing/plan.json --confirm --output json
asc xcode archive --project Example.xcodeproj --scheme Example --configuration Release --archive-path .asc/artifacts/Example.xcarchive
asc xcode export --archive-path .asc/artifacts/Example.xcarchive --export-options .asc/artifacts/ExportOptions.plist --ipa-path .asc/artifacts/Example.ipa
asc signing keychain delete --keychain "$keychain_path" --confirm
```

`p12_path` and `profile_path` come from the `p12Path` and `profileFile` fields
of the fetch receipt. `xcode signing plan` exits 0 with `ready: false` when a
target has no matching profile; check `ready` before applying.

`asc signing fetch --delete-stale-profiles` removes expired or invalid
profiles, and `asc certificates` deactivates or revokes certificates. With Git
storage only, `asc signing sync push --renew-expired` replaces an expired synced
profile, `--force-for-new-devices` (optionally with `--include-mac-in-profiles`)
recreates a development or ad hoc profile when its devices change, and
`asc signing sync nuke` removes one profile type with its certificates and
encrypted files.

### Signing assets in object storage with workload identity

`asc signing sync push`, `pull`, and `rotate-password` can keep the encrypted
artifacts in an S3 bucket instead of a Git repository with `--storage object`
and `--object-bucket`, plus optional `--object-prefix`, `--object-region`, and
`--object-endpoint`. Objects use the Git tree's layout, so a bucket prefix and
a signing repository can be mirrored byte for byte. No flag accepts a cloud
credential: credentials and, unless `--object-region` is set, the region come
from the standard AWS configuration chain (environment variables, shared
config or SSO profiles, web identity, or container and instance metadata).

In GitHub Actions, OpenID Connect lets the job assume an AWS role and receive
short-lived credentials, so no AWS access key is stored in the repository or
passed to `asc`:

```yaml
permissions:
  id-token: write
  contents: read

jobs:
  build:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4

      - uses: rudrankriyam/setup-asc@v1
        with:
          version: latest

      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::123456789012:role/asc-signing-read
          aws-region: us-east-1

      - name: Pull signing assets
        env:
          ASC_SIGNING_SYNC_PASSWORD: ${{ secrets.ASC_SIGNING_SYNC_PASSWORD }}
        run: |
          asc signing sync pull --storage object --object-bucket team-certs --object-prefix asc/ --output-dir .asc/signing/pulled --output json
```

`configure-aws-credentials` exchanges the job's OIDC token for temporary role
credentials and exports them, with `AWS_REGION`, to later steps. The role's
trust policy should accept only this repository (and, ideally, a protected
branch or environment), and a pull-only role needs just `s3:ListBucket` and
`s3:GetObject` on the prefix. A job that runs `sync push` or
`sync rotate-password` also needs `s3:PutObject`, and rotation needs
`s3:DeleteObject` to remove its staged objects. The repository encryption
password still comes from the CI secret store, never from a command argument:
push and pull read `ASC_SIGNING_SYNC_PASSWORD` or a protected
`--password-file`, and `rotate-password` requires protected
`--password-file` and `--new-password-file` files and ignores the environment
variable.

Push and rotation use conditional writes, so a push that races another writer
fails with exit code 1 instead of overwriting newer data; pull and retry. For an
S3-compatible service, add `--object-endpoint` with its HTTPS origin; requests
then use path-style addressing, and a service that ignores `If-Match` and
`If-None-Match` loses the conflict protection. Receipts include
`storage.kind` (`object`) and `storage.location` (`s3://<bucket>/<prefix>`).
