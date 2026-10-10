package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func readReleaseWorkflow() ([]byte, error) {
	repositoryRoot, err := rootfs.New(".")
	if err != nil {
		return nil, err
	}
	return repositoryRoot.ReadFile(filepath.Join(".github", "workflows", "release.yml"))
}

var (
	releaseWorkflowJobsPattern     = regexp.MustCompile(`(?m)^jobs:\s*$`)
	releaseWorkflowTopLevelPattern = regexp.MustCompile(`(?m)^[A-Za-z0-9_-]+:\s*$`)
	releaseWorkflowJobPattern      = regexp.MustCompile(`(?m)^  [A-Za-z0-9_-]+:\s*$`)
)

func releaseWorkflowJobsBlock(t *testing.T, workflow string) string {
	t.Helper()

	jobsMatch := releaseWorkflowJobsPattern.FindStringIndex(workflow)
	if jobsMatch == nil {
		t.Fatal("release workflow missing jobs mapping")
	}

	jobsBlock := workflow[jobsMatch[1]:]
	if nextTopLevel := releaseWorkflowTopLevelPattern.FindStringIndex(jobsBlock); nextTopLevel != nil {
		jobsBlock = jobsBlock[:nextTopLevel[0]]
	}
	return jobsBlock
}

func releaseWorkflowJobBlock(t *testing.T, workflow, jobName string) string {
	t.Helper()

	jobsBlock := releaseWorkflowJobsBlock(t, workflow)
	jobHeader := "  " + jobName + ":"
	jobMatches := releaseWorkflowJobPattern.FindAllStringIndex(jobsBlock, -1)
	for index, match := range jobMatches {
		if jobsBlock[match[0]:match[1]] != jobHeader {
			continue
		}
		end := len(jobsBlock)
		if index+1 < len(jobMatches) {
			end = jobMatches[index+1][0]
		}
		return jobsBlock[match[0]:end]
	}

	t.Fatalf("release workflow missing %s job", jobName)
	return ""
}

func TestReleaseWorkflowJobBlockStopsAtNextJob(t *testing.T) {
	tests := []struct {
		name       string
		workflow   string
		unexpected []string
	}{
		{
			name:       "next job",
			workflow:   "jobs:\n  winget:\n    name: Submit WinGet package\n  later:\n    name: Later job\n",
			unexpected: []string{"later"},
		},
		{
			name:       "outside jobs mapping",
			workflow:   "jobs:\n  winget:\n    name: Submit WinGet package\non:\n  workflow_dispatch:\n",
			unexpected: []string{"on:", "workflow_dispatch"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := releaseWorkflowJobBlock(t, test.workflow, "winget")
			for _, unexpected := range test.unexpected {
				if strings.Contains(block, unexpected) {
					t.Fatalf("winget block included content outside the job: %q", block)
				}
			}
		})
	}
}

func TestReleaseWorkflowExportsHomebrewChecksumsBeforeFormulaGeneration(t *testing.T) {
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing historical guardrail %q", want)
		}
	}
	shardScript := "scripts/go_test_shard.py"
	guard := "if [ -f " + shardScript + " ]; then\n"
	fallback := "elif [ \"$UNSHARDED_FALLBACK\" = \"true\" ]; then\n"
	guardStart := strings.Index(workflow, guard)
	fallbackStart := strings.Index(workflow, fallback)
	if guardStart == -1 || fallbackStart < guardStart {
		t.Fatal("release test shards must guard the shard script and fall back for older tags")
	}
	if !strings.Contains(workflow[fallbackStart:], `ASC_BYPASS_KEYCHAIN=1 ASC_CONFIG_PATH="$config_dir/config.json" go test -count=1 -v ./...`) {
		t.Fatal("historical release tags must still run the full Go suite")
	}
	if !strings.Contains(workflow, "UNSHARDED_FALLBACK: ${{ matrix.name == 'packages' }}") {
		t.Fatal("exactly one release test shard must run the historical full suite")
	}
	outside := workflow[:guardStart] + workflow[fallbackStart:]
	if strings.Count(workflow, "python3 "+shardScript) != 1 || strings.Contains(outside, "python3 "+shardScript) {
		t.Fatal("release workflow may run the shard script only behind its existence guard")
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
	data, err := readReleaseWorkflow()
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

func TestReleaseWorkflowEnablesCGOForEveryMacOSArchitecture(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	workflow := string(data)
	for _, setting := range []string{`CGO_CFLAGS: "-O2 -g -mmacosx-version-min=13.0"`, `CGO_LDFLAGS: "-O2 -g -mmacosx-version-min=13.0"`} {
		if !strings.Contains(workflow, setting) {
			t.Fatalf("release workflow must include its macOS minimum in the CGO cache key: %s", setting)
		}
	}
	for _, arch := range []string{"amd64", "arm64"} {
		want := "MACOSX_DEPLOYMENT_TARGET=13.0 CGO_ENABLED=1 GOOS=darwin GOARCH=" + arch + " go build"
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow missing cgo-enabled macOS %s build: %q", arch, want)
		}
	}
}

func TestReleaseWorkflowDisablesCGOForNonDarwinBuilds(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	workflow := string(data)
	for _, target := range []string{
		"GOOS=linux GOARCH=amd64",
		"GOOS=linux GOARCH=arm64",
		"GOOS=windows GOARCH=amd64",
	} {
		want := "CGO_ENABLED=0 " + target + " go build"
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow missing Cgo-disabled %s build: %q", target, want)
		}
	}
}

func TestReleaseWorkflowDoesNotInterpolateDispatchInputIntoShell(t *testing.T) {
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	releaseStart := strings.Index(workflow, "\n  macos:\n")
	if releaseStart == -1 {
		t.Fatal("release workflow missing macos job")
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
	data, err := readReleaseWorkflow()
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
		`cache: false`,
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
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
	for _, publisher := range []string{"publish", "homebrew", "winget"} {
		if got := strings.Count(releaseWorkflowJobBlock(t, workflow, publisher), "actions/download-artifact@"); got != 1 {
			t.Fatalf("%s must consume one qualified artifact, got %d downloads", publisher, got)
		}
	}
}

func TestReleaseWorkflowUsesValidSecureReleaseSteps(t *testing.T) {
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
		`if: needs.resolve.outputs.reused != 'true'`,
		`if: steps.published_artifact.outputs.reused != 'true'`,
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("release workflow missing rerun artifact contract %q", want)
		}
	}
}

func TestReleaseWorkflowRepairsPackageManagersWithoutRebuildingPublishedRelease(t *testing.T) {
	data, err := readReleaseWorkflow()
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

func TestReleaseWorkflowPublishesPackageManagersWhenBuildIsSkipped(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	const publisherCondition = "\n    if: always() && needs.publish.result == 'success'\n"
	const publisherNeeds = "\n    needs: publish\n"

	for _, jobName := range []string{"homebrew", "winget"} {
		jobBlock := releaseWorkflowJobBlock(t, workflow, jobName)
		// publish escapes the skipped build with always(); GitHub propagates that skip
		// down the whole needs chain, so every publisher must break the chain the same
		// way or the already-published repair path silently ships nothing.
		if !strings.Contains(jobBlock, publisherCondition) {
			t.Errorf("%s job must contain the exact publisher condition %q", jobName, strings.TrimSpace(publisherCondition))
		}
		if !strings.Contains(jobBlock, publisherNeeds) {
			t.Errorf("%s job must depend on publish", jobName)
		}
	}
}

func TestVerifyReleaseAssetsRequiresExactChecksumCoverage(t *testing.T) {
	workflowData, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	for _, boundary := range []struct {
		name  string
		count int
	}{
		{"repair-notarization", 1}, {"build", 1}, {"publish", 3}, {"homebrew", 1}, {"winget", 1},
	} {
		job := releaseWorkflowJobBlock(t, string(workflowData), boundary.name)
		if got := strings.Count(job, "verify_release_assets.py"); got != boundary.count {
			t.Fatalf("%s must verify exact coverage at every consumer boundary: want %d, got %d", boundary.name, boundary.count, got)
		}
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
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
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
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	for _, unwanted := range []string{
		"--force-with-lease",
		"git push --force",
		`git merge-base --is-ancestor origin/master "origin/${BRANCH}"`,
		`git checkout -b "${BRANCH}" upstream/master`,
		`git checkout -B "${BRANCH}" upstream/master`,
		`grep -Ev "^manifests/r/Rorkai/ASC/${VERSION}/"`,
	} {
		if strings.Contains(workflow, unwanted) {
			t.Errorf("WinGet publication must not rewrite branch history; found %q", unwanted)
		}
	}
	for _, want := range []string{
		`git merge-base --is-ancestor origin/master upstream/master`,
		`retry_transient git checkout -B "${BRANCH}" origin/master`,
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

func TestReleaseWorkflowSignsMacOSBinariesWithStableCodeSigningIdentifier(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	workflow := string(data)
	signStart := strings.Index(workflow, "- name: Sign and verify macOS binaries")
	notarizeStart := strings.Index(workflow, "- name: Notarize macOS binaries")
	if signStart == -1 || notarizeStart == -1 || signStart >= notarizeStart {
		t.Fatal("release workflow must sign macOS binaries before notarizing them")
	}
	signStep := workflow[signStart:notarizeStart]

	// codesign derives the identifier from the file name unless one is given,
	// so unnamed signing gives every release a version-specific designated
	// requirement. macOS keychain ACLs trust applications by that requirement,
	// which re-prompts for stored credentials after every upgrade.
	for _, want := range []string{
		`ASC_CODESIGN_IDENTIFIER: com.rorkai.asc`,
		`--identifier "${ASC_CODESIGN_IDENTIFIER}"`,
		`sed -n 's/^Identifier=//p'`,
		`unexpected code signing identifier`,
	} {
		if !strings.Contains(signStep, want) {
			t.Errorf("sign step missing stable identifier contract %q", want)
		}
	}
	for _, line := range strings.Split(signStep, "\n") {
		if !strings.Contains(line, "codesign --force --sign") {
			continue
		}
		if !strings.Contains(line, `--identifier "${ASC_CODESIGN_IDENTIFIER}"`) {
			t.Errorf("codesign invocation must pin the identifier: %s", strings.TrimSpace(line))
		}
	}
}

func TestReleaseWorkflowHomebrewRequiresVentura(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatal(err)
	}
	job := releaseWorkflowJobBlock(t, string(data), "homebrew")
	if !strings.Contains(job, "depends_on macos: :ventura") || strings.Contains(job, "depends_on :macos") {
		t.Fatal("Go 1.27 release formula must require macOS Ventura or later")
	}
}

func validateReleaseFanout(data []byte) error {
	var workflow struct {
		Jobs map[string]struct {
			Needs   any               `yaml:"needs"`
			If      string            `yaml:"if"`
			RunsOn  string            `yaml:"runs-on"`
			Outputs map[string]string `yaml:"outputs"`
			Steps   []struct {
				Uses string         `yaml:"uses"`
				With map[string]any `yaml:"with"`
				Run  string         `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return err
	}
	resolve, ok := workflow.Jobs["resolve"]
	if !ok || resolve.Outputs["sha"] != "${{ steps.source.outputs.sha }}" || resolve.Outputs["reused"] != "${{ steps.candidate_artifact.outputs.reused }}" {
		return fmt.Errorf("missing frozen source/reuse outputs")
	}
	freshCondition := "needs.prepare.outputs.published != 'true' && needs.resolve.outputs.reused != 'true'"
	for _, name := range []string{"quality", "quality-tests", "macos", "portable"} {
		job, ok := workflow.Jobs[name]
		if !ok || job.If != freshCondition {
			return fmt.Errorf("%s must run only for fresh candidates", name)
		}
		if fmt.Sprint(job.Needs) != "[prepare resolve]" {
			return fmt.Errorf("%s must depend on source resolver", name)
		}
		runner := "macos-latest"
		if name == "portable" {
			runner = "ubuntu-latest"
		}
		if job.RunsOn != runner {
			return fmt.Errorf("%s has wrong platform", name)
		}
		checkout, cleanGo := false, false
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout@") && fmt.Sprint(step.With["ref"]) == "${{ needs.resolve.outputs.sha }}" {
				checkout = true
			}
			if strings.HasPrefix(step.Uses, "actions/setup-go@") && fmt.Sprint(step.With["cache"]) == "false" {
				cleanGo = true
			}
			if strings.HasPrefix(step.Uses, "actions/cache@") || strings.Contains(fmt.Sprint(step.With["name"]), "candidate-release-") {
				return fmt.Errorf("%s exposes unqualified candidate or shared cache", name)
			}
		}
		if !checkout || !cleanGo {
			return fmt.Errorf("%s must compile exact source without shared cache", name)
		}
	}
	build := workflow.Jobs["build"]
	tools, join := false, false
	for _, step := range build.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") && fmt.Sprint(step.With["ref"]) == "${{ github.workflow_sha }}" && fmt.Sprint(step.With["path"]) == "workflow-source" {
			tools = true
		}
		if strings.Contains(step.Run, "python3 workflow-source/scripts/assemble_release_candidate.py") && strings.Contains(step.Run, `--expected-sha "$RELEASE_SHA"`) && strings.Contains(step.Run, "python3 workflow-source/scripts/verify_release_assets.py") {
			join = true
		}
	}
	if !tools || !join {
		return fmt.Errorf("assembly must use current workflow tools and validate exact-source canonical assets")
	}
	if fmt.Sprint(build.Needs) != "[prepare resolve quality quality-tests macos portable]" {
		return fmt.Errorf("candidate join missing a fresh gate")
	}
	expected := "always() && needs.prepare.result == 'success' && needs.resolve.result == 'success' && (needs.resolve.outputs.reused == 'true' || (needs.quality.result == 'success' && needs.quality-tests.result == 'success' && needs.macos.result == 'success' && needs.portable.result == 'success'))"
	if build.If != expected {
		return fmt.Errorf("candidate join permits failed gates or blocks qualified reuse")
	}
	return nil
}

func TestReleaseWorkflowFreshFanoutGates(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateReleaseFanout(data); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range [][2]string{
		{"needs.quality.result == 'success'", "needs.quality.result != 'cancelled'"},
		{"needs.quality-tests.result == 'success'", "needs.quality-tests.result != 'cancelled'"},
		{"      - quality-tests\n", ""},
		{"needs.macos.result == 'success'", "needs.macos.result != 'cancelled'"},
		{"needs.portable.result == 'success'", "needs.portable.result != 'cancelled'"},
		{"cache: false", "cache: true"},
		{"      - quality\n", ""},
		{"needs.resolve.outputs.reused != 'true'", "needs.resolve.outputs.reused == 'true'"},
		{"ref: ${{ needs.resolve.outputs.sha }}", "ref: main"},
		{"name: intermediate-macos-", "name: candidate-release-"},
		{"ref: ${{ github.workflow_sha }}", "ref: ${{ needs.resolve.outputs.sha }}"},
		{`--expected-sha "$RELEASE_SHA"`, `--expected-sha "unknown"`},
	} {
		changed := strings.ReplaceAll(string(data), mutation[0], mutation[1])
		if changed == string(data) {
			t.Fatalf("mutation target absent: %s", mutation[0])
		}
		if err := validateReleaseFanout([]byte(changed)); err == nil {
			t.Errorf("unsafe mutation accepted: %s", mutation[0])
		}
	}
}

func TestReleaseCandidateAssembler(t *testing.T) {
	command := exec.Command("python3", "scripts/test_assemble_release_candidate.py")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("assembler tests: %v\n%s", err, output)
	}
}

func TestReleaseWorkflowNativeNotaryUsesReleaseBinary(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatal(err)
	}
	job := releaseWorkflowJobBlock(t, string(data), "macos")
	if strings.Contains(job, "asc-notary-bootstrap") || strings.Count(job, "go build") != 2 {
		t.Fatal("macOS must compile each release architecture exactly once")
	}
	for _, required := range []string{`arm64) arch=arm64`, `x86_64) arch=amd64`, `"$NOTARY_CLI" notarization list`, `"$NOTARY_CLI" notarization submit`, `"$NOTARY_CLI" notarization log`} {
		if !strings.Contains(job, required) {
			t.Errorf("native release notarization missing %q", required)
		}
	}
	build := strings.Index(job, "- name: Build macOS binaries")
	auth := strings.Index(job, "- name: Verify notarization auth")
	sign := strings.Index(job, "- name: Sign and verify macOS binaries")
	notarize := strings.Index(job, "- name: Notarize macOS binaries")
	if build < 0 || auth <= build || sign <= auth || notarize <= sign {
		t.Fatal("native release must build, validate auth, sign, then notarize")
	}
}

func TestReleaseWorkflowRecoversIntermediatesAcrossPartialReruns(t *testing.T) {
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatal(err)
	}
	job := releaseWorkflowJobBlock(t, string(data), "build")
	for _, required := range []string{`actions/runs/${GITHUB_RUN_ID}/artifacts?per_page=100`, `select(.expired == false)`, `sort_by(.created_at, .id)`, `artifact-ids: ${{ steps.intermediates.outputs.macos }}`, `artifact-ids: ${{ steps.intermediates.outputs.portable }}`} {
		if !strings.Contains(job, required) {
			t.Errorf("partial rerun recovery missing %q", required)
		}
	}
	if strings.Contains(job, "name: intermediate-macos-${{ github.run_attempt }}") || strings.Contains(job, "name: intermediate-portable-${{ github.run_attempt }}") {
		t.Fatal("join must reuse prior successful lanes from this run")
	}
}

func releaseWorkflowStepRun(t *testing.T, job, name string) string {
	t.Helper()
	data, err := readReleaseWorkflow()
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	for _, step := range workflow.Jobs[job].Steps {
		if step.Name == name {
			return step.Run
		}
	}
	t.Fatalf("missing %s step %q", job, name)
	return ""
}

func TestReleaseWorkflowMacOSIntermediateOmitsAppleDouble(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires native macOS tar and extended attributes")
	}
	script := releaseWorkflowStepRun(t, "macos", "Pack macos intermediate")
	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	if err := os.Mkdir(release, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"asc_1.2.3_macOS_amd64", "asc_1.2.3_macOS_arm64"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(release, name), []byte("release fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Native tar otherwise serializes this attribute into an extra ._release member.
	if output, err := exec.Command("xattr", "-w", "com.asc.release-test", "fixture", release).CombinedOutput(); err != nil {
		t.Fatalf("set macOS archive attribute: %v %s", err, output)
	}
	stub := "#!/bin/sh\nprintf '%s\\n' " + strings.Repeat("a", 40) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
	command.Dir = dir
	command.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("pack native macOS intermediate: %v %s", err, output)
	}
	// Feed the actual producer's archive into the production validator.
	args := []string{"-c", "import sys; sys.path.insert(0, 'scripts'); from assemble_release_candidate import read_lane; read_lane(sys.argv[1], sys.argv[2:], 'a' * 40)", filepath.Join(dir, "workflow-artifact", "macos.tar")}
	args = append(args, names...)
	if output, err := exec.Command("python3", args...).CombinedOutput(); err != nil {
		t.Fatalf("native intermediate rejected by assembler: %v %s", err, output)
	}
}

func TestReleaseWorkflowIntermediatesResolverBehavior(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release artifact resolver runs in a Unix shell")
	}
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required to execute the release resolver: %v", tool, err)
		}
	}
	script := releaseWorkflowStepRun(t, "build", "Resolve retained intermediates from this run")
	for _, fixture := range []struct {
		name, json, want string
		failure          bool
	}{
		{name: "paginated partial retry", json: `{"artifacts":[
    {"id":11,"name":"intermediate-macos-1","created_at":"2026-10-08T01:00:00Z","expired":false},
    {"id":21,"name":"intermediate-portable-1","created_at":"2026-10-08T01:00:00Z","expired":false},
    {"id":99,"name":"intermediate-macos-3","created_at":"2026-10-08T03:00:00Z","expired":true}]}
   {"artifacts":[
    {"id":22,"name":"intermediate-portable-2","created_at":"2026-10-08T02:00:00Z","expired":false},
    {"id":98,"name":"intermediate-portable-3","created_at":"2026-10-08T03:00:00Z","expired":true},
    {"id":97,"name":"intermediate-macos-unqualified","created_at":"2026-10-08T04:00:00Z","expired":false},
    {"id":96,"name":"intermediate-other-4","created_at":"2026-10-08T04:00:00Z","expired":false},
    {"id":95,"name":"candidate-release-1.2.3","created_at":"2026-10-08T04:00:00Z","expired":false}]}`, want: "macos=11\nportable=22\n"},
		{name: "missing portable fails", json: `{"artifacts":[{"id":11,"name":"intermediate-macos-1","created_at":"2026-10-08T01:00:00Z","expired":false}]}`, failure: true},
		{name: "only expired macos fails", json: `{"artifacts":[{"id":11,"name":"intermediate-macos-1","created_at":"2026-10-08T01:00:00Z","expired":true},{"id":22,"name":"intermediate-portable-2","created_at":"2026-10-08T02:00:00Z","expired":false}]}`, failure: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			jsonPath, outputPath := filepath.Join(dir, "fixture.json"), filepath.Join(dir, "output")
			if err := os.WriteFile(jsonPath, []byte(fixture.json), 0o600); err != nil {
				t.Fatal(err)
			}
			stub := `#!/bin/sh
if [ "$*" != "api --paginate repos/test/repo/actions/runs/123/artifacts?per_page=100" ]; then
 echo "unexpected artifact API request" >&2
 exit 7
fi
cat "$GH_STUB_JSON"
`
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
			command.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_STUB_JSON="+jsonPath, "RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+outputPath, "GITHUB_RUN_ID=123", "GH_REPO=test/repo")
			output, err := command.CombinedOutput()
			if fixture.failure {
				if err == nil || !strings.Contains(string(output), "Missing retained") {
					t.Fatalf("missing lane must fail closed: err=%v output=%s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolver failed: %v %s", err, output)
			}
			got, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != fixture.want {
				t.Fatalf("resolved incorrect lane attempts: want %q got %q", fixture.want, got)
			}
		})
	}
}

func TestReleaseWorkflowSameRunCandidateProvenance(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release artifact resolver runs in a Unix shell")
	}
	for _, tool := range []string{"bash", "jq", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required to execute the release resolver: %v", tool, err)
		}
	}
	script := releaseWorkflowStepRun(t, "resolve", "Check for existing immutable build artifact")
	sha := strings.Repeat("a", 40)
	for _, fixture := range []struct {
		name, commit, failure string
		missingTar, corrupt   bool
		downloadFailure       bool
	}{
		{name: "same source", commit: sha},
		{name: "moved tag", commit: strings.Repeat("b", 40), failure: "does not match release source"},
		{name: "missing provenance", failure: "does not match release source"},
		{name: "missing archive", commit: sha, missingTar: true, failure: "does not match release source"},
		{name: "damaged ZIP", corrupt: true, failure: "BadZipFile"},
		{name: "download fails", downloadFailure: true, failure: "download failed"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			zipPath, outputPath := filepath.Join(dir, "fixture.zip"), filepath.Join(dir, "output")
			archive, err := os.Create(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(archive)
			entries := map[string]string{}
			if !fixture.missingTar {
				entries["candidate-release-1.2.3.tar"] = "retained candidate"
			}
			if fixture.commit != "" {
				entries["candidate-release-1.2.3.commit"] = fixture.commit + "\n"
			}
			for name, contents := range entries {
				entry, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write([]byte(contents)); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if fixture.corrupt {
				if err := os.WriteFile(zipPath, []byte("damaged ZIP"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stub := `#!/bin/sh
case "$*" in
 "api --paginate repos/test/repo/actions/runs/123/artifacts?per_page=100")
  printf '%s\n' '{"artifacts":[{"id":7,"name":"candidate-release-1.2.3","expired":false}]}' ;;
 "api repos/test/repo/actions/artifacts/7/zip")
  if [ "$DOWNLOAD_FAILURE" = true ]; then echo 'download failed' >&2; exit 7; fi
  cat "$GH_STUB_ZIP" ;;
 *) echo "unexpected artifact API request" >&2; exit 8 ;;
esac
`
			for name, contents := range map[string]string{"gh": stub, "git": "#!/bin/sh\nprintf '%s\\n' " + sha + "\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
			command.Dir = dir
			command.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_STUB_ZIP="+zipPath, "RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+outputPath, "GITHUB_RUN_ID=123", "GH_REPO=test/repo", "VERSION=1.2.3", fmt.Sprintf("DOWNLOAD_FAILURE=%t", fixture.downloadFailure))
			output, err := command.CombinedOutput()
			got, _ := os.ReadFile(outputPath)
			if fixture.failure != "" {
				if err == nil || !strings.Contains(string(output), fixture.failure) || strings.Contains(string(got), "reused=true") {
					t.Fatalf("invalid candidate must fail before reuse: err=%v output=%s outputs=%q", err, output, got)
				}
				return
			}
			if err != nil || string(got) != "reused=true\ncross_run=false\n" {
				t.Fatalf("same-source candidate must remain reusable: err=%v output=%s outputs=%q", err, output, got)
			}
		})
	}
}
