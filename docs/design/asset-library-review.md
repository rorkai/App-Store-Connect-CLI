# Asset Library items in review submissions

Historical implementation scope and probe evidence are preserved below. For the current integrated workflow and OpenAPI 4.5.1 contract, see the [Asset Library guide](../../website/src/content/docs/guides/asset-library.mdx).

## Contract and evidence

Extend the existing generic review submission commands with `appAssetLibraryImages` and `appAssetLibraryVideos`. No new command group or orchestration wrapper is needed. `asc review items-add` and `asc review items add` previously rejected these resource types.

During the initial October 6, 2026 investigation against OpenAPI 4.5, the published snapshot did not describe these relationships. The observed App Store Connect browser bundle maps image and video resources to `appAssetLibraryImage` and `appAssetLibraryVideo` review item relationships. Its review item POST uses the existing `/v1/reviewSubmissionItems` endpoint with `reviewSubmission` linkage plus one asset linkage whose resource type is the plural form. An authenticated public GET of `/v1/reviewSubmissions/{id}/items` returned HTTP 200 with `fields[reviewSubmissionItems]=state,appAssetLibraryImage,appAssetLibraryVideo` and `include=appAssetLibraryImage,appAssetLibraryVideo`. These maintainer checks verify read support and the browser request contract; they do not independently prove a public POST or App Review acceptance. OpenAPI 4.5.1 now documents the `appAssetLibraryImage` and `appAssetLibraryVideo` review relationships. That schema support does not independently establish provider acceptance or App Review approval.

On October 6, 2026, [simonsruggi reported public image submission](https://github.com/rorkai/App-Store-Connect-CLI/pull/2932#issuecomment-6013396808) using a team API key with an ES256 JWT, `asc` 5.12.0 image upload, and direct public review API calls. The report covers nine apps with approved live versions and 18 images: a 21:9 product page header and a 3:2 search results asset per app. Creating an image review item returned HTTP 201; submitting the review moved the submission and images to `WAITING_FOR_REVIEW`. This is community-reported evidence, distinct from the maintainers' independent live checks described here and in capability verification notes. It does not establish video submission, asset approval, or successful placement on an approved live version; the reporter was still waiting for approval.

## Workflow

Apple's [Submit assets from Asset Library](https://developer.apple.com/help/app-store-connect/manage-submissions-to-app-review/submit-assets-from-asset-library) help describes standalone submission after an approved app version and co-submission with the first app version. For an app with an approved version, create a review submission and attach the assets independently:

```sh
asc review submissions-create --app APP_ID --platform IOS
asc review items-add --submission SUBMISSION_ID --item-type appAssetLibraryImages --item-id IMAGE_ID
asc review items-list --submission SUBMISSION_ID --include appAssetLibraryImage
asc review submissions-submit --id SUBMISSION_ID --confirm
```

The CLI also accepts `--item-type appAssetLibraryVideos` and `--include appAssetLibraryVideo`; successful public video submission has not been verified by the maintainer checks or the community image report.

For the first app version, submit the assets together with the version. Assets assigned to an active parent submission cannot be submitted separately. Apple validates asset and submission readiness; adding an item alone does not submit it. The existing submission confirmation requirement remains in place. No actual submission was performed by the maintainers for this change.

In the community report, two apps accepted a separate asset submission while an app version was already `WAITING_FOR_REVIEW`, leaving two submissions waiting simultaneously. Both apps already had an approved live version. This does not relax the first-version requirement or the restriction on assets assigned to an active submission.

## Place reviewed images on a live version

For an approved live app, the reported sequence is upload, standalone review, wait for each image to become `APPROVED`, then create its placements. `WAITING_FOR_REVIEW` is not approval, and the upload receipt's `ready` flag only describes image processing.

```sh
asc asset-library images view --id IMAGE_ID
# Continue only when the returned image state is APPROVED.
asc localizations placements create --localization-id LIVE_LOCALIZATION_ID --image-id HEADER_IMAGE_ID --placement-type PRODUCT_PAGE_HEADER_ASSET
asc localizations placements create --localization-id LIVE_LOCALIZATION_ID --image-id SEARCH_IMAGE_ID --placement-type APP_STORE_SEARCH_RESULTS_ASSET
```

The report found that creating a placement on a version in `WAITING_FOR_REVIEW` was rejected because its version state did not permit creation. Creating a placement on a live version with an unapproved asset was also rejected: the approved parent requires an `APPROVED` asset. Placement creation cannot bypass asset review. Successful creation after approval and the resulting live App Store presentation remain unverified in that report. See [header and search image assignment](creative-placement-writes.md) for the supported assignment scope.

## Compatibility and validation

The additional types are additive. Existing flags, timeouts, output formats, errors, and destructive-operation confirmations retain their behavior. List responses retain asset relationships and included resource envelopes; table and Markdown show their type and ID. Review history includes asset targets without requiring an app version. `--if-exists skip` reads back the matching relationship and retains strict collection/linkage validation: wrong resource types, null linkages, removed items, and unrelated conflicts cannot turn failure into success.

CLI tests establish RED by rejecting the new types, then check exact image/video POST bodies, list fields and includes, JSON included content, table/Markdown targets, history targets, strict duplicate read-back, and unknown-type validation before authentication. Existing submission tests cover required `--confirm`. Maintainer live submission remains unverified; the community image report reaches `WAITING_FOR_REVIEW`, not final approval or live placement.
