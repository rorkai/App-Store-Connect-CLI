# Local archive and export identity

`asc xcode archive` reports `displayName` from the finished app's
`CFBundleDisplayName`, falling back to `CFBundleName`. `asc xcode export` reports
that field when it reads a final IPA. Local publish receipts preserve each
stage's name separately under `archive` and `export`. PKG and direct-upload
exports do not infer a display name from the archive; the archive stage still
reports its own name. Existing bundle, version, build-number and path fields
keep their established shape.

`--source-provenance` is an opt-in for `asc xcode archive` and local
`asc publish testflight` / `asc publish appstore` builds using `--project`.
The project must belong to a Git checkout with a committed HEAD. The archive
receipt adds `projectCheckout` containing `revision`, `dirty`, and
`observedAt: "buildStart"`. Local publish places this observation in its
`archive` stage only. No source path is included.

The observation happens before archiving, so outputs created during the build
do not make an initially clean observation dirty. Tracked changes, non-ignored untracked
files and dirty submodules count as dirty. This describes the selected project
checkout at that point; it does not attest to all build inputs, associate later
source changes with the artifact, or establish reproducibility.

Workspace inputs, prebuilt artifacts and existing remote builds reject
`--source-provenance` before building. A workspace can reference projects in
other checkouts. Exporting an existing archive never attributes the artifact
to the caller's current checkout.

```sh
asc xcode archive --project App.xcodeproj --scheme App \
  --configuration Release --archive-path .asc/artifacts/App.xcarchive \
  --source-provenance --output json
```
