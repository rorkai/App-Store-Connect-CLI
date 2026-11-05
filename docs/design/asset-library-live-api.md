# Asset Library public reads

Historical implementation scope and probe evidence are preserved below. For the current integrated workflow and OpenAPI 4.5.1 contract, see the [Asset Library guide](../../website/src/content/docs/guides/asset-library.mdx).

Apple's published OpenAPI 4.5 has no Asset Library operations. On 2026-10-06,
read-only requests authenticated with an App Store Connect API key returned 200
for these operations on the owner's ASC Test app:

- GET /v1/apps/{appId}/assetLibrary
- GET /v1/appAssetLibraries/{libraryId}
- GET /v1/appAssetLibraries/{libraryId}/images?limit=1
- GET /v1/appAssetLibraries/{libraryId}/videos?limit=1
- GET /v1/appAssetLibraryImages/{imageId}
- GET /v1/appAssetLibraryImages/{imageId}/placements?limit=1
- GET /v1/appAssetLibraryRefData

The signed-in web UI uses /iris/v1/appAssetLibraries/{libraryId}/media. The
public /v1 equivalent returns 404 (relationship does not exist); use images
and videos separately. Public collection GET /v1/appAssetLibraries returns 403
and explicitly allows GET_INSTANCE only.

## CLI contract

Expose `asc asset-library view --app APP_ID`, `images list --library-id ID`,
`videos list --library-id ID`, `images view --id ID`, `images placements --id ID`,
and `specs`. Collection reads accept --limit (1–200, or 0 for the server default), --next,
and --paginate. Live reads on images, videos, and image placements accepted
200 and rejected 201 with HTTP 400. --next
replaces resource selection and conflicts with selectors and --limit.

Preserve the complete Apple JSON envelope using the existing raw client and
raw pagination. Table and Markdown summarize media IDs, names, categories,
states, and placement types; specs display Apple's dimensions and compatibility.
Aspect ratios display known strings directly and opaque values as compact JSON,
without assuming a schema; absent and null values remain blank.
There are no upload, assignment, deletion, or review-submission commands in this
change. They need independently verified mutation contracts and authorized live
verification. Do not fabricate entries in the official OpenAPI snapshot.

## Verification

CLI RED/GREEN coverage establishes root registration, exact GET paths,
selector validation before authentication, unknown/null JSON preservation,
pagination, table output, and malformed responses. Run the full repository gates
and both mandated local reviews. Built commands receive read-only live smoke
verification on the same test app. Videos are an empty collection on that app;
nonempty video detail and mutation acceptance are unverified.
