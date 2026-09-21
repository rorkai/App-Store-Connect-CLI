# Readiness repairs

Read only the section that matches a proven `asc validate` or `asc review doctor` blocker.

## Contents

- [Build processing and encryption](#build-processing-and-encryption)
- [Content rights](#content-rights)
- [Age rating](#age-rating)
- [Version metadata and localizations](#version-metadata-and-localizations)
- [App info and privacy policy URL](#app-info-and-privacy-policy-url)
- [Screenshots](#screenshots)
- [Initial availability](#initial-availability)
- [Review details](#review-details)

## Build processing and encryption

Inspect the exact build before editing declarations:

```bash
asc builds info --build-id "BUILD_ID" --output table
```

Require `processingState` to be `VALID`. If encryption remains unresolved, inspect existing declarations:

```bash
asc encryption declarations list --app "APP_ID" --output table
```

Create a declaration only when its answers accurately describe the binary:

```bash
asc encryption declarations create \
  --app "APP_ID" \
  --app-description "Uses standard HTTPS/TLS" \
  --contains-proprietary-cryptography=false \
  --contains-third-party-cryptography=true \
  --available-on-french-store=true
```

Assign the declaration to the resolved build:

```bash
asc encryption declarations assign-builds --id "DECLARATION_ID" --build-id "BUILD_ID"
```

If the app truly uses only exempt transport encryption, update the local plist and rebuild instead of making a false declaration:

```bash
asc encryption declarations exempt-declare --plist "./Info.plist"
```

## Content rights

Read the current declaration, then edit it only when the answer is known:

```bash
asc apps content-rights view --app "APP_ID" --output table
asc apps content-rights edit --app "APP_ID" --uses-third-party-content=false
```

## Age rating

Inspect the full declaration before a partial edit:

```bash
asc age-rating view --app "APP_ID" --output table
```

Set the social-media fields only when they describe the app:

```bash
asc age-rating edit --app "APP_ID" --social-media false --social-media-age-restricted false
```

Apple requires `userGeneratedContent=true` before `socialMedia` can be true. It requires both `ageAssurance=true` and `socialMedia=true` before `socialMediaAgeRestricted` can be true. Set every required prerequisite in the same edit or preserve the current values explicitly.

Use `--age-rating-override-v2` when an override is required. The older `--age-rating-override` flag is deprecated.

## Version metadata and localizations

Inspect the version and its attached resources:

```bash
asc versions view --version-id "VERSION_ID" --include-build --include-submission --output table
asc localizations list --version "VERSION_ID" --output table
```

Use the canonical metadata workflow for repair:

```bash
asc metadata pull --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
asc metadata validate --dir "./metadata" --output table
asc metadata push --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run --output table
asc metadata push --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
```

Review the dry-run diff before applying metadata. Do not overwrite local changes with a pull unless the user has chosen the remote copy as the source of truth.

## App info and privacy policy URL

Resolve the app-info record and its localizations:

```bash
asc apps info list --app "APP_ID" --output table
asc localizations list --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --output table
```

For every iOS or macOS app, populate the privacy policy URL when missing:

```bash
asc app-setup info set \
  --app "APP_ID" \
  --primary-locale "en-US" \
  --privacy-policy-url "https://example.com/privacy"
```

## Screenshots

Inspect the localization's current screenshot set and the accepted dimensions:

```bash
asc screenshots list --version-localization "LOC_ID" --output table
asc screenshots sizes --output table
asc screenshots validate --path "./screenshots" --device-type "IPHONE_65" --output table
```

Use `asc-screenshot-resize` or `asc-shots-pipeline` for screenshot production. Return here after the files pass validation and are uploaded.

## Initial availability

First confirm that no availability record exists:

```bash
asc pricing availability view --app "APP_ID" --output table
```

Bootstrap the first record through the public API:

```bash
asc pricing availability create \
  --app "APP_ID" \
  --territory "USA,GBR" \
  --available true \
  --available-in-new-territories true
```

If Apple rejects this public-API bootstrap, it has not configured availability. Authenticate a web session with `asc web auth login --apple-id "EMAIL"`, then retry through the web fallback:

```bash
asc web apps availability create \
  --app "APP_ID" \
  --territory "USA,GBR" \
  --available-in-new-territories true
```

Alternatively, configure Pricing and Availability in App Store Connect.

Use the edit command only after a record exists:

```bash
asc pricing availability edit \
  --app "APP_ID" \
  --territory "USA,GBR" \
  --available true \
  --available-in-new-territories true
```

Do not assume a standard territory list. Ask the user which territories should receive the app.

## Review details

Inspect the version's review details:

```bash
asc review details-for-version --version-id "VERSION_ID" --output table
```

Create missing details:

```bash
asc review details-create \
  --version-id "VERSION_ID" \
  --contact-first-name "Dev" \
  --contact-last-name "Support" \
  --contact-email "dev@example.com" \
  --contact-phone "+1 555 0100" \
  --notes "Explain the reviewer access path here."
```

Update the resolved record instead of creating a duplicate:

```bash
asc review details-update --id "DETAIL_ID" --notes "Updated reviewer instructions."
```

Set demo-account fields only when App Review needs credentials, and never print secrets in logs or handoff text.
