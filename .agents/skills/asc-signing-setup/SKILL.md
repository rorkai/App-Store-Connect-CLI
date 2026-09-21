---
name: asc-signing-setup
description: Set up bundle IDs, capabilities, signing certificates, provisioning profiles, and encrypted signing sync with the asc cli. Use when onboarding a new app, rotating signing assets, or sharing them across a team.
---

# asc signing setup

Use this skill when you need to create or renew signing assets for iOS/macOS apps.

## Preconditions
- Auth is configured (`asc auth login` or `ASC_*` env vars).
- You know the bundle identifier and target platform.
- You have a CSR file for certificate creation, or you will let `asc certificates create --generate-csr` create one.

## Workflow
1. Create or find the bundle ID:
   - `asc bundle-ids list --paginate`
   - `asc bundle-ids create --identifier "com.example.app" --name "Example" --platform IOS`
2. Configure bundle ID capabilities:
   - `asc bundle-ids capabilities list --bundle "BUNDLE_ID"`
   - `asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability ICLOUD`
   - Add capability settings when required:
     - `--settings '[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]'`
   - For the Developer Portal-only `PRIVATE_CLOUD_COMPUTE` capability, use a
     user-owned web session and the Developer Portal Bundle ID resource ID:
     - `asc web bundle-ids capabilities enable --bundle-id "BUNDLE_RESOURCE_ID" --capability PRIVATE_CLOUD_COMPUTE --confirm`
     - This capability is not available through the public App Store Connect
     capability enum. If the cached session cannot access Developer Portal,
       clear its scoped cache, then log in again with the same binary:
       - `asc web auth logout --apple-id "user@example.com"`
       - `asc web auth login --apple-id "user@example.com"`
   - For App Groups, the public API can enable `APP_GROUPS` but cannot create or
     associate App Group resources. Use an Account Holder or Admin web session:
     - `asc web app-groups list --paginate --output table`
     - `asc web app-groups create --name "Example Shared" --identifier "group.com.example.app.shared" --confirm`
     - `asc web app-groups assign --group "GROUP_RESOURCE_ID" --bundle-id "BUNDLE_RESOURCE_ID" --confirm`
     - Resolve the opaque group ID with `asc web app-groups list` and the opaque
       Bundle ID resource ID with `asc bundle-ids list`. A changed assignment
       invalidates provisioning profiles containing that App ID, so regenerate
       affected profiles before the next signed build.
3. Create a signing certificate:
   - `asc certificates list --certificate-type IOS_DISTRIBUTION`
   - `asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"`
   - Or generate a key and CSR inline:
     - `asc certificates create --certificate-type IOS_DISTRIBUTION --generate-csr --key-out "./signing/dist.key" --csr-out "./signing/dist.csr"`
   - For Wallet passes, create the Pass Type ID first, then create its certificate:
     - `asc pass-type-ids create --identifier "pass.com.example" --name "Example Pass"`
     - `asc certificates create --certificate-type PASS_TYPE_ID --pass-type-id "PASS_TYPE_ID" --csr "./pass.csr"`
     - `asc pass-type-ids certificates list --pass-type-id "PASS_TYPE_ID" --paginate`
4. Create a provisioning profile:
   - `asc profiles create --name "AppStore Profile" --profile-type IOS_APP_STORE --bundle "BUNDLE_ID" --certificate "CERT_ID"`
   - Include devices for development/ad-hoc:
     - `asc profiles create --name "Dev Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID" --device "DEVICE_ID"`
5. Download the profile:
   - `asc profiles download --id "PROFILE_ID" --output "./profiles/AppStore.mobileprovision"`
6. Inspect and install the downloaded profile locally when needed:
   - `asc profiles inspect --path "./profiles/AppStore.mobileprovision" --output table`
   - `asc profiles inspect --path "./profiles/AppStore.mobileprovision" --entitlements --output markdown`
   - `asc profiles local install --path "./profiles/AppStore.mobileprovision"`
   - `asc profiles local list --output table`
   - On macOS, the default directory follows the active Xcode: Xcode 16 or newer uses `~/Library/Developer/Xcode/UserData/Provisioning Profiles`; Xcode 15 or older uses `~/Library/MobileDevice/Provisioning Profiles`. Hosts without a full active Xcode fall back to the legacy directory and print a note to stderr.
   - Pass `--install-dir` when automation must target a fixed directory.

## Rotation and cleanup
- Revoke old certificates:
  - `asc certificates revoke --id "CERT_ID" --confirm`
- Audit remote provisioning profiles before deleting or rotating:
  - `asc profiles list --profile-state ACTIVE,INVALID --paginate --output json`
  - Apple `profileState` is not a complete expiration signal: some profiles can have a past `expirationDate` while still reporting `ACTIVE`. For true expired-profile audits, compare `expirationDate` against the current date instead of relying only on `INVALID`.
- Delete old profiles:
  - `asc profiles delete --id "PROFILE_ID" --confirm`
- Clean local Xcode provisioning profiles:
  - `asc profiles local clean --expired --dry-run`
  - `asc profiles local clean --expired --confirm`
  - Check the resolved directory in the dry-run output before confirming, or pin it with `--install-dir`.

## Shared team storage with `asc signing sync`
Use this when you want a lightweight, non-interactive alternative to fastlane match for encrypted git-backed certificate/profile storage.

```bash
# Protect secret inputs before use
chmod 600 "./signing-sync-password" "./distribution.p12" "./distribution-p12-password"

# Push a usable private identity with its matching certificate and profile
asc signing sync push \
  --bundle-id "com.example.app" \
  --profile-type IOS_APP_ADHOC \
  --repo "git@github.com:team/certs.git" \
  --password-file "./signing-sync-password" \
  --identity "./distribution.p12" \
  --identity-password-file "./distribution-p12-password" \
  --output json

# Pull and decrypt them into a local directory
asc signing sync pull \
  --repo "git@github.com:team/certs.git" \
  --password-file "./signing-sync-password" \
  --output-dir "./signing" \
  --output json
```

Notes:
- App Store Connect never returns a private key. Supply the local PKCS#12 with
  `--identity`, or use `--private-key` with `--identity-sha256` to select its
  matching App Store Connect certificate. A multi-identity PKCS#12 also needs
  `--identity-sha256`.
- Prefer `--password-file`; `ASC_SIGNING_SYNC_PASSWORD` is the non-file fallback.
  `--password` was removed in 5.0.0 and is rejected. `ASC_MATCH_PASSWORD` is
  no longer read and is ignored if set.
- Certificate/profile-only sync remains supported but reports
  `identityPresent: false`; it is not a usable signing identity by itself.
- `pull` reports private identities in `sensitiveFiles` and writes them mode
  `0600`. Importing or using the pulled identity remains a separate explicit step.
- Private identity sync rejects `MAC_APP_DIRECT` and
  `MAC_CATALYST_APP_DIRECT`; certificate/profile-only sync remains available.

## Reconcile ad hoc devices and profiles

Use the experimental reconcile workflow for deterministic, additive changes
derived from an Xcode archive and a protected desired-devices file:

```bash
asc signing reconcile plan \
  --archive-path ".asc/artifacts/App.xcarchive" \
  --devices-file ".asc/distribution/devices.json" \
  --output json

asc signing reconcile apply \
  --plan ".asc/distribution/signing/plan.json" \
  --confirm \
  --output json
```

Planning performs no mutation and may return `ready: false`. Apply can register
missing devices, create safe baseline App IDs, and create successor ad hoc
profiles; it never deletes or patches resources, enables capabilities, or
creates certificates. Review the plan before `--confirm`. Use the
`asc-ad-hoc-distribution` skill when these signing effects should be bound into
an end-to-end distribution plan hash.

## Run one command with an ephemeral identity

On macOS, avoid persistent login-keychain and profile changes by wrapping the
child command:

```bash
asc signing run \
  --identity "./signing/App.p12" \
  --identity-password-file "./signing/App-password" \
  --profile "./signing/App.mobileprovision" \
  --receipt ".asc/distribution/signing-run.json" \
  -- xcodebuild -exportArchive \
    -archivePath ".asc/artifacts/App.xcarchive" \
    -exportPath ".asc/artifacts/release-testing" \
    -exportOptionsPlist ".asc/ExportOptions.release-testing.plist"
```

The command runs directly without a shell, preserves the child's exit code,
uses an isolated temporary keychain, and cleans up its temporary profile. It
does not print success data, so the child owns stdout. Never pass identity
passwords inline.

## Notes
- Always check `--help` for the exact enum values (certificate types, profile types).
- Use `--paginate` for large accounts.
- `--certificate` accepts comma-separated IDs when multiple certificates are required.
- Device management uses `asc devices` commands (UDID required).
- `asc profiles inspect` and `asc profiles local ...` operate on local disk state, not App Store Connect API resources.
