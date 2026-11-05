# Asset Library video and standalone media uploads

Historical implementation scope and probe evidence are preserved below. For the current integrated workflow and OpenAPI 4.5.1 contract, see the [Asset Library guide](../../website/src/content/docs/guides/asset-library.mdx).

Add `asc asset-library videos upload --library-id ID --file video.mp4`
and `--category CREATIVE_ASSETS|APP_SCREENSHOTS_AND_PREVIEWS` to image and video
upload commands. The default remains CREATIVE_ASSETS. These operations reserve
media independently; they do not assign placements or submit review.

The captured Apple uploader reserves POST /v1/appAssetLibraryImages or
/v1/appAssetLibraryVideos with fileName, fileSize, category and an assetLibrary
relationship. It transfers returned uploadOperations then PATCHes the reserved
resource with uploaded=true. These operations remain absent from OpenAPI 4.5;
public video/category acceptance requires the coordinating agent's live probe.

Video files must have .mp4/.m4v/.mov extensions, be regular nonempty files and
not exceed 500 MiB. ffprobe examines a private staged copy with a 30-second
limit, a 250 ms process-pipe shutdown grace, a 64 KiB output cap and local file/pipe protocols. The same staged copy
is uploaded. Runtime reference videoSpecs select dimensions, aspect ratio,
duration, frame rate, extension/MIME type, category and required audio before reservation.
ffprobe must identify the MOV/MP4/M4V container family before any API request;
a renamed unsupported container is rejected. Creative header/search videos use 5-30 seconds and 30/60 FPS; preview reference
specs use 15-30 seconds, 23-30 FPS and required audio, including the Duo 886x1920
and 1920x886 sizes. Container MIME comes from major/compatible ISO file brands, not the filename.
Concrete 3GPP/MJ2 brands are rejected even under the shared MOV demuxer. Unknown
major brands are accepted when compatible brands establish MPEG-4; an unknown
brand without supported compatibility is rejected. The reference schema supplies no codec allowlist; inspection
retains codec metadata but provider validation remains authoritative for codecs.

Reuse existing chunked upload transport and request/upload timeouts. Receipts
retain reserved IDs on transfer/commit/processing failure. Uploaded and Ready
are distinct: Ready requires PREPARE_FOR_SUBMISSION with a specId and a usable videoAsset URL; processed
poster dimensions remain optional and are not claimed as decoded-preview proof.
No automatic deletion, placement assignment or review submission occurs.

Focused tests cover command validation, category reservation body, video
reservation/transfer/commit/read paths, credential isolation on storage transfer,
and preview duration/FPS/audio/category distinctions. The coordinating agent
runs full gates, mandated reviews and public live transfer probes before a PR.


Processing boundary: captured public GET initially returned PREPARE_FOR_SUBMISSION,
a specId and a COMPLETE poster while videoAsset was null; later it returned FAILED
with POST_PROCESSING_FAILED. The editor passes videoAsset directly as videoUrl and
holds its reserved state in Processing when that field is absent. Waiting must
therefore require the video URL and must not treat poster completion as video
completion. Public successful library video processing remains a live acceptance
criterion, not established by the synthetic command test.


Video aspect ratios accept the reference endpoint's string ("21:9") and object
({"width":21,"height":9}) forms. Positive finite components are normalized as
exact rational numbers; variable dimension ranges require exact pixel ratios.
Fixed dimensions remain authoritative for rounded sizes such as 3840x1646.
Invalid individual ratio values cannot poison decoding of the entire envelope.

Live acceptance correction: a processing deadline controlled by `ASC_UPLOAD_TIMEOUT` originally rendered the generic `ASC_TIMEOUT` hint. The command now marks only an expired overall upload context as an upload operation, preserving request-timeout hints for child request deadlines. The existing pending-processing test reproduces the wrong hint before the change and passes afterward; a separate reservation-request deadline verifies the narrower request hint.

## PNG preflight integration

Merging the image transparency preflight exposed an unbounded ancillary PNG
metadata scan before image-data chunks. A cumulative 16 MiB metadata budget
now rejects oversized declared chunks before reading their payload and limits
numerous small chunks. Regression tests establish the old EOF/accepted-file
behavior before the fix and verify the corrected error and bounded reads.
Compressed image-data chunks are not drained; existing RGB/JPEG acceptance and
encoded alpha rejection remain covered. PNGs with unusually large pre-IDAT
metadata must be re-exported with that metadata removed.
