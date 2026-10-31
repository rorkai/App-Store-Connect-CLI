package signing

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

const nukeTestPassword = "repository-password"

var (
	nukeDevProfilePath   = "profiles/development/Dev.mobileprovision"
	nukeDev2ProfilePath  = "profiles/development/com.example.app--profile-dev-2.mobileprovision"
	nukeMacProfilePath   = "profiles/development/Mac Dev.provisionprofile"
	nukeStoreProfilePath = "profiles/appstore/Store.mobileprovision"
	nukeDevCertPath      = "certs/development/serial-dev.cer"
	nukeOtherDevCertPath = "certs/development/serial-other.cer"
	nukeDistCertPath     = "certs/distribution/serial-dist.cer"
)

func seedSigningNukeRepository(t *testing.T) (string, string) {
	t.Helper()
	remoteURL, remotePath := newSigningSyncBareRemote(t)
	store := &signingpkg.GitStore{RepoURL: remoteURL, LocalDir: filepath.Join(t.TempDir(), "seed"), Branch: "main"}
	t.Cleanup(func() { _ = store.Cleanup() })
	if err := store.Clone(context.Background(), true); err != nil {
		t.Fatalf("clone seed: %v", err)
	}
	profiles := []struct {
		path, bundleID, profileType, resourceID string
	}{
		{nukeDevProfilePath, "com.example.app", "IOS_APP_DEVELOPMENT", "profile-dev-1"},
		{nukeDev2ProfilePath, "com.example.app", "IOS_APP_DEVELOPMENT", "profile-dev-2"},
		{nukeMacProfilePath, "com.example.mac", "MAC_APP_DEVELOPMENT", "profile-mac"},
		{nukeStoreProfilePath, "com.example.app", "IOS_APP_STORE", "profile-store"},
	}
	for _, profile := range profiles {
		metadata := signingpkg.EncryptedFileMetadata{
			Version:           1,
			Kind:              signingProfileArtifactKind,
			BundleID:          profile.bundleID,
			ProfileType:       profile.profileType,
			ProfileResourceID: profile.resourceID,
		}
		if err := store.WriteEncryptedFileWithMetadata(profile.path, []byte("profile-"+profile.resourceID), nukeTestPassword, metadata); err != nil {
			t.Fatalf("seed %s: %v", profile.path, err)
		}
	}
	for path, content := range map[string]string{
		nukeDevCertPath:      "dev-certificate",
		nukeOtherDevCertPath: "other-dev-certificate",
		nukeDistCertPath:     "dist-certificate",
	} {
		if err := store.WriteEncryptedFile(path, []byte(content), nukeTestPassword); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}
	if err := store.CommitAndPush(context.Background(), "seed signing assets"); err != nil {
		t.Fatalf("push seed: %v", err)
	}
	return remoteURL, remotePath
}

type nukeAPI struct {
	mu          sync.Mutex
	requests    []string
	failDeletes map[string]bool
}

func (api *nukeAPI) server(t *testing.T) *httptest.Server {
	t.Helper()
	devCertificate := base64.StdEncoding.EncodeToString([]byte("dev-certificate"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		api.mu.Lock()
		api.requests = append(api.requests, req.Method+" "+req.URL.Path)
		fail := api.failDeletes[req.URL.Path]
		api.mu.Unlock()
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles":
			if got := req.URL.Query().Get("filter[profileType]"); got != "IOS_APP_DEVELOPMENT" {
				t.Errorf("profiles filter[profileType] = %q", got)
			}
			// A profile of another type is returned to prove client-side scoping.
			signingFetchWriteJSON(t, w, http.StatusOK, `{"data":[
				{"type":"profiles","id":"profile-dev-2","attributes":{"name":"Dev 2","profileType":"IOS_APP_DEVELOPMENT","profileState":"ACTIVE"}},
				{"type":"profiles","id":"profile-mac","attributes":{"name":"Mac Dev","profileType":"MAC_APP_DEVELOPMENT","profileState":"ACTIVE"}},
				{"type":"profiles","id":"profile-dev-1","attributes":{"name":"Dev","profileType":"IOS_APP_DEVELOPMENT","profileState":"INVALID","expirationDate":"2000-01-01T00:00:00Z"}}
			]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			if got := req.URL.Query().Get("filter[certificateType]"); got != "IOS_DEVELOPMENT,DEVELOPMENT" {
				t.Errorf("certificates filter[certificateType] = %q", got)
			}
			signingFetchWriteJSON(t, w, http.StatusOK, fmt.Sprintf(`{"data":[
				{"type":"certificates","id":"cert-dist","attributes":{"serialNumber":"serial-dist","certificateType":"IOS_DISTRIBUTION"}},
				{"type":"certificates","id":"cert-dev","attributes":{"name":"iOS Development","serialNumber":"serial-dev","certificateType":"IOS_DEVELOPMENT","expirationDate":"2100-01-01T00:00:00Z","certificateContent":%q}}
			]}`, devCertificate))
		case req.Method == http.MethodDelete && (strings.HasPrefix(req.URL.Path, "/v1/profiles/") || strings.HasPrefix(req.URL.Path, "/v1/certificates/")):
			if fail {
				signingFetchWriteJSON(t, w, http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"delete failed"}]}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func (api *nukeAPI) mutations() []string {
	api.mu.Lock()
	defer api.mu.Unlock()
	mutations := []string{}
	for _, request := range api.requests {
		if !strings.HasPrefix(request, http.MethodGet+" ") {
			mutations = append(mutations, request)
		}
	}
	return mutations
}

func runSigningSyncNuke(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := syncNukeCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Parse(args); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func remoteTree(t *testing.T, remotePath string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(gitOutput(t, remotePath, "ls-tree", "-r", "--name-only", "main")), "\n")
	sort.Strings(lines)
	return lines
}

func TestSigningSyncNukeValidatesFlagsBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing confirm", []string{"--profile-type", "IOS_APP_DEVELOPMENT", "--repo", "git@github.com:team/certs.git"}, "--confirm is required to delete profiles and revoke certificates (or pass --dry-run to preview)"},
		{"missing profile type", []string{"--repo", "git@github.com:team/certs.git", "--confirm"}, "--profile-type is required"},
		{"unsupported profile type", []string{"--profile-type", "WATCH_APP", "--repo", "git@github.com:team/certs.git", "--confirm"}, "--profile-type must be a supported App Store Connect profile type"},
		{"missing repo", []string{"--profile-type", "IOS_APP_DEVELOPMENT", "--confirm"}, "--repo is required"},
		{"empty branch", []string{"--profile-type", "IOS_APP_DEVELOPMENT", "--repo", "git@github.com:team/certs.git", "--branch", " ", "--confirm"}, "--branch must not be empty"},
		{"unsupported certificate type", []string{"--profile-type", "IOS_APP_DEVELOPMENT", "--repo", "git@github.com:team/certs.git", "--certificate-type", "NOPE", "--confirm"}, "--certificate-type: unsupported certificate type NOPE"},
		{"unexpected argument", []string{"--profile-type", "IOS_APP_DEVELOPMENT", "--repo", "git@github.com:team/certs.git", "--confirm", "extra"}, "unexpected argument(s): extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, nukeTestPassword)
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client must not be created")
			}))
			stdout, stderr, err := runSigningSyncNuke(t, tt.args...)
			if err == nil || !errors.Is(err, flag.ErrHelp) || err.Error() != tt.want {
				t.Fatalf("error = %v, want usage error %q", err, tt.want)
			}
			if clientCalls != 0 || stdout != "" || stderr != "Error: "+tt.want+"\n" {
				t.Fatalf("clientCalls=%d stdout=%q stderr=%q", clientCalls, stdout, stderr)
			}
		})
	}
}

func TestSigningSyncNukeDryRunPerformsZeroWrites(t *testing.T) {
	remoteURL, remotePath := seedSigningNukeRepository(t)
	t.Setenv(signingSyncPasswordEnvVar, nukeTestPassword)
	api := &nukeAPI{}
	server := api.server(t)
	client := newSigningFetchServerTestClient(t, server)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	headBefore := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main"))
	treeBefore := remoteTree(t, remotePath)

	stdout, stderr, err := runSigningSyncNuke(t, "--profile-type", "IOS_APP_DEVELOPMENT", "--repo", remoteURL, "--dry-run", "--output", "json")
	if err != nil {
		t.Fatalf("dry run error: %v\n%s", err, stderr)
	}
	if got := api.mutations(); len(got) != 0 {
		t.Fatalf("dry run App Store Connect mutations = %v, want none", got)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-parse", "main")); got != headBefore {
		t.Fatalf("dry run moved the remote branch from %s to %s", headBefore, got)
	}
	if got := remoteTree(t, remotePath); !reflect.DeepEqual(got, treeBefore) {
		t.Fatalf("dry run changed the remote tree: %v", got)
	}
	if !strings.Contains(stderr, "Dry run: would delete 2 profile(s), revoke 1 certificate(s), and remove 3 encrypted file(s); nothing was changed") {
		t.Fatalf("stderr = %q", stderr)
	}

	var result asc.SigningSyncNukeResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if !result.DryRun || result.Operation != "nuke" || result.ProfileType != "IOS_APP_DEVELOPMENT" || result.PublicationState != "skipped" {
		t.Fatalf("result = %+v", result)
	}
	if got := nukeResourceIDs(result.Profiles.Planned); !reflect.DeepEqual(got, []string{"profile-dev-1", "profile-dev-2"}) {
		t.Fatalf("planned profiles = %v", got)
	}
	if got := nukeResourceIDs(result.Certificates.Planned); !reflect.DeepEqual(got, []string{"cert-dev"}) {
		t.Fatalf("planned certificates = %v", got)
	}
	if want := []string{nukeDevCertPath, nukeDevProfilePath, nukeDev2ProfilePath}; !reflect.DeepEqual(result.RepositoryFiles.Planned, want) {
		t.Fatalf("planned files = %v, want %v", result.RepositoryFiles.Planned, want)
	}
	if len(result.Profiles.Deleted) != 0 || len(result.Certificates.Revoked) != 0 || len(result.RepositoryFiles.Removed) != 0 {
		t.Fatalf("dry run reported completed work: %+v", result)
	}
	if !strings.Contains(stdout, `"deleted":[]`) || !strings.Contains(stdout, `"revoked":[]`) || !strings.Contains(stdout, `"removed":[]`) {
		t.Fatalf("dry run JSON must keep empty completion arrays: %s", stdout)
	}
}

func TestSigningSyncNukeDeletesProfilesThenRevokesCertificatesAndRemovesArtifacts(t *testing.T) {
	remoteURL, remotePath := seedSigningNukeRepository(t)
	t.Setenv(signingSyncPasswordEnvVar, nukeTestPassword)
	api := &nukeAPI{}
	client := newSigningFetchServerTestClient(t, api.server(t))
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	stdout, stderr, err := runSigningSyncNuke(t, "--profile-type", "ios_app_development", "--repo", remoteURL, "--confirm", "--output", "json")
	if err != nil {
		t.Fatalf("nuke error: %v\n%s", err, stderr)
	}
	want := []string{
		"DELETE /v1/profiles/profile-dev-1",
		"DELETE /v1/profiles/profile-dev-2",
		"DELETE /v1/certificates/cert-dev",
	}
	if got := api.mutations(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mutations = %v, want %v", got, want)
	}
	wantTree := []string{"README", nukeDistCertPath + ".enc", nukeOtherDevCertPath + ".enc", nukeStoreProfilePath + ".enc", nukeMacProfilePath + ".enc"}
	sort.Strings(wantTree)
	if got := remoteTree(t, remotePath); !reflect.DeepEqual(got, wantTree) {
		t.Fatalf("remote tree = %v, want %v", got, wantTree)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "rev-list", "--count", "main")); got != "3" {
		t.Fatalf("remote commits = %s, want seed README + seed assets + one nuke commit", got)
	}
	if got := strings.TrimSpace(gitOutput(t, remotePath, "log", "-1", "--format=%s", "main")); got != "Remove IOS_APP_DEVELOPMENT signing assets" {
		t.Fatalf("nuke commit message = %q", got)
	}

	var result asc.SigningSyncNukeResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if result.DryRun || result.Partial || result.PublicationState != "succeeded" {
		t.Fatalf("result = %+v", result)
	}
	if got := nukeResourceIDs(result.Profiles.Deleted); !reflect.DeepEqual(got, []string{"profile-dev-1", "profile-dev-2"}) {
		t.Fatalf("deleted profiles = %v", got)
	}
	if got := nukeResourceIDs(result.Certificates.Revoked); !reflect.DeepEqual(got, []string{"cert-dev"}) {
		t.Fatalf("revoked certificates = %v", got)
	}
	if want := []string{nukeDevCertPath, nukeDevProfilePath, nukeDev2ProfilePath}; !reflect.DeepEqual(result.RepositoryFiles.Removed, want) {
		t.Fatalf("removed files = %v, want %v", result.RepositoryFiles.Removed, want)
	}
}

func TestSigningSyncNukePartialFailureKeepsFailedArtifactsAndExitsNonZero(t *testing.T) {
	remoteURL, remotePath := seedSigningNukeRepository(t)
	t.Setenv(signingSyncPasswordEnvVar, nukeTestPassword)
	api := &nukeAPI{failDeletes: map[string]bool{"/v1/profiles/profile-dev-2": true}}
	client := newSigningFetchServerTestClient(t, api.server(t))
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	stdout, _, err := runSigningSyncNuke(t, "--profile-type", "IOS_APP_DEVELOPMENT", "--repo", remoteURL, "--confirm", "--output", "json")
	if err == nil || errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "failed to delete 1 profile(s) and revoke 0 certificate(s)") {
		t.Fatalf("error = %v, want operational partial-failure error", err)
	}
	want := []string{
		"DELETE /v1/profiles/profile-dev-1",
		"DELETE /v1/profiles/profile-dev-2",
		"DELETE /v1/certificates/cert-dev",
	}
	if got := api.mutations(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mutations = %v, want every planned operation attempted in order %v", got, want)
	}
	tree := strings.Join(remoteTree(t, remotePath), "\n")
	if !strings.Contains(tree, nukeDev2ProfilePath+".enc") || strings.Contains(tree, nukeDevProfilePath+".enc") || strings.Contains(tree, nukeDevCertPath+".enc") {
		t.Fatalf("remote tree = %s, want only the failed profile artifact kept", tree)
	}

	var result asc.SigningSyncNukeResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if !result.Partial || result.PublicationState != "succeeded" {
		t.Fatalf("result = %+v, want partial with published removals", result)
	}
	if len(result.Profiles.Failed) != 1 || result.Profiles.Failed[0].ID != "profile-dev-2" || !strings.Contains(result.Profiles.Failed[0].Error, "delete failed") {
		t.Fatalf("failed profiles = %+v", result.Profiles.Failed)
	}
	if !reflect.DeepEqual(result.RepositoryFiles.Kept, []string{nukeDev2ProfilePath}) {
		t.Fatalf("kept files = %v", result.RepositoryFiles.Kept)
	}
}

func TestPlanSigningNukeFilesKeepsIdentityCoresReferencedByOtherTypes(t *testing.T) {
	certificateDER := []byte("dev-certificate")
	digest := sha256.Sum256(certificateDER)
	fingerprint := strings.ToUpper(hex.EncodeToString(digest[:]))
	otherFingerprint := strings.Repeat("A", 64)
	contextFile := func(path, profileType, resourceID, certificate string) decryptedSigningFile {
		binding := identityContextBinding{CertificateSHA256: certificate, TeamID: "TEAM123", BundleID: "com.example.app", ProfileType: profileType, ProfileResourceID: resourceID}
		data, _ := json.Marshal(binding)
		return decryptedSigningFile{RelativePath: path, Plaintext: data, Metadata: signingpkg.EncryptedFileMetadata{Version: 1, Kind: "identity-context"}}
	}
	core := func(path, certificate string) decryptedSigningFile {
		return decryptedSigningFile{RelativePath: path, Metadata: signingpkg.EncryptedFileMetadata{Version: 1, Kind: "pkcs12-identity", CertificateSHA256: certificate, Sensitive: true}}
	}
	files := []decryptedSigningFile{
		contextFile("identity-contexts/DEV.json", "IOS_APP_DEVELOPMENT", "profile-dev-1", fingerprint),
		contextFile("identity-contexts/MAC.json", "MAC_APP_DEVELOPMENT", "profile-mac", otherFingerprint),
		core("identities/development/"+fingerprint+".p12", fingerprint),
		core("identities/development/"+otherFingerprint+".p12", otherFingerprint),
	}
	plan := signingNukePlan{
		ProfileType: "IOS_APP_DEVELOPMENT",
		ProfileIDs:  map[string]struct{}{"profile-dev-1": {}},
		Certificates: []asc.Resource[asc.CertificateAttributes]{
			{ID: "cert-dev", Attributes: asc.CertificateAttributes{SerialNumber: "serial-dev", CertificateContent: base64.StdEncoding.EncodeToString(certificateDER)}},
		},
	}
	got := planSigningNukeFiles(files, plan)
	want := []string{"identities/development/" + fingerprint + ".p12", "identity-contexts/DEV.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("planned = %v, want %v", got, want)
	}

	// The same core stays when a context of another profile type still
	// references it.
	files = append(files, contextFile("identity-contexts/MAC2.json", "MAC_APP_DEVELOPMENT", "profile-mac-2", fingerprint))
	got = planSigningNukeFiles(files, plan)
	if want := []string{"identity-contexts/DEV.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("planned = %v, want %v", got, want)
	}
}

func TestPlanSigningNukeFilesMatchesEquivalentCertificateSerials(t *testing.T) {
	certificateFile := func(path string) decryptedSigningFile {
		return decryptedSigningFile{RelativePath: path, Plaintext: []byte("stored-" + path)}
	}
	files := []decryptedSigningFile{
		certificateFile("certs/distribution/E41.cer"),
		certificateFile("certs/distribution/00e41.cer"),
		certificateFile("certs/distribution/E410.cer"),
		certificateFile("certs/distribution/0F41.cer"),
	}
	for _, serial := range []string{"0E41", "E41", "e41"} {
		t.Run(serial, func(t *testing.T) {
			plan := signingNukePlan{Certificates: []asc.Resource[asc.CertificateAttributes]{
				{ID: "cert-1", Attributes: asc.CertificateAttributes{SerialNumber: serial}},
			}}
			got := planSigningNukeFiles(files, plan)
			want := []string{"certs/distribution/00e41.cer", "certs/distribution/E41.cer"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("planned = %v, want %v", got, want)
			}
		})
	}
}

func nukeResourceIDs(resources []asc.SigningSyncNukeResource) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.ID)
	}
	return ids
}
