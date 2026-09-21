# Digital-goods readiness

Read this reference only when IAP or subscription validation fails, Apple requires a first-review attachment, or a versioned product must join an existing review submission.

When work starts from an existing review draft, preserve its `SUBMISSION_ID` in the handoff to `asc-release-flow`. The release flow must inspect and reuse that draft rather than create another submission.

## Contents

- [Run product validators](#run-product-validators)
- [Attach subscriptions for first review](#attach-subscriptions-for-first-review)
- [Prepare a subscription version](#prepare-a-subscription-version)
- [Prepare a subscription-group version](#prepare-a-subscription-group-version)
- [Prepare an IAP version](#prepare-an-iap-version)
- [Handle first-version IAP attachment](#handle-first-version-iap-attachment)

## Run product validators

Start with the relevant validator:

```bash
asc validate iap --app "APP_ID" --output table
asc validate subscriptions --app "APP_ID" --output table
```

Use JSON when automation needs the exact diagnostic payload:

```bash
asc validate subscriptions --app "APP_ID" --output json --pretty
```

Fix missing localization, pricing, availability, review screenshots, and promotional images before attaching products to review.

## Attach subscriptions for first review

The public API cannot perform the first-review subscription selection. Inspect the web-session state:

```bash
asc web review subscriptions list --app "APP_ID"
```

Attach the intended group:

```bash
asc web review subscriptions attach-group \
  --app "APP_ID" \
  --group-id "GROUP_ID" \
  --confirm
```

Attach one subscription instead when that is the intended review unit:

```bash
asc web review subscriptions attach \
  --app "APP_ID" \
  --subscription-id "SUB_ID" \
  --confirm
```

These commands require an authenticated Apple web session. If the user declines web-session automation, select the subscription manually in App Store Connect.

Do not build workflows around `asc subscriptions review submit`; it was removed in 5.0.0.

## Prepare a subscription version

Use this public-API flow for an existing review submission. It does not replace first-review web attachment.

Resolve a `PREPARE_FOR_SUBMISSION` version:

```bash
asc subscriptions versions list \
  --subscription-id "SUB_ID" \
  --state PREPARE_FOR_SUBMISSION \
  --paginate \
  --output json
```

Branch before writing:

- Zero versions: create one and capture `.data.id` as `SUBSCRIPTION_VERSION_ID`.
- One version: reuse `.data[0].id`.
- More than one: stop and require an explicit version ID.

Create only in the zero-result branch:

```bash
asc subscriptions versions create --subscription-id "SUB_ID" --output json
```

Resolve the intended locale:

```bash
asc subscriptions versions localizations list \
  --version-id "SUBSCRIPTION_VERSION_ID" \
  --paginate \
  --output json
```

Create `en-US` only when absent:

```bash
asc subscriptions versions localizations create \
  --version-id "SUBSCRIPTION_VERSION_ID" \
  --locale "en-US" \
  --name "Premium" \
  --description "Premium access"
```

Update the one resolved localization when its values differ:

```bash
asc subscriptions versions localizations update \
  --id "SUBSCRIPTION_LOC_ID" \
  --name "Premium" \
  --description "Premium access"
```

Resolve images before uploading or deleting anything:

```bash
asc subscriptions versions images list \
  --version-id "SUBSCRIPTION_VERSION_ID" \
  --paginate \
  --output json
```

Upload only when the intended image is absent:

```bash
asc subscriptions versions images upload \
  --version-id "SUBSCRIPTION_VERSION_ID" \
  --file "./promotional.png"
```

Treat replacement as a separate confirmed branch:

```bash
asc subscriptions versions images delete --id "SUBSCRIPTION_IMAGE_ID" --confirm
asc subscriptions versions images upload --version-id "SUBSCRIPTION_VERSION_ID" --file "./promotional.png"
```

Inspect the completed version and return its ID to `asc-release-flow`:

```bash
asc subscriptions versions view --id "SUBSCRIPTION_VERSION_ID" --output table
asc subscriptions versions localizations list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output table
asc subscriptions versions images primary --version-id "SUBSCRIPTION_VERSION_ID" --output table
asc subscriptions versions images list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output table
```

Do not add the version to a review submission from this skill. Hand `SUBSCRIPTION_VERSION_ID` to the multi-item workflow in `asc-release-flow`.

## Prepare a subscription-group version

Resolve the group version:

```bash
asc subscriptions groups versions list \
  --group-id "GROUP_ID" \
  --state PREPARE_FOR_SUBMISSION \
  --paginate \
  --output json
```

Branch before writing:

- Zero versions: create one and capture `.data.id` as `GROUP_VERSION_ID`.
- One version: reuse `.data[0].id`.
- More than one: stop and require an explicit version ID.

Create only in the zero-result branch:

```bash
asc subscriptions groups versions create --group-id "GROUP_ID" --output json
```

Resolve and repair the intended locale:

```bash
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output json
asc subscriptions groups versions localizations create --version-id "GROUP_VERSION_ID" --locale "en-US" --name "Premium"
asc subscriptions groups versions localizations update --id "GROUP_LOC_ID" --name "Premium"
```

Run create only when the locale is absent; run update only when the single resolved locale differs.

Inspect the group version and return its ID to `asc-release-flow`:

```bash
asc subscriptions groups versions view --version-id "GROUP_VERSION_ID" --output table
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output table
```

Do not add the version to a review submission from this skill. Hand `GROUP_VERSION_ID` to the multi-item workflow in `asc-release-flow`.

## Prepare an IAP version

Upload a missing legacy review screenshot when diagnostics request one:

```bash
asc iap review-screenshots create --iap-id "IAP_ID" --file "./review.png"
```

Do not build workflows around `asc iap submit`; it was removed in 5.0.0.

Resolve a version-scoped IAP:

```bash
asc iap versions list \
  --iap-id "IAP_ID" \
  --state PREPARE_FOR_SUBMISSION \
  --paginate \
  --output json
```

Branch before writing:

- Zero versions: create one and capture `.data.id` as `IAP_VERSION_ID`.
- One version: reuse `.data[0].id`.
- More than one: stop and require an explicit version ID.

Create only in the zero-result branch:

```bash
asc iap versions create --iap-id "IAP_ID" --output json
```

Resolve and repair the intended localization:

```bash
asc iap versions localizations list --version-id "IAP_VERSION_ID" --paginate --output json
asc iap versions localizations create --version-id "IAP_VERSION_ID" --locale "en-US" --name "Premium" --description "Unlock premium features"
asc iap versions localizations update --localization-id "IAP_LOC_ID" --name "Premium" --description "Unlock premium features"
```

Create only when `en-US` is absent. Update only when one resolved localization differs.

Resolve images before writing:

```bash
asc iap versions images list --version-id "IAP_VERSION_ID" --paginate --output json
```

Create an image only when the intended file is absent:

```bash
asc iap versions images create --version-id "IAP_VERSION_ID" --file "./review.png"
```

Treat replacement as a separate confirmed branch:

```bash
asc iap versions images delete --image-id "IAP_IMAGE_ID" --confirm
asc iap versions images create --version-id "IAP_VERSION_ID" --file "./review.png"
```

Inspect the version and return its ID to `asc-release-flow`:

```bash
asc iap versions view --version-id "IAP_VERSION_ID" --output table
asc iap versions localizations list --version-id "IAP_VERSION_ID" --paginate --output table
asc iap versions image --version-id "IAP_VERSION_ID" --output table
asc iap versions images list --version-id "IAP_VERSION_ID" --paginate --output table
```

Do not add the version to a review submission from this skill. Hand `IAP_VERSION_ID` to the multi-item workflow in `asc-release-flow`.

## Handle first-version IAP attachment

Apple may require the first IAP, a new IAP type, or a non-renewing IAP to be selected with the next app version. Prepare localization, pricing, and review screenshots first.

Use the web-session fallback only for that public-API gap:

```bash
asc web review iaps attach --app "APP_ID" --iap-id "IAP_ID" --confirm
```

Say that the command requires an authenticated Apple web session. Keep manual selection in the app version's “In-App Purchases and Subscriptions” section as the fallback.
