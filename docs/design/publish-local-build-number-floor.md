# Local build-number floor for publish

`publish testflight` and `publish appstore` already choose a number above processed builds and in-flight uploads for the selected app/version/platform. They can choose a number below an app target's local `CURRENT_PROJECT_VERSION`.

Opt in with `--local-build-number-floor --project App.xcodeproj --scheme App`, optionally `--target App --configuration Release`. The next number is the greater of the existing remote-history result and the number immediately above the selected local value, using the existing dotted numeric comparison. With no history, `--initial-build-number` remains a lower bound. This does not reserve a number or edit the project.

The selected target must be an application. The existing structured project/xcconfig reader may infer a sole application target; multiple applications require `--target`. Read only the selected configuration (Release by default), not the maximum across other targets or configurations. The reader must resolve `CURRENT_PROJECT_VERSION` and `PRODUCT_BUNDLE_IDENTIFIER` without invoking Xcode. Unresolved variables or conditional settings fail preflight with guidance to use an explicit build number. Workspaces and prebuilt artifacts are unsupported for this opt-in.

The flag conflicts with `--build-number`; `--target` requires the flag. Raw archive arguments cannot override project, workspace, scheme, target, configuration, xcconfig, or the computed build number while the policy is active. After archiving, require the actual application's bundle ID and build number to match the selected target and computed number before export. Verify exported IPA metadata before upload as well; PKG metadata continues to come from the archive.

Without the opt-in, existing selection, explicit overrides, output shapes, and raw argument behavior remain unchanged. Usage errors go to stderr and exit 2; an artifact mismatch is an execution error and prevents subsequent stages. No API endpoint, schema, write, or response contract is added: existing read-only build and build-upload history queries remain the source for the remote candidate.

Validation covers CLI preflight, configuration and target selection, mixed local/remote numeric floors, and archive mismatch stops. A disposable native macOS project has an application (Debug 7, Release 42) and framework (Debug 800, Release 900); native proof uses one build job and no signing, export, reservation, or upload.
