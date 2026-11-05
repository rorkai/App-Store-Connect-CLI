# Product-page header and search-result placement reads

Historical implementation scope and probe evidence are preserved below. For the current integrated workflow and OpenAPI 4.5.1 contract, see the [Asset Library guide](../../website/src/content/docs/guides/asset-library.mdx).

This note records the initial shipped scope. The shared placement commands are extended by [Library asset reuse](library-asset-reuse.md).
On 2026-10-06 the ASC Test app's signed-in browser requested
GET /iris/v1/appStoreVersionLocalizations/{id}/placements with
filter[placementType]=PRODUCT_PAGE_HEADER_ASSET,APP_STORE_SEARCH_RESULTS_ASSET.
The equivalent API-key authenticated public /v1 route returned 200, including
when each type was requested separately and when include=image,video and
sort=placementGroupPosition were included. The test app has no header or search
asset assigned: empty 200 verifies the read/filter contract, not assignment.

Add `asc localizations placements list --localization-id ID` alongside the
existing preview-sets and screenshot-sets groups. Accept --placement-type for
Apple's observed enum values, --include image,video, --sort
placementGroupPosition, --limit, --next and --paginate. IDs are explicit;
localization discovery remains `asc localizations list --version VERSION_ID`.

A live public placement request with `limit=201` returned HTTP 400 with
`PARAMETER_ERROR.INVALID` and "The maximum allowable limit is '200'".
Both placement command paths accept 1-200, or 0 to omit the parameter and use
the server default; values outside that range fail before authentication.

Use the existing raw authenticated client and raw pagination so undocumented
attributes, relationships, included resources, nulls, and top-level fields remain
in JSON output. Table/Markdown summarize placement ID, media type, placement
kind, placement group, and state. Validate flags before auth/network. There are
no assignment/removal commands or private web-session calls.

CLI RED/GREEN tests cover a nonempty header response fixture, exact GET and
query, envelope preservation, table/Markdown, validation, pagination across empty
pages, and public errors. Live read-only smoke covers both empty placement types
on the exact test-app localization. Run full repository checks and both local
reviews before PR creation. Do not modify the official OpenAPI snapshot to
invent an undocumented schema.

Custom product page localization header and search placements were verified on an existing account page with HTTP200 using `/v1/appCustomProductPageLocalizations/{id}/placements`. Add the same read contract under `asc product-pages custom-pages localizations placements list`. PPO placements remain unexposed: no existing treatments, and the approved draft probe failed409 because ASC Test is not distributed on the App Store. No records were created.
