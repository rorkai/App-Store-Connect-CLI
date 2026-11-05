# Asset Library image upload

Historical implementation scope and probe evidence are preserved below. For the current integrated workflow and OpenAPI 4.5.1 contract, see the [Asset Library guide](../../website/src/content/docs/guides/asset-library.mdx).

On 2026-10-06 the owner's disposable ASC Test app verified public image
reservation (POST 201), returned object-storage PUT (200), commit (PATCH 200),
and GET processing to PREPARE_FOR_SUBMISSION with imageAsset dimensions.
The public operations remain absent from published OpenAPI 4.5.

`asc asset-library images upload --library-id ID --file image.png` adds an image
to the library. It never assigns placements or submits review. The CLI validates
and retains a rooted open PNG or JPEG image file before reserving remote state, uploads the
returned chunk operations with the shared uploader, and commits only the observed
`uploaded: true` attribute. It waits under the upload timeout for processed image
data in PREPARE_FOR_SUBMISSION, rather than legacy screenshot COMPLETE/checksum.
Each ASC HTTP request receives a fresh normal request timeout within that budget.
The owner verified GET 200 responses with state FAILED and null imageAsset on
2026-10-06. That state now ends processing immediately with an unsuccessful partial
receipt preserving the image ID, uploaded flag, and FAILED state. Other unknown
states remain pending until readiness or the upload timeout.

The JSON mutation receipt has imageId, libraryId, fileName, fileSize, uploaded,
and ready, plus state, specId, width, and height when known. uploaded means commit was accepted;
ready means processing reached the verified usable state. Table/Markdown show
that same result. `ready` does not mean App Review approved the asset. To assign
images to an approved live version, follow the [standalone review workflow](asset-library-review.md)
and wait for `APPROVED` before creating placements. An error after reservation prints the partial receipt before
returning unsuccessful status, retaining the ID for inspection or cleanup. The
command never silently deletes reservations, repeats an uncertain POST, or
prints signed upload operations. Required/invalid flags fail before side effects
and use usage exit code 2. Reads and existing commands remain unchanged.

Tests exercise CLI validation, exact reservation/commit contracts, multipart
bytes and headers, storage bearer-token separation, processing with initial null
imageAsset, signed URL error redaction, and partial receipts on upload, commit,
and processing failures. Existing uploader coverage proves retry mechanics.
Production write acceptance is supplied by the live probe evidence; this change
is verified locally without further live mutations. Full repository gates and
mandatory local reviews apply before PR readiness.
