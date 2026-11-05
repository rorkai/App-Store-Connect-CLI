# Header and search image assignment

Historical implementation scope and probe evidence are preserved below. For the current integrated workflow and OpenAPI 4.5.1 contract, see the [Asset Library guide](../../website/src/content/docs/guides/asset-library.mdx).

This note records the initial shipped scope. The shared placement commands are extended by [Library asset reuse](library-asset-reuse.md).
Current `asc localizations placements --help` exposes only `list`. Add `create`
and confirmed `delete` on this version-localization group. Creation supports only
version-localization images; CPP/PPO assignments, video assignment, reordering,
and other placement groups are outside the verified contract. Deletion is an
explicit generic placement-ID operation.

Apple's published OpenAPI 4.5 omits these resources. On 2026-10-06 authorized
public JWT probes returned POST `/v1/appAssetLibraryPlacements` HTTP 201 for both
`PRODUCT_PAGE_HEADER_ASSET` and `APP_STORE_SEARCH_RESULTS_ASSET`, followed by
GET instance HTTP 200 with state ACTIVE. DELETE of the owned placement returned
204. The request data type is appAssetLibraryPlacements; attributes are
placementType and placementGroup DEFAULT_PROFILE; relationships are image
(appAssetLibraryImages) and appStoreVersionLocalization
(appStoreVersionLocalizations). No private session is needed.

`asc localizations placements create --localization-id ID --image-id ID
--placement-type TYPE` assigns an already uploaded image. Apple validates image
spec suitability and occupied slots; the CLI never replaces an existing placement
automatically. Creating does not submit or publish. `delete --id ID --confirm`
removes the explicitly selected placement, retaining its reusable library media.
Deletion does not validate placement type or localization and is a generic
confirmed resource operation. Reads keep raw envelopes;
writes return exported camelCase receipts with registered table/Markdown
renderers. Usage validation precedes auth; API failures print no success receipt.

The maintainer POST/GET/DELETE probes above used the disposable app's draft
version; they do not prove placement acceptance on an approved live version.
A [community report on October 6, 2026](https://github.com/rorkai/App-Store-Connect-CLI/pull/2932#issuecomment-6013396808)
found that placement creation on a version in `WAITING_FOR_REVIEW` was rejected
because the version's state did not permit creation. On a live version, an
unapproved asset was rejected because an approved parent requires an `APPROVED`
asset. Follow the [standalone image review workflow](asset-library-review.md):
upload, submit the images for review, wait for `APPROVED`, then create placements
on the live version's localization. Image processing readiness and
`WAITING_FOR_REVIEW` do not satisfy this requirement. The report had not yet
verified approval, successful live-version placement, or live App Store display.

CLI RED/GREEN tests assert both exact payloads, required and unsupported inputs,
confirm-before-auth, create response-derived ID/state, deletion receipts,
human output, and API failures. Run repository gates and mandated reviews before
PR readiness. The parent owns live account fixtures and their cleanup; no live
mutations are part of this implementation agent's checks.
