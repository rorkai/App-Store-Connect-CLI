package signing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

// lifecycleProfileState is the mutable App Store Connect state shared by the
// git-backed lifecycle tests.
type lifecycleProfileState struct {
	id         string
	name       string
	profile    string
	expiration string
	devices    []string
	// noCertificates makes the team certificate list empty.
	noCertificates bool
	// otherBundle, when set, is a second registered bundle ID. It has no
	// profiles unless otherProfileCurrent gives it an active profile whose
	// devices already match the enabled device list.
	otherBundle         string
	otherProfileCurrent bool
}

func lifecycleStateAPI(t *testing.T, state *lifecycleProfileState, enabledDevices []string, events *[]string) *asc.Client {
	t.Helper()
	certificate := base64.StdEncoding.EncodeToString([]byte("lifecycle-certificate"))
	return newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		*events = append(*events, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds" && state.otherBundle != "" && req.URL.Query().Get("filter[identifier]") == state.otherBundle:
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"bundleIds","id":"bundle-2","attributes":{"identifier":%q}}]}`, state.otherBundle))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-2/profiles":
			if !state.otherProfileCurrent {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"profiles","id":"profile-other","attributes":{"name":"Other","profileType":%q,"profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z","profileContent":%q}}]}`,
				state.profile, base64.StdEncoding.EncodeToString([]byte("profile-profile-other"))))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles/profile-other/devices":
			items := make([]string, 0, len(enabledDevices))
			for _, id := range enabledDevices {
				items = append(items, fmt.Sprintf(`{"type":"devices","id":%q,"attributes":{"deviceClass":"IPHONE","status":"ENABLED"}}`, id))
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[`+strings.Join(items, ",")+`]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1/profiles":
			if state.id == "" {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"profiles","id":%q,"attributes":{"name":%q,"profileType":%q,"profileState":"ACTIVE","expirationDate":%q,"profileContent":%q}}]}`,
				state.id, state.name, state.profile, state.expiration, base64.StdEncoding.EncodeToString([]byte("profile-"+state.id))))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates" && state.noCertificates:
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/certificates"):
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"serialNumber":"serial-1","certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z","certificateContent":%q}}]}`, certificate))
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/devices") && strings.HasPrefix(req.URL.Path, "/v1/profiles/"):
			items := make([]string, 0, len(state.devices))
			for _, id := range state.devices {
				items = append(items, fmt.Sprintf(`{"type":"devices","id":%q,"attributes":{"deviceClass":"IPHONE","status":"ENABLED"}}`, id))
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[`+strings.Join(items, ",")+`]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/devices":
			items := make([]string, 0, len(enabledDevices))
			for _, id := range enabledDevices {
				items = append(items, fmt.Sprintf(`{"type":"devices","id":%q,"attributes":{"deviceClass":"IPHONE","status":"ENABLED"}}`, id))
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[`+strings.Join(items, ",")+`]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/profiles/"+state.id:
			state.id = ""
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			var payload asc.ProfileCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			if state.id != "" {
				t.Fatalf("profile created before the replaced profile %s was deleted", state.id)
			}
			state.id = "profile-new"
			state.name = payload.Data.Attributes.Name
			state.expiration = "2100-01-01T00:00:00Z"
			state.devices = createDeviceIDs(t, payload)
			return signingFetchJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"profiles","id":"profile-new","attributes":{"name":%q,"profileType":%q,"profileState":"ACTIVE","expirationDate":%q,"profileContent":%q}}}`,
				state.name, state.profile, state.expiration, base64.StdEncoding.EncodeToString([]byte("profile-profile-new"))))
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
}

func mutationEvents(events []string) []string {
	mutations := []string{}
	for _, event := range events {
		if !strings.HasPrefix(event, http.MethodGet+" ") {
			mutations = append(mutations, event)
		}
	}
	return mutations
}

func TestSigningSyncPushRenewExpiredReplacesProfileAndRepositoryArtifact(t *testing.T) {
	remoteURL, remotePath := newSigningSyncBareRemote(t)
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	state := &lifecycleProfileState{id: "profile-old", name: "Store", profile: "IOS_APP_STORE", expiration: "2100-01-01T00:00:00Z"}
	var events []string
	client := lifecycleStateAPI(t, state, nil, &events)

	run := func(args ...string) (string, error) {
		d, depsStdout, _ := newSigningTestDeps()
		d.Client = func() (*asc.Client, error) { return client, nil }
		cmd := syncPushCommand(d)
		cmd.FlagSet.SetOutput(io.Discard)
		var runErr error
		if err := cmd.Parse(append([]string{"--bundle-id", "com.example.app", "--profile-type", "IOS_APP_STORE", "--repo", remoteURL, "--output", "json"}, args...)); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = cmd.Run(context.Background())
		stdout := depsStdout.String()
		return stdout, runErr
	}

	if _, err := run(); err != nil {
		t.Fatalf("seed push: %v", err)
	}
	profilePath := filepath.Join("profiles", "appstore", "Store.mobileprovision")
	assertStoredProfileResource(t, remotePath, profilePath, "profile-old")

	state.expiration = "2000-01-01T00:00:00Z"
	events = nil
	stdout, err := run("--renew-expired")
	if err != nil {
		t.Fatalf("renew push: %v", err)
	}
	if want := []string{"DELETE /v1/profiles/profile-old", "POST /v1/profiles"}; !reflect.DeepEqual(mutationEvents(events), want) {
		t.Fatalf("mutations = %v, want %v", mutationEvents(events), want)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	lifecycle, _ := result["profileLifecycle"].(map[string]any)
	if lifecycle["action"] != "renewed" || lifecycle["replacedProfileId"] != "profile-old" || lifecycle["profileId"] != "profile-new" || lifecycle["replacedProfileState"] != "deleted" {
		t.Fatalf("profileLifecycle = %v", lifecycle)
	}
	assertStoredProfileResource(t, remotePath, profilePath, "profile-new")
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-list", "--count", "main")); got != "3" {
		t.Fatalf("remote commits = %s, want seed + push + renewal", got)
	}

	events = nil
	stdout, err = run("--renew-expired")
	if err != nil {
		t.Fatalf("second renew push: %v", err)
	}
	if got := mutationEvents(events); len(got) != 0 {
		t.Fatalf("mutations = %v, want none once the profile is current", got)
	}
	if !strings.Contains(stdout, `"profileLifecycle":{"action":"unchanged","profileId":"profile-new"}`) {
		t.Fatalf("second output = %s", stdout)
	}
	assertStoredProfileResource(t, remotePath, profilePath, "profile-new")
}

func TestRunSigningSyncBatchForceForNewDevicesReplacesTargetArtifact(t *testing.T) {
	remoteURL, remotePath := newSigningSyncBareRemote(t)
	state := &lifecycleProfileState{id: "profile-old", name: "Ad Hoc", profile: "IOS_APP_ADHOC", expiration: "2100-01-01T00:00:00Z", devices: []string{"device-1"}}
	var events []string
	client := lifecycleStateAPI(t, state, []string{"device-2", "device-1"}, &events)
	options := signingSyncBatchOptions{
		RepoURL:     remoteURL,
		Password:    "repository-password",
		ProfileType: "IOS_APP_ADHOC",
		BundleIDs:   []string{"com.example.app"},
	}
	var err error
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	oldPath := signingSyncBatchProfilePath("com.example.app", "IOS_APP_ADHOC", "profile-old")
	newPath := signingSyncBatchProfilePath("com.example.app", "IOS_APP_ADHOC", "profile-new")

	options.ForceForNewDevices = true
	events = nil
	var result SyncResult
	stderrDeps, _, stderrDepsStderr := newSigningTestDeps()
	_, _ = captureOutput(t, func() { result, err = runSigningSyncBatch(context.Background(), stderrDeps, client, options) })
	stderr := stderrDepsStderr.String()
	if err != nil {
		t.Fatalf("refresh batch: %v", err)
	}
	if want := []string{"DELETE /v1/profiles/profile-old", "POST /v1/profiles"}; !reflect.DeepEqual(mutationEvents(events), want) {
		t.Fatalf("mutations = %v, want %v", mutationEvents(events), want)
	}
	if !reflect.DeepEqual(state.devices, []string{"device-1", "device-2"}) || state.name != "Ad Hoc" {
		t.Fatalf("recreated profile name=%q devices=%v", state.name, state.devices)
	}
	want := &asc.SigningSyncProfileLifecycle{
		Action:               asc.SigningSyncLifecycleDevicesRefreshed,
		ProfileID:            "profile-new",
		ReplacedProfileID:    "profile-old",
		ReplacedProfileName:  "Ad Hoc",
		ReplacedProfileState: asc.SigningSyncReplacedProfileDeleted,
		DevicesAdded:         []string{"device-2"},
	}
	if len(result.Targets) != 1 || !reflect.DeepEqual(result.Targets[0].ProfileLifecycle, want) {
		t.Fatalf("target lifecycle = %+v, want %+v", result.Targets, want)
	}
	if !strings.Contains(stderr, "Removed "+oldPath) {
		t.Fatalf("stderr = %q, want removal diagnostic", stderr)
	}
	tree := gitOutput(t, remotePath, "ls-tree", "-r", "--name-only", "main")
	if strings.Contains(tree, filepath.ToSlash(oldPath)+".enc") || !strings.Contains(tree, filepath.ToSlash(newPath)+".enc") {
		t.Fatalf("remote tree after refresh:\n%s", tree)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-list", "--count", "main")); got != "3" {
		t.Fatalf("remote commits = %s, want seed + push + refresh", got)
	}

	events = nil
	_, _ = captureOutput(t, func() { result, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err != nil {
		t.Fatalf("unchanged batch: %v", err)
	}
	if got := mutationEvents(events); len(got) != 0 {
		t.Fatalf("mutations = %v, want none when devices match", got)
	}
	if result.Targets[0].ProfileLifecycle == nil || result.Targets[0].ProfileLifecycle.Action != asc.SigningSyncLifecycleUnchanged {
		t.Fatalf("lifecycle = %+v, want unchanged", result.Targets[0].ProfileLifecycle)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-list", "--count", "main")); got != "3" {
		t.Fatalf("remote commits = %s, want unchanged", got)
	}
}

func assertStoredProfileResource(t *testing.T, remotePath, profilePath, wantID string) {
	t.Helper()
	checkout := filepath.Join(t.TempDir(), "checkout")
	runGitCommand(t, filepath.Dir(checkout), "clone", remotePath, checkout)
	store := &signingpkg.GitStore{LocalDir: checkout}
	_, metadata, err := store.ReadEncryptedFileWithMetadata(profilePath, "repository-password")
	if err != nil {
		t.Fatalf("read %s: %v", profilePath, err)
	}
	if metadata.ProfileResourceID != wantID {
		t.Fatalf("stored profile resource = %q, want %q", metadata.ProfileResourceID, wantID)
	}
}

func TestRunSigningSyncBatchLifecyclePreflightsEveryTargetBeforeReplacing(t *testing.T) {
	remoteURL, remotePath := newSigningSyncBareRemote(t)
	state := &lifecycleProfileState{id: "profile-old", name: "Ad Hoc", profile: "IOS_APP_ADHOC", expiration: "2100-01-01T00:00:00Z", devices: []string{"device-1"}}
	var events []string
	client := lifecycleStateAPI(t, state, []string{"device-2", "device-1"}, &events)
	options := signingSyncBatchOptions{
		RepoURL:     remoteURL,
		Password:    "repository-password",
		ProfileType: "IOS_APP_ADHOC",
		BundleIDs:   []string{"com.example.app"},
	}
	var err error
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	headBefore := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main"))

	// The first target needs a device refresh; the second cannot be resolved.
	options.BundleIDs = []string{"com.example.missing", "com.example.app"}
	options.ForceForNewDevices = true
	events = nil
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err == nil || !strings.Contains(err.Error(), "com.example.missing") {
		t.Fatalf("error = %v, want the unresolved target", err)
	}
	if got := mutationEvents(events); len(got) != 0 {
		t.Fatalf("mutations = %v, want none when a later target fails preflight", got)
	}
	if state.id != "profile-old" {
		t.Fatalf("profile replaced before every target was preflighted: %+v", state)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main")); got != headBefore {
		t.Fatalf("remote moved from %s to %s", headBefore, got)
	}
}

func TestRunSigningSyncBatchLifecycleRejectsCertificateCreationBeforeMutating(t *testing.T) {
	remoteURL, remotePath := newSigningSyncBareRemote(t)
	state := &lifecycleProfileState{id: "profile-old", name: "Store", profile: "IOS_APP_STORE", expiration: "2100-01-01T00:00:00Z"}
	var events []string
	client := lifecycleStateAPI(t, state, nil, &events)
	options := signingSyncBatchOptions{
		RepoURL:     remoteURL,
		Password:    "repository-password",
		ProfileType: "IOS_APP_STORE",
		BundleIDs:   []string{"com.example.app"},
	}
	var err error
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	headBefore := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main"))

	state.expiration = "2000-01-01T00:00:00Z"
	state.noCertificates = true
	state.otherBundle = "com.example.other"
	options.BundleIDs = []string{"com.example.app", "com.example.other"}
	options.RenewExpired = true
	options.CreateMissing = true
	options.CreateMissingCertificate = true
	options.IdentityPassword = []byte("identity-password")
	events = nil
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err == nil || !strings.Contains(err.Error(), "cannot create a certificate; create it with a single-target push first") {
		t.Fatalf("error = %v, want the certificate-creation refusal", err)
	}
	if got := mutationEvents(events); len(got) != 0 {
		t.Fatalf("mutations = %v, want none", got)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main")); got != headBefore {
		t.Fatalf("remote moved from %s to %s", headBefore, got)
	}
}

func TestRunSigningSyncBatchLifecyclePreflightsUnchangedTargetArtifactsBeforeReplacing(t *testing.T) {
	remoteURL, remotePath := newSigningSyncBareRemote(t)
	state := &lifecycleProfileState{id: "profile-old", name: "Ad Hoc", profile: "IOS_APP_ADHOC", expiration: "2100-01-01T00:00:00Z", devices: []string{"device-1"}}
	var events []string
	client := lifecycleStateAPI(t, state, []string{"device-2", "device-1"}, &events)
	options := signingSyncBatchOptions{
		RepoURL:     remoteURL,
		Password:    "repository-password",
		ProfileType: "IOS_APP_ADHOC",
		BundleIDs:   []string{"com.example.app"},
	}
	var err error
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err != nil {
		t.Fatalf("seed batch: %v", err)
	}

	// The unchanged target's destination holds an artifact the repository
	// password cannot authenticate, so its publication preflight fails.
	seed := &signingpkg.GitStore{RepoURL: remoteURL, LocalDir: filepath.Join(t.TempDir(), "seed"), Branch: "main"}
	t.Cleanup(func() { _ = seed.Cleanup() })
	if err := seed.Clone(context.Background(), false); err != nil {
		t.Fatalf("clone: %v", err)
	}
	otherPath := signingSyncBatchProfilePath("com.example.other", "IOS_APP_ADHOC", "profile-other")
	if err := seed.WriteEncryptedFile(otherPath, []byte("foreign"), "another-password"); err != nil {
		t.Fatalf("write foreign artifact: %v", err)
	}
	if err := seed.CommitAndPush(context.Background(), "foreign artifact"); err != nil {
		t.Fatalf("push foreign artifact: %v", err)
	}
	headBefore := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main"))

	state.otherBundle = "com.example.other"
	state.otherProfileCurrent = true
	options.BundleIDs = []string{"com.example.app", "com.example.other"}
	options.ForceForNewDevices = true
	events = nil
	_, _ = captureOutput(t, func() { _, err = runSigningSyncBatch(context.Background(), testSigningDeps(), client, options) })
	if err == nil || !strings.Contains(err.Error(), filepath.ToSlash(otherPath)) {
		t.Fatalf("error = %v, want the unchanged target's preflight failure", err)
	}
	if got := mutationEvents(events); len(got) != 0 {
		t.Fatalf("mutations = %v, want none", got)
	}
	if state.id != "profile-old" {
		t.Fatalf("profile replaced before the unchanged target was preflighted: %+v", state)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main")); got != headBefore {
		t.Fatalf("remote moved from %s to %s", headBefore, got)
	}
}
