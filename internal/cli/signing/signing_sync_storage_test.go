package signing

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

func runSigningSyncCommandForStorage(t *testing.T, cmd *ffcli.Command, args []string) (string, string, error) {
	t.Helper()
	cmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestSigningSyncStorageFlagsArePresent(t *testing.T) {
	for _, command := range []*ffcli.Command{syncPushCommand(), syncPullCommand()} {
		for _, name := range []string{"storage", "prefix", "region", "gitlab-host", "gitlab-project", "gitlab-token-file"} {
			found := command.FlagSet.Lookup(name)
			if found == nil {
				t.Fatalf("%s: expected --%s flag", command.Name, name)
			}
			if strings.TrimSpace(found.Usage) == "" {
				t.Fatalf("%s: --%s usage is empty", command.Name, name)
			}
		}
		storage := command.FlagSet.Lookup("storage")
		for _, value := range []string{signingSyncStorageGit, signingSyncStorageGitLab, signingSyncStorageAWS} {
			if !strings.Contains(storage.Usage, value) {
				t.Fatalf("%s: --storage usage = %q, want it to name %q", command.Name, storage.Usage, value)
			}
		}
		if storage.DefValue != signingSyncStorageGit {
			t.Fatalf("%s: --storage default = %q, want git", command.Name, storage.DefValue)
		}
	}
}

func TestSigningSyncStorageSelectionRejectsInvalidCombinations(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unsupported storage",
			args: []string{"--storage", "s3-bucket"},
			want: `unsupported --storage "s3-bucket"; use git, gitlab-secure-files, or aws-secrets-manager`,
		},
		{
			name: "git storage still requires repo",
			args: []string{},
			want: "--repo is required",
		},
		{
			name: "repo with gitlab storage",
			args: []string{"--storage", signingSyncStorageGitLab, "--repo", "git@github.com:team/certs.git", "--gitlab-project", "42", "--prefix", "asc-signing", "--gitlab-token-file", "token"},
			want: "--repo is only supported with --storage git, not --storage gitlab-secure-files",
		},
		{
			name: "repo with aws storage",
			args: []string{"--storage", signingSyncStorageAWS, "--repo", "git@github.com:team/certs.git", "--prefix", "asc-signing", "--region", "us-east-1"},
			want: "--repo is only supported with --storage git, not --storage aws-secrets-manager",
		},
		{
			name: "branch with remote storage",
			args: []string{"--storage", signingSyncStorageAWS, "--branch", "release", "--prefix", "asc-signing", "--region", "us-east-1"},
			want: "--branch is only supported with --storage git, not --storage aws-secrets-manager",
		},
		{
			name: "gitlab storage requires project",
			args: []string{"--storage", signingSyncStorageGitLab, "--prefix", "asc-signing", "--gitlab-token-file", "token"},
			want: "--gitlab-project is required with --storage gitlab-secure-files",
		},
		{
			name: "gitlab storage requires prefix",
			args: []string{"--storage", signingSyncStorageGitLab, "--gitlab-project", "42", "--gitlab-token-file", "token"},
			want: "--prefix is required with --storage gitlab-secure-files",
		},
		{
			name: "gitlab storage requires token file",
			args: []string{"--storage", signingSyncStorageGitLab, "--gitlab-project", "42", "--prefix", "asc-signing"},
			want: "--gitlab-token-file is required with --storage gitlab-secure-files",
		},
		{
			name: "gitlab storage rejects region",
			args: []string{"--storage", signingSyncStorageGitLab, "--gitlab-project", "42", "--prefix", "asc-signing", "--gitlab-token-file", "token", "--region", "us-east-1"},
			want: "--region requires --storage aws-secrets-manager",
		},
		{
			name: "aws storage requires prefix",
			args: []string{"--storage", signingSyncStorageAWS, "--region", "us-east-1"},
			want: "--prefix is required with --storage aws-secrets-manager",
		},
		{
			name: "aws storage requires region",
			args: []string{"--storage", signingSyncStorageAWS, "--prefix", "asc-signing"},
			want: "--region is required with --storage aws-secrets-manager",
		},
		{
			name: "aws storage rejects gitlab flags",
			args: []string{"--storage", signingSyncStorageAWS, "--prefix", "asc-signing", "--region", "us-east-1", "--gitlab-project", "42"},
			want: "--gitlab-project requires --storage gitlab-secure-files",
		},
		{
			name: "git storage rejects prefix",
			args: []string{"--repo", "git@github.com:team/certs.git", "--prefix", "asc-signing"},
			want: "--prefix requires --storage gitlab-secure-files or --storage aws-secrets-manager",
		},
		{
			name: "prefix must not traverse",
			args: []string{"--storage", signingSyncStorageAWS, "--prefix", "../escape", "--region", "us-east-1"},
			want: "prefix component \"..\" must start with a letter or digit and use only letters, digits, dot, underscore, or hyphen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, "repository-password")
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client must not be created")
			}))

			stdout, stderr, err := runSigningSyncCommandForStorage(t, syncPullCommand(), tt.args)
			if err == nil {
				t.Fatalf("Run() error = nil, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", err, tt.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want a usage error", err)
			}
			if clientCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientCalls)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want no output", stdout)
			}
			if strings.Contains(stderr, "Cloning signing repo") || strings.Contains(stderr, "Fetching encrypted signing artifacts") {
				t.Fatalf("stderr shows storage side effects: %q", stderr)
			}
		})
	}
}

func TestSigningSyncPushRejectsUnsupportedStorageBeforeSecrets(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client must not be created")
	}))

	_, stderr, err := runSigningSyncCommandForStorage(t, syncPushCommand(), []string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--storage", "vault",
	})
	want := `unsupported --storage "vault"; use git, gitlab-secure-files, or aws-secrets-manager`
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want a usage error", err)
	}
	if clientCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientCalls)
	}
	if stderr != "Error: "+want+"\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSigningSyncRejectsUnprotectedGitLabTokenFile(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	tokenPath := filepath.Join(t.TempDir(), "gitlab-token")
	if err := os.WriteFile(tokenPath, []byte("glpat-token-value\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	_, stderr, err := runSigningSyncCommandForStorage(t, syncPullCommand(), []string{
		"--storage", signingSyncStorageGitLab,
		"--gitlab-project", "42",
		"--prefix", "asc-signing",
		"--gitlab-token-file", tokenPath,
		"--output-dir", filepath.Join(t.TempDir(), "signing"),
	})
	if err == nil {
		t.Fatal("Run() error = nil, want the protected token file check to fail")
	}
	if !strings.Contains(err.Error(), "GitLab token file permissions must be 0600 or more restrictive") {
		t.Fatalf("error = %q, want the protected secret-file failure", err)
	}
	if strings.Contains(err.Error(), "glpat-token-value") || strings.Contains(stderr, "glpat-token-value") {
		t.Fatalf("token leaked: error=%q stderr=%q", err, stderr)
	}
	if strings.Contains(stderr, "Fetching encrypted signing artifacts") {
		t.Fatalf("stderr shows a remote request before validation: %q", stderr)
	}
}

func TestSigningSyncRejectsEmptyGitLabTokenFile(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	tokenPath := filepath.Join(t.TempDir(), "gitlab-token")
	if err := os.WriteFile(tokenPath, []byte("\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	_, _, err := runSigningSyncCommandForStorage(t, syncPullCommand(), []string{
		"--storage", signingSyncStorageGitLab,
		"--gitlab-project", "42",
		"--prefix", "asc-signing",
		"--gitlab-token-file", tokenPath,
	})
	if err == nil || !strings.Contains(err.Error(), "GitLab token file is empty") {
		t.Fatalf("error = %v, want an empty token file failure", err)
	}
}

func TestSigningSyncRotatePasswordRejectsNonGitStorage(t *testing.T) {
	for _, storage := range []string{signingSyncStorageGitLab, signingSyncStorageAWS} {
		t.Run(storage, func(t *testing.T) {
			_, _, err := runSigningSyncCommandForStorage(t, syncRotatePasswordCommand(), []string{
				"--repo", "git@github.com:team/certs.git",
				"--password-file", "current",
				"--new-password-file", "next",
				"--confirm",
				"--storage", storage,
			})
			want := "signing sync rotate-password supports only --storage git"
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %q", err, want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want a usage error", err)
			}
		})
	}
}

func TestSigningSyncStorageTransportLocatorsDescribeBackend(t *testing.T) {
	git := signingSyncGitTransport{repoURL: "https://token:secret@example.com/team/certs.git", branch: "main"}
	if strings.Contains(git.Locator(), "secret") {
		t.Fatalf("git locator leaks credentials: %q", git.Locator())
	}
	if git.Kind() != signingSyncStorageGit {
		t.Fatalf("git transport kind = %q, want git", git.Kind())
	}

	selection := signingSyncStorageSelection{
		kind:   signingSyncStorageAWS,
		prefix: "asc-signing",
		region: "us-east-1",
	}
	transport, err := selection.transport(context.Background())
	if err != nil {
		t.Fatalf("transport() error: %v", err)
	}
	if transport.Kind() != signingSyncStorageAWS {
		t.Fatalf("transport kind = %q, want aws-secrets-manager", transport.Kind())
	}
	locator := transport.Locator()
	if !strings.Contains(locator, "us-east-1") || !strings.Contains(locator, "asc-signing") {
		t.Fatalf("locator = %q, want the region and prefix", locator)
	}
}

// fakeGitLabSecureFilesHandler is the minimum GitLab Secure Files surface the
// transport uses, so the command-side decrypt path can be exercised end to end
// without dialing GitLab.
func fakeGitLabSecureFilesHandler(t *testing.T) http.Handler {
	t.Helper()
	type storedFile struct {
		id      int64
		name    string
		content []byte
	}
	var (
		mu     sync.Mutex
		files  = map[string]storedFile{}
		nextID = int64(1)
	)
	base := "/api/v4/projects/42/secure_files"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") == "" {
			t.Error("request without PRIVATE-TOKEN header")
		}
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			payload := make([]map[string]any, 0, len(files))
			for _, file := range files {
				payload = append(payload, map[string]any{"id": file.id, "name": file.name})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodPost && r.URL.Path == base:
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			upload, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "missing file", http.StatusBadRequest)
				return
			}
			defer upload.Close()
			content, err := io.ReadAll(upload)
			if err != nil {
				http.Error(w, "unreadable file", http.StatusBadRequest)
				return
			}
			name := r.FormValue("name")
			files[name] = storedFile{id: nextID, name: name, content: content}
			nextID++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": files[name].id, "name": name})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/download"):
			rawID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, base+"/"), "/download")
			for _, file := range files {
				if strconv.FormatInt(file.id, 10) == rawID {
					w.Header().Set("Content-Type", "application/octet-stream")
					_, _ = w.Write(file.content)
					return
				}
			}
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

func TestSigningSyncGitLabTransportRoundTripsIntoDecryptedOutput(t *testing.T) {
	server := httptest.NewTLSServer(fakeGitLabSecureFilesHandler(t))
	t.Cleanup(server.Close)

	backend, err := signingpkg.NewGitLabSecureFilesStore(signingpkg.GitLabSecureFilesOptions{
		Host:       server.URL,
		ProjectID:  "42",
		Prefix:     "asc-signing",
		Token:      "glpat-token-value",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewGitLabSecureFilesStore() error: %v", err)
	}
	transport := signingSyncRemoteTransport{kind: signingSyncStorageGitLab, label: "GitLab Secure Files", backend: backend}

	password := "repository-password"
	certificate := []byte("certificate-plaintext")
	source := newSigningSyncStore(transport, t.TempDir(), "", "")
	t.Cleanup(func() { _ = source.Cleanup() })
	relPath := filepath.Join("certs", "distribution", "serial.cer")
	if err := source.WriteEncryptedFile(relPath, certificate, password); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := transport.Publish(context.Background(), source, "Update signing assets"); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	destination := newSigningSyncStore(transport, t.TempDir(), "", "")
	t.Cleanup(func() { _ = destination.Cleanup() })
	if err := transport.Fetch(context.Background(), destination, false); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	encryptedFiles, err := destination.ListEncryptedFiles()
	if err != nil {
		t.Fatalf("ListEncryptedFiles() error: %v", err)
	}
	if len(encryptedFiles) != 1 || filepath.ToSlash(encryptedFiles[0]) != "certs/distribution/serial.cer" {
		t.Fatalf("fetched files = %v, want the published artifact", encryptedFiles)
	}

	decrypted, err := prepareDecryptedSigningFiles(destination, encryptedFiles, password, t.TempDir())
	if err != nil {
		t.Fatalf("prepareDecryptedSigningFiles() error: %v", err)
	}
	if len(decrypted) != 1 || string(decrypted[0].Plaintext) != string(certificate) {
		t.Fatalf("decrypted files = %+v, want the original plaintext", decrypted)
	}
}

func TestNewSigningSyncStoreOmitsRepositoryForRemoteStorage(t *testing.T) {
	remote := signingSyncRemoteTransport{kind: signingSyncStorageGitLab, label: "GitLab Secure Files"}
	store := newSigningSyncStore(remote, t.TempDir(), "git@github.com:team/certs.git", "main")
	if store.RepoURL != "" || store.Branch != "" {
		t.Fatalf("remote store = %+v, want no git repository selectors", store)
	}

	gitStore := newSigningSyncStore(signingSyncGitTransport{}, t.TempDir(), "git@github.com:team/certs.git", "release")
	if gitStore.RepoURL != "git@github.com:team/certs.git" || gitStore.Branch != "release" {
		t.Fatalf("git store = %+v, want the repository and branch preserved", gitStore)
	}
}
