---
name: asc-notarization
description: Archive, export, and notarize macOS apps using xcodebuild and asc. Use when you need to prepare a macOS app for distribution outside the App Store with Developer ID signing and Apple notarization.
---

# macOS Notarization

Use this skill when you need to notarize a macOS app for distribution outside the App Store.

## Preconditions
- Xcode installed and command line tools configured.
- Auth is configured (`asc auth login` or `ASC_*` env vars).
- A Developer ID Application certificate in the local keychain.
- The app's Xcode project builds for macOS.

## Preflight: Verify Signing Identity

Before archiving, confirm a valid Developer ID Application identity exists:

```bash
security find-identity -v -p codesigning | grep "Developer ID Application"
```

If no identity is found, create one at https://developer.apple.com/account/resources/certificates/add (the App Store Connect API does not support creating Developer ID certificates).

### Inspect Trust Settings

If `codesign` or `xcodebuild` fails with "Invalid trust settings" or "errSecInternalComponent", the certificate may have custom trust overrides that break the chain:

```bash
# Check for custom trust settings
security dump-trust-settings 2>&1 | grep -A1 "Developer ID"
```

These errors do not by themselves prove a trust override is the cause. Inspect the affected certificate before proposing a repair. Changing trust settings requires explicit authorization for that certificate; do not remove trust overrides or re-sign an arbitrary app as a diagnostic step.

## Step 1: Archive

```bash
xcodebuild archive \
  -scheme "YourMacScheme" \
  -configuration Release \
  -archivePath /tmp/YourApp.xcarchive \
  -destination "generic/platform=macOS"
```

## Step 2: Export with Developer ID

Create an ExportOptions plist for Developer ID distribution:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>method</key>
    <string>developer-id</string>
    <key>signingStyle</key>
    <string>automatic</string>
    <key>teamID</key>
    <string>YOUR_TEAM_ID</string>
</dict>
</plist>
```

Export the archive:

```bash
xcodebuild -exportArchive \
  -archivePath /tmp/YourApp.xcarchive \
  -exportPath /tmp/YourAppExport \
  -exportOptionsPlist ExportOptions.plist
```

This produces a `.app` bundle signed with Developer ID Application and a secure timestamp.

### Verify the Export

Verify the exported app's existing signature and nested code, then display its signing details:

```bash
codesign --verify --deep --strict --verbose=2 "/tmp/YourAppExport/YourApp.app" && \
  codesign --display --verbose=4 "/tmp/YourAppExport/YourApp.app" 2>&1
```

Confirm:
- The verification command exits successfully; displaying signing details alone does not validate the signature
- The Authority chain is Developer ID Application → Developer ID Certification Authority → Apple Root CA, with the expected TeamIdentifier
- A Timestamp is present

Stop before packaging or uploading if verification fails or the signing identity is unexpected. Diagnose the failure and rebuild or re-export the intended app before checking again. These checks do not modify the bundle and do not establish notarization acceptance.

Do not add `--sign` or `--force` to verification. [Apple's code-signing guidance](https://developer.apple.com/library/archive/technotes/tn2206/) distinguishes recursive verification with `--deep` from signing with `--deep --force`, which forcibly re-signs nested code.

## Step 3: Create a ZIP for Notarization

```bash
ditto -c -k --keepParent "/tmp/YourAppExport/YourApp.app" "/tmp/YourAppExport/YourApp.zip"
```

## Step 4: Submit for Notarization

### Fire-and-forget
```bash
asc notarization submit --file "/tmp/YourAppExport/YourApp.zip"
```

### Wait for result
```bash
asc notarization submit --file "/tmp/YourAppExport/YourApp.zip" --wait
```

### Custom polling
```bash
asc notarization submit --file "/tmp/YourAppExport/YourApp.zip" --wait --poll-interval 30s --timeout 1h
```

## Step 5: Check Results

### Status
```bash
asc notarization status --id "SUBMISSION_ID" --output table
```

### Developer Log (for failures)
```bash
asc notarization log --id "SUBMISSION_ID"
```

Fetch the log URL to see detailed issues:
```bash
curl -sL "LOG_URL" | python3 -m json.tool
```

### List Previous Submissions
```bash
asc notarization list --output table
asc notarization list --limit 5 --output table
```

## Step 6: Staple (Optional)

After notarization succeeds, staple the ticket so the app works offline:

```bash
xcrun stapler staple "/tmp/YourAppExport/YourApp.app"
```

For DMG or PKG distribution, staple after creating the container:
```bash
# Create DMG
hdiutil create -volname "YourApp" -srcfolder "/tmp/YourAppExport/YourApp.app" -ov -format UDZO "/tmp/YourApp.dmg"
xcrun stapler staple "/tmp/YourApp.dmg"
```

## Supported File Formats

| Format | Use Case |
|--------|----------|
| `.zip`  | Simplest; zip a signed `.app` bundle |
| `.dmg`  | Disk image for drag-and-drop install |
| `.pkg`  | Installer package (requires Developer ID Installer certificate) |

## PKG Notarization

To notarize `.pkg` files, you need a **Developer ID Installer** certificate (separate from Developer ID Application). This certificate type is not available through the App Store Connect API — create it at https://developer.apple.com/account/resources/certificates/add.

Sign the package:
```bash
productsign --sign "Developer ID Installer: YOUR NAME (TEAM_ID)" unsigned.pkg signed.pkg
```

Then submit:
```bash
asc notarization submit --file signed.pkg --wait
```

## Troubleshooting

### "Invalid trust settings" during export
Custom trust overrides are one possible cause. Use the read-only inspection in Preflight, identify the affected certificate, and obtain authorization before changing its trust settings.

### "The binary is not signed with a valid Developer ID certificate"
The app was signed with a Development or App Store certificate. Re-export with `method: developer-id` in ExportOptions.plist.

### "The signature does not include a secure timestamp"
Add `--timestamp` to manual `codesign` calls, or use `xcodebuild -exportArchive` which adds timestamps automatically.

### Upload timeout for large files
Set a longer upload timeout:
```bash
ASC_UPLOAD_TIMEOUT=5m asc notarization submit --file ./LargeApp.zip --wait
```

### Notarization returns "Invalid" but signing looks correct
Fetch the developer log for specific issues:
```bash
asc notarization log --id "SUBMISSION_ID"
```

Common causes: unsigned nested binaries, missing hardened runtime, embedded libraries without timestamps.

## Notes
- The `asc notarization` commands use the Apple Notary API v2, not `xcrun notarytool`.
- Authentication uses the same API key as other `asc` commands.
- Files are uploaded directly to Apple's S3 bucket with streaming (no full-file buffering).
- Files over 5 GB use multipart upload automatically.
- Always use `--help` to verify flags: `asc notarization submit --help`.
