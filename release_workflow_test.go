package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowExportsHomebrewChecksumsBeforeFormulaGeneration(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	assignAMD64 := "SHA256_AMD64=$(shasum"
	assignArm64 := "SHA256=$(shasum"
	exportChecksums := "export SHA256_AMD64 SHA256"
	pythonStep := "python3 - <<'PY'"

	assignAMD64Index := strings.Index(workflow, assignAMD64)
	if assignAMD64Index == -1 {
		t.Fatalf("release workflow missing %q", assignAMD64)
	}

	assignArm64Index := strings.Index(workflow, assignArm64)
	if assignArm64Index == -1 {
		t.Fatalf("release workflow missing %q", assignArm64)
	}

	exportIndex := strings.Index(workflow, exportChecksums)
	if exportIndex == -1 {
		t.Fatalf("release workflow missing %q", exportChecksums)
	}

	pythonIndex := strings.Index(workflow, pythonStep)
	if pythonIndex == -1 {
		t.Fatalf("release workflow missing %q", pythonStep)
	}

	if assignAMD64Index > exportIndex || exportIndex > pythonIndex {
		t.Fatalf("%q must be assigned and exported before %q", assignAMD64, pythonStep)
	}
	if assignArm64Index > exportIndex || exportIndex > pythonIndex {
		t.Fatalf("%q must be assigned and exported before %q", assignArm64, pythonStep)
	}
}

func TestReleaseWorkflowTestsHomebrewFormulaUsingVersionStdout(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	want := `assert_match version.to_s, shell_output("#{{bin}}/asc --version")`
	if !strings.Contains(workflow, want) {
		t.Fatalf("release workflow missing stdout-based Homebrew formula test %q", want)
	}

	unwanted := `shell_output("#{{bin}}/asc --help")`
	if strings.Contains(workflow, unwanted) {
		t.Fatalf("release workflow still tests help output from stderr with %q", unwanted)
	}
}

func TestReleaseWorkflowKeepsHistoricalGuardrailsInline(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, want := range []string{
		`python3 scripts/test_check_docs.py`,
		`make format-check`,
		`make check-docs`,
		`make check-wall-of-apps`,
		`make lint`,
		`ASC_BYPASS_KEYCHAIN=1 make test`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing historical guardrail %q", want)
		}
	}
	if strings.Contains(workflow, "make release-guardrails") {
		t.Fatal("release workflow cannot call a target absent from historical tags")
	}
}

func TestReleaseRehearsalGuardrailsIncludeDocsValidatorSelfTest(t *testing.T) {
	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	makefile := string(data)
	start := strings.Index(makefile, "release-guardrails:\n")
	if start == -1 {
		t.Fatal("Makefile missing release-guardrails target")
	}
	end := strings.Index(makefile[start:], "\n# Show help")
	if end == -1 {
		t.Fatal("could not find end of release-guardrails target")
	}
	guardrails := makefile[start : start+end]
	if !strings.Contains(guardrails, "python3 scripts/test_check_docs.py") {
		t.Fatal("release-guardrails must run the docs-validator self-test")
	}
}

func TestReleaseWorkflowBuildsStrippedTrimmedBinaries(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	buildLines := 0
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "go build") {
			continue
		}
		buildLines++
		if !strings.Contains(line, "-trimpath") {
			t.Errorf("release build line missing -trimpath: %s", strings.TrimSpace(line))
		}
		if !strings.Contains(line, "-s -w") {
			t.Errorf("release build line missing -s -w: %s", strings.TrimSpace(line))
		}
	}
	if buildLines == 0 {
		t.Fatal("release workflow contains no go build lines")
	}
}

func TestReleaseWorkflowDoesNotInterpolateDispatchInputIntoShell(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	if strings.Contains(workflow, `TAG="${{ github.event_name == 'workflow_dispatch'`) {
		t.Fatal("release workflow interpolates dispatch input directly into shell")
	}
	for _, want := range []string{
		`RELEASE_TAG: ${{ github.event_name == 'workflow_dispatch' && inputs.version || github.ref_name }}`,
		`TAG="${RELEASE_TAG}"`,
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow missing safe dispatch handling %q", want)
		}
	}
}

func TestReleaseWorkflowNotarizesMacOSBinariesBeforePublishing(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	releaseStart := strings.Index(workflow, "\n  build:\n")
	if releaseStart == -1 {
		t.Fatal("release workflow missing build job")
	}
	releaseJob := workflow[releaseStart:]
	for _, want := range []string{
		`ASC_KEY_ID: ${{ secrets.ASC_KEY_ID }}`,
		`ASC_ISSUER_ID: ${{ secrets.ASC_ISSUER_ID }}`,
		`ASC_PRIVATE_KEY_B64: ${{ secrets.ASC_PRIVATE_KEY_B64 }}`,
		`ASC_PRIVATE_KEY_PATH: ${{ runner.temp }}/AuthKey.p8`,
		`ASC_BYPASS_KEYCHAIN: "1"`,
		`notarization list --limit 1 --output json`,
		`notarization submit`,
		`--wait`,
		`--timeout 1h`,
		`notarization log --id`,
		`codesign -vvvv -R="notarized" --check-notarization`,
	} {
		if !strings.Contains(releaseJob, want) {
			t.Errorf("release workflow missing notarization contract %q", want)
		}
	}
	if strings.Contains(releaseJob, `auth status --validate`) {
		t.Fatal("release workflow must validate the environment credentials with a Notary API request")
	}
	if strings.Contains(releaseJob, `spctl --assess`) {
		t.Fatal("release workflow must not assess standalone binaries with spctl")
	}

	notarizeIndex := strings.Index(workflow, "- name: Notarize macOS binaries")
	checksumIndex := strings.Index(workflow, "- name: Create checksums")
	publishIndex := strings.Index(workflow, "- name: Create, resume, or verify GitHub Release")
	if notarizeIndex == -1 || checksumIndex == -1 || publishIndex == -1 {
		t.Fatalf("release workflow must contain notarization, checksum, and publish steps")
	}
	if notarizeIndex >= checksumIndex || checksumIndex >= publishIndex {
		t.Fatalf("release workflow must notarize before checksums and publish: notarize=%d checksum=%d publish=%d", notarizeIndex, checksumIndex, publishIndex)
	}
}

func TestReleaseWorkflowCanRepairExistingNotarizationWithoutReplacingAssets(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	if !strings.Contains(workflow, "notarize_existing:") {
		t.Fatal("release workflow missing notarize_existing dispatch input")
	}

	start := strings.Index(workflow, "\n  repair-notarization:\n")
	end := strings.Index(workflow, "\n  prepare:\n")
	if start == -1 || end == -1 || start >= end {
		t.Fatal("release workflow must define repair-notarization before release")
	}
	repairJob := workflow[start:end]

	for _, want := range []string{
		`persist-credentials: false`,
		`gh release download "${VERSION}"`,
		`python3 workflow-source/scripts/verify_release_assets.py --release-dir existing-release --version "${VERSION}"`,
		`codesign --verify --deep --strict --verbose=2`,
		`is signed by an unexpected Developer ID`,
		`ASC_KEY_ID: ${{ secrets.ASC_KEY_ID }}`,
		`ASC_ISSUER_ID: ${{ secrets.ASC_ISSUER_ID }}`,
		`ASC_PRIVATE_KEY_B64: ${{ secrets.ASC_PRIVATE_KEY_B64 }}`,
		`ASC_PRIVATE_KEY_PATH: ${{ runner.temp }}/AuthKey.p8`,
		`ASC_BYPASS_KEYCHAIN: "1"`,
		`notarization list --limit 1 --output json`,
		`notarization submit`,
		`codesign -vvvv -R="notarized" --check-notarization`,
	} {
		if !strings.Contains(repairJob, want) {
			t.Errorf("repair-notarization job missing %q", want)
		}
	}
	for _, unwanted := range []string{
		`auth status --validate`,
		`spctl --assess`,
	} {
		if strings.Contains(repairJob, unwanted) {
			t.Errorf("repair-notarization job contains invalid verification %q", unwanted)
		}
	}

	for _, unwanted := range []string{
		`gh release upload`,
		`gh release create`,
		`codesign --force`,
		`--clobber`,
	} {
		if strings.Contains(repairJob, unwanted) {
			t.Errorf("repair-notarization job must not replace release assets; found %q", unwanted)
		}
	}
}

func TestReleaseWorkflowCreatesPrivateKeyWithRestrictedModeInRunnerTemp(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	if got := strings.Count(workflow, `install -m 600 /dev/null "$RUNNER_TEMP/AuthKey.p8"`); got != 2 {
		t.Fatalf("release workflow must securely create both temporary keys, got %d sites", got)
	}
	cleanup := "- name: Remove notarization credentials\n        if: always()\n        run: rm -f \"$RUNNER_TEMP/AuthKey.p8\""
	if got := strings.Count(workflow, cleanup); got != 2 {
		t.Fatalf("release workflow must unconditionally clean up both temporary keys, got %d sites", got)
	}
	if strings.Contains(workflow, "/tmp/AuthKey.p8") {
		t.Fatal("release workflow must not store private keys in shared /tmp")
	}
}

func TestReleaseWorkflowReusesOneBuildArtifactForEveryPublisher(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, want := range []string{
		"\n  prepare:\n",
		"\n  build:\n",
		"\n  publish:\n",
		"\n  homebrew:\n",
		"\n  winget:\n",
		"outputs:\n      version: ${{ steps.version.outputs.version }}",
		"actions/upload-artifact@",
		"actions/download-artifact@",
		"name: candidate-release-${{ needs.prepare.outputs.version }}",
		"name: published-release-${{ needs.publish.outputs.version }}",
		`tar -cf "workflow-artifact/candidate-release-${VERSION}.tar" release`,
		`tar -cf "workflow-artifact/published-release-${VERSION}.tar" release`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing split-job contract %q", want)
		}
	}

	if got := strings.Count(workflow, "name: candidate-release-${{ needs.prepare.outputs.version }}"); got != 3 {
		t.Fatalf("release workflow must define two mutually exclusive uploads and one download for the candidate artifact, got %d references", got)
	}
	if got := strings.Count(workflow, "actions/download-artifact@"); got != 3 {
		t.Fatalf("publishers must consume the candidate and published artifacts in three downloads, got %d", got)
	}
}

func TestReleaseWorkflowUsesValidSecureReleaseSteps(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	if strings.Contains(workflow, "- parallel:") {
		t.Fatal("release workflow must use valid run or uses steps")
	}
	if !strings.Contains(workflow, "go-version-file: go.mod\n          cache: false") {
		t.Fatal("release build must not restore a shared Go build cache")
	}
	if !strings.Contains(workflow, `gh release download "${VERSION}" --repo "${GH_REPO}" --dir published-release`) {
		t.Fatal("published-release recovery must select the repository explicitly")
	}
}

func TestReleaseWorkflowReusesArtifactsAcrossRerunAttempts(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, want := range []string{
		`actions/runs/${GITHUB_RUN_ID}/artifacts`,
		`repos/${GH_REPO}/actions/artifacts`,
		`repos/${GH_REPO}/actions/artifacts/${artifact_id}/zip`,
		`repos/${GH_REPO}/actions/runs/${artifact_run_id}`,
		`artifact_name="candidate-release-${VERSION}"`,
		`artifact_name="published-release-${VERSION}"`,
		`candidate-release-${VERSION}.commit`,
		`run_path=""`,
		`run_head_sha=""`,
		`run_event=""`,
		`run_repository=""`,
		`run_head_branch=""`,
		`run_path" != ".github/workflows/release.yml`,
		`run_repository" != "$GH_REPO`,
		`default_branch=$(gh api "repos/${GH_REPO}" --jq '.default_branch')`,
		`case "$run_event" in`,
		`push) [ "$run_head_sha" = "$release_commit" ] || continue ;;`,
		`workflow_dispatch) [ "$run_head_branch" = "$default_branch" ] || continue ;;`,
		`test "$(cat "$provenance")" = "$release_commit"`,
		`gh release download "${VERSION}" --dir "$draft_dir"`,
		`cmp -s "$draft_asset" "$candidate_dir/release/$name"`,
		`No retained candidate artifact matches the existing draft assets`,
		`cross_run=true`,
		`if: steps.candidate_artifact.outputs.cross_run == 'true'`,
		`if: steps.candidate_artifact.outputs.reused != 'true'`,
		`if: steps.published_artifact.outputs.reused != 'true'`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing rerun artifact contract %q", want)
		}
	}
}

func TestReleaseWorkflowRepairsPackageManagersWithoutRebuildingPublishedRelease(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, want := range []string{
		"\n  prepare:\n",
		`published: ${{ steps.release.outputs.published }}`,
		`gh api "repos/${GH_REPO}/releases/tags/${VERSION}" --jq '.draft'`,
		`elif ! grep -Fq "HTTP 404" "$release_error"`,
		`needs: prepare`,
		`needs.prepare.outputs.published != 'true'`,
		"needs:\n      - prepare\n      - build",
		`needs.prepare.outputs.published == 'true' || needs.build.result == 'success'`,
		`if: steps.published_artifact.outputs.reused != 'true' && needs.prepare.outputs.published == 'true'`,
		`gh release download "${VERSION}" --dir published-release`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing published-release repair contract %q", want)
		}
	}

	packStart := strings.Index(workflow, "- name: Pack immutable published artifact")
	packEnd := strings.Index(workflow, "- name: Upload immutable published artifact")
	if packStart == -1 || packEnd == -1 || packStart >= packEnd {
		t.Fatal("release workflow missing published artifact packing step")
	}
	if !strings.Contains(workflow[packStart:packEnd], "mkdir -p workflow-artifact") {
		t.Fatal("published-release repair must create the artifact directory before packing")
	}
}

func TestVerifyReleaseAssetsRequiresExactChecksumCoverage(t *testing.T) {
	workflowData, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	if got := strings.Count(string(workflowData), "verify_release_assets.py"); got != 6 {
		t.Fatalf("every release consumer must run exact checksum coverage verification, got %d verifier calls", got)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatalf("python3 is required for release asset verification tests: %v", err)
	}

	const version = "1.2.3"
	canonicalNames := []string{
		"asc_1.2.3_macOS_amd64",
		"asc_1.2.3_macOS_arm64",
		"asc_1.2.3_linux_amd64",
		"asc_1.2.3_linux_arm64",
		"asc_1.2.3_windows_amd64.exe",
	}

	tests := []struct {
		name             string
		omitCanonical    string
		addUnlistedAsset bool
		emptyAsset       string
		wrongDigest      bool
		pathTraversal    bool
		wantSucceeded    bool
	}{
		{name: "complete", wantSucceeded: true},
		{name: "missing canonical asset", omitCanonical: "asc_1.2.3_windows_amd64.exe"},
		{name: "unlisted asset", addUnlistedAsset: true},
		{name: "empty canonical asset", emptyAsset: "asc_1.2.3_macOS_arm64"},
		{name: "wrong digest", wrongDigest: true},
		{name: "path traversal", pathTraversal: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			releaseDir := t.TempDir()
			var manifest strings.Builder
			for index, name := range canonicalNames {
				if name == tt.omitCanonical {
					continue
				}
				asset := []byte("signed binary bytes for " + name)
				if name == tt.emptyAsset {
					asset = nil
				}
				if err := os.WriteFile(filepath.Join(releaseDir, name), asset, 0o600); err != nil {
					t.Fatalf("write asset: %v", err)
				}
				digest := sha256.Sum256(asset)
				digestText := fmt.Sprintf("%x", digest)
				manifestName := name
				if tt.wrongDigest && index == 0 {
					digestText = strings.Repeat("0", 64)
				}
				if tt.pathTraversal && index == 0 {
					manifestName = "../" + name
				}
				fmt.Fprintf(&manifest, "%s  %s\n", digestText, manifestName)
			}
			if tt.addUnlistedAsset {
				if err := os.WriteFile(filepath.Join(releaseDir, "asc_1.2.3_extra"), []byte("extra"), 0o600); err != nil {
					t.Fatalf("write extra asset: %v", err)
				}
			}
			if err := os.WriteFile(filepath.Join(releaseDir, "asc_1.2.3_checksums.txt"), []byte(manifest.String()), 0o600); err != nil {
				t.Fatalf("write checksum manifest: %v", err)
			}

			cmd := exec.Command("python3", "scripts/verify_release_assets.py", "--release-dir", releaseDir, "--version", version)
			output, err := cmd.CombinedOutput()
			if tt.wantSucceeded && err != nil {
				t.Fatalf("verify release assets: %v\n%s", err, output)
			}
			if !tt.wantSucceeded && err == nil {
				t.Fatalf("verification unexpectedly accepted incomplete or unsafe checksum coverage\n%s", output)
			}
		})
	}
}

func TestReleaseWorkflowResumesDraftButNeverClobbersPublishedAssets(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	if strings.Contains(workflow, "--clobber") {
		t.Fatal("release workflow must not replace a published asset")
	}
	for _, want := range []string{
		`--json isDraft`,
		`gh release create "${VERSION}" --draft --generate-notes --verify-tag`,
		`if [ "$release_is_draft" = "true" ]`,
		`gh release upload "${VERSION}" "$asset"`,
		`cmp -s "$asset" "published-release/$name"`,
		`gh release edit "${VERSION}" --draft=false`,
		`gh release download "${VERSION}"`,
		`python3 workflow-source/scripts/verify_release_assets.py --release-dir published-release --version "${VERSION}"`,
		`published-release-${VERSION}.tar`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing retry-safe publication contract %q", want)
		}
	}

	stepStart := strings.Index(workflow, "- name: Create, resume, or verify GitHub Release")
	stepEnd := strings.Index(workflow, "- name: Pack immutable published artifact")
	if stepStart == -1 || stepEnd == -1 || stepStart >= stepEnd {
		t.Fatal("release workflow missing publication step boundaries")
	}
	publishStep := workflow[stepStart:stepEnd]
	draftStart := strings.Index(publishStep, `if [ "$release_is_draft" = "true" ]`)
	if draftStart == -1 {
		t.Fatal("release workflow must checksum-verify and publish a completed draft")
	}
	checksumIndex := strings.Index(publishStep[draftStart:], `python3 workflow-source/scripts/verify_release_assets.py --release-dir published-release --version "${VERSION}"`)
	publishIndex := strings.Index(publishStep[draftStart:], `gh release edit "${VERSION}" --draft=false`)
	if checksumIndex == -1 || publishIndex == -1 {
		t.Fatal("release workflow must checksum-verify and publish a completed draft")
	}
	if checksumIndex >= publishIndex {
		t.Fatal("release workflow must verify draft checksums before making the release public")
	}
}

func TestReleaseWorkflowGrantsArtifactReadPermissionToArtifactJobs(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	jobs := []struct {
		name string
		next string
	}{
		{name: "build", next: "publish"},
		{name: "publish", next: "homebrew"},
		{name: "homebrew", next: "winget"},
		{name: "winget", next: ""},
	}
	for _, job := range jobs {
		start := strings.Index(workflow, "\n  "+job.name+":\n")
		if start == -1 {
			t.Fatalf("release workflow missing %s job", job.name)
		}
		end := len(workflow)
		if job.next != "" {
			next := strings.Index(workflow[start+1:], "\n  "+job.next+":\n")
			if next == -1 {
				t.Fatalf("release workflow missing %s job boundary", job.next)
			}
			end = start + 1 + next
		}
		if !strings.Contains(workflow[start:end], "permissions:\n      actions: read") {
			t.Errorf("%s job must have actions read permission for artifact access", job.name)
		}
	}
}

func TestReleaseWorkflowSerializesTagAndDispatchForSameVersion(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	want := "group: release-cli-${{ github.event_name == 'workflow_dispatch' && inputs.version || github.ref_name }}"
	if !strings.Contains(workflow, want) {
		t.Fatalf("release workflow missing normalized concurrency group %q", want)
	}
	if strings.Contains(workflow, "inputs.version || github.ref }}") {
		t.Fatal("release workflow splits tag and dispatch concurrency groups")
	}
}

func TestReleaseWorkflowUsesCommitTimestampForBuildMetadata(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	if !strings.Contains(workflow, `DATE=$(git show -s --format=%cI HEAD)`) {
		t.Fatal("release workflow must derive build metadata from the release commit")
	}
	if strings.Contains(workflow, `DATE=$(date -u`) {
		t.Fatal("release workflow must not embed the wall-clock build time")
	}
}

func TestReleaseWorkflowPushesWinGetBranchWithoutHistoryRewriteOrWorkflowScope(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, unwanted := range []string{
		"--force-with-lease",
		"git push --force",
		`git merge-base --is-ancestor origin/master "origin/${BRANCH}"`,
		`git checkout -b "${BRANCH}" upstream/master`,
		`grep -Ev "^manifests/r/Rorkai/ASC/${VERSION}/"`,
	} {
		if strings.Contains(workflow, unwanted) {
			t.Errorf("WinGet publication must not rewrite branch history; found %q", unwanted)
		}
	}
	for _, want := range []string{
		`git merge-base --is-ancestor origin/master upstream/master`,
		`git checkout -b "${BRANCH}" origin/master`,
		`git push --set-upstream origin "${BRANCH}"`,
		`git diff --name-only -z upstream/master...HEAD`,
		`case "$changed_path" in`,
		`"manifests/r/Rorkai/ASC/${VERSION}/"*)`,
		`WinGet branch contains changes outside the ${VERSION} manifest directory`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing fast-forward-only WinGet contract %q", want)
		}
	}
}
