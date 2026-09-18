package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	gitLabTestProject  = "42"
	gitLabTestPrefix   = "asc-signing"
	gitLabTestToken    = "glpat-super-secret-token"
	gitLabTestPassword = "repository-password"
)

type fakeGitLabSecureFile struct {
	id      int64
	name    string
	content []byte
}

type fakeGitLabServer struct {
	mu         sync.Mutex
	files      map[string]fakeGitLabSecureFile
	nextID     int64
	tokens     []string
	deletes    []string
	creates    []string
	downloads  []string
	failCreate func(name string, content []byte) bool
}

func newFakeGitLabServer(t *testing.T) (*fakeGitLabServer, *httptest.Server) {
	t.Helper()
	fake := &fakeGitLabServer{files: make(map[string]fakeGitLabSecureFile), nextID: 1}
	server := httptest.NewTLSServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)
	return fake, server
}

func (f *fakeGitLabServer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.tokens = append(f.tokens, r.Header.Get("PRIVATE-TOKEN"))
	f.mu.Unlock()

	base := "/api/v4/projects/" + gitLabTestProject + "/secure_files"
	switch {
	case r.Method == http.MethodGet && r.URL.Path == base:
		f.list(w)
	case r.Method == http.MethodPost && r.URL.Path == base:
		f.create(w, r)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/download"):
		f.download(w, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, base+"/"), "/download"))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, base+"/"):
		f.delete(w, strings.TrimPrefix(r.URL.Path, base+"/"))
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (f *fakeGitLabServer) list(w http.ResponseWriter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.files))
	for name := range f.files {
		names = append(names, name)
	}
	sort.Strings(names)
	payload := make([]map[string]any, 0, len(names))
	for _, name := range names {
		file := f.files[name]
		digest := sha256.Sum256(file.content)
		payload = append(payload, map[string]any{
			"id":                 file.id,
			"name":               file.name,
			"checksum":           hex.EncodeToString(digest[:]),
			"checksum_algorithm": "sha256",
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (f *fakeGitLabServer) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
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

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCreate != nil && f.failCreate(name, content) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"500 Internal Server Error"}`))
		return
	}
	if _, exists := f.files[name]; exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":{"name":["has already been taken"]}}`))
		return
	}
	file := fakeGitLabSecureFile{id: f.nextID, name: name, content: content}
	f.nextID++
	f.files[name] = file
	f.creates = append(f.creates, name)
	digest := sha256.Sum256(content)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                 file.id,
		"name":               file.name,
		"checksum":           hex.EncodeToString(digest[:]),
		"checksum_algorithm": "sha256",
	})
}

func (f *fakeGitLabServer) download(w http.ResponseWriter, rawID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, _ := strconv.ParseInt(rawID, 10, 64)
	for _, file := range f.files {
		if file.id == id {
			f.downloads = append(f.downloads, file.name)
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(file.content)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (f *fakeGitLabServer) delete(w http.ResponseWriter, rawID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, _ := strconv.ParseInt(rawID, 10, 64)
	for name, file := range f.files {
		if file.id == id {
			delete(f.files, name)
			f.deletes = append(f.deletes, name)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (f *fakeGitLabServer) storedContent(name string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[name]
	return file.content, ok
}

func (f *fakeGitLabServer) put(name string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[name] = fakeGitLabSecureFile{id: f.nextID, name: name, content: content}
	f.nextID++
}

func newGitLabTestStore(t *testing.T, server *httptest.Server) *GitLabSecureFilesStore {
	t.Helper()
	store, err := NewGitLabSecureFilesStore(GitLabSecureFilesOptions{
		Host:       server.URL,
		ProjectID:  gitLabTestProject,
		Prefix:     gitLabTestPrefix,
		Token:      gitLabTestToken,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewGitLabSecureFilesStore() error: %v", err)
	}
	return store
}

func newLocalArtifactStore(t *testing.T) *GitStore {
	t.Helper()
	store := &GitStore{LocalDir: t.TempDir()}
	t.Cleanup(func() { _ = store.Cleanup() })
	return store
}

func TestGitLabSecureFilesRoundTripsEncryptedArtifacts(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	remote := newGitLabTestStore(t, server)

	source := newLocalArtifactStore(t)
	plaintext := []byte("provisioning-profile-plaintext")
	relPath := "profiles/appstore/com.example.app.mobileprovision"
	if err := source.WriteEncryptedFile(relPath, plaintext, gitLabTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}

	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	wantName := gitLabTestPrefix + "/" + relPath + EncryptedArtifactSuffix
	stored, ok := fake.storedContent(wantName)
	if !ok {
		t.Fatalf("secure file %q was not stored", wantName)
	}
	if bytes.Contains(stored, plaintext) {
		t.Fatal("stored secure file contains plaintext signing material")
	}
	local, err := source.ReadEncryptedArtifact(relPath)
	if err != nil {
		t.Fatalf("ReadEncryptedArtifact() error: %v", err)
	}
	if !bytes.Equal(stored, local) {
		t.Fatal("stored secure file does not match the local ciphertext")
	}

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	fetched, err := destination.ReadEncryptedFile(relPath, gitLabTestPassword)
	if err != nil {
		t.Fatalf("ReadEncryptedFile() error: %v", err)
	}
	if !bytes.Equal(fetched, plaintext) {
		t.Fatalf("decrypted artifact = %q, want %q", fetched, plaintext)
	}

	fake.mu.Lock()
	tokens := append([]string(nil), fake.tokens...)
	fake.mu.Unlock()
	if len(tokens) == 0 {
		t.Fatal("expected authenticated requests")
	}
	for _, token := range tokens {
		if token != gitLabTestToken {
			t.Fatalf("PRIVATE-TOKEN header = %q, want the configured token", token)
		}
	}
}

func TestGitLabSecureFilesPublishSkipsUnchangedAndReplacesChangedArtifacts(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	remote := newGitLabTestStore(t, server)

	source := newLocalArtifactStore(t)
	relPath := "certs/distribution/serial.cer"
	if err := source.WriteEncryptedFile(relPath, []byte("certificate-one"), gitLabTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("second Publish() error: %v", err)
	}

	fake.mu.Lock()
	creates := len(fake.creates)
	deletes := len(fake.deletes)
	fake.mu.Unlock()
	if creates != 1 || deletes != 0 {
		t.Fatalf("creates = %d, deletes = %d, want an unchanged artifact to be skipped", creates, deletes)
	}

	if err := source.ReplaceEncryptedFile(relPath, []byte("certificate-two"), gitLabTestPassword); err != nil {
		t.Fatalf("ReplaceEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("third Publish() error: %v", err)
	}

	fake.mu.Lock()
	creates = len(fake.creates)
	deletes = len(fake.deletes)
	fake.mu.Unlock()
	if creates != 2 || deletes != 1 {
		t.Fatalf("creates = %d, deletes = %d, want the changed artifact replaced once", creates, deletes)
	}

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	plaintext, err := destination.ReadEncryptedFile(relPath, gitLabTestPassword)
	if err != nil {
		t.Fatalf("ReadEncryptedFile() error: %v", err)
	}
	if string(plaintext) != "certificate-two" {
		t.Fatalf("decrypted artifact = %q, want the replaced content", plaintext)
	}
}

func TestGitLabSecureFilesRestoresPreviousArtifactWhenReplacementFails(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	remote := newGitLabTestStore(t, server)

	source := newLocalArtifactStore(t)
	relPath := "certs/distribution/serial.cer"
	if err := source.WriteEncryptedFile(relPath, []byte("certificate-one"), gitLabTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	name := gitLabTestPrefix + "/" + relPath + EncryptedArtifactSuffix
	original, ok := fake.storedContent(name)
	if !ok {
		t.Fatalf("secure file %q was not stored", name)
	}

	if err := source.ReplaceEncryptedFile(relPath, []byte("certificate-two"), gitLabTestPassword); err != nil {
		t.Fatalf("ReplaceEncryptedFile() error: %v", err)
	}
	replacement, err := source.ReadEncryptedArtifact(relPath)
	if err != nil {
		t.Fatalf("ReadEncryptedArtifact() error: %v", err)
	}
	fake.mu.Lock()
	fake.failCreate = func(_ string, content []byte) bool {
		return bytes.Equal(content, replacement)
	}
	fake.mu.Unlock()

	err = remote.Publish(context.Background(), source)
	if err == nil {
		t.Fatal("Publish() error = nil, want the failed replacement reported")
	}
	if !strings.Contains(err.Error(), "restored") {
		t.Fatalf("error = %q, want the restoration reported", err)
	}
	restored, ok := fake.storedContent(name)
	if !ok {
		t.Fatal("previous secure file was not restored")
	}
	if !bytes.Equal(restored, original) {
		t.Fatal("restored secure file does not match the previous ciphertext")
	}

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	plaintext, err := destination.ReadEncryptedFile(relPath, gitLabTestPassword)
	if err != nil {
		t.Fatalf("ReadEncryptedFile() error: %v", err)
	}
	if string(plaintext) != "certificate-one" {
		t.Fatalf("decrypted artifact = %q, want the preserved content", plaintext)
	}
}

func TestGitLabSecureFilesFetchIgnoresOtherPrefixes(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	remote := newGitLabTestStore(t, server)
	fake.put("other-team/certs/distribution/foreign.cer"+EncryptedArtifactSuffix, []byte("foreign"))
	fake.put("keystore.jks", []byte("unrelated"))

	destination := newLocalArtifactStore(t)
	if err := remote.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	files, err := destination.ListEncryptedFiles()
	if err != nil {
		t.Fatalf("ListEncryptedFiles() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("fetched files = %v, want only the configured prefix", files)
	}
	fake.mu.Lock()
	downloads := len(fake.downloads)
	fake.mu.Unlock()
	if downloads != 0 {
		t.Fatalf("downloads = %d, want no out-of-prefix download", downloads)
	}
}

func TestGitLabSecureFilesFailsWhenUploadIsStoredUnderAnotherName(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": path.Base(r.FormValue("name"))})
	}))
	t.Cleanup(server.Close)
	remote := newGitLabTestStore(t, server)

	source := newLocalArtifactStore(t)
	if err := source.WriteEncryptedFile("certs/distribution/serial.cer", []byte("certificate"), gitLabTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	err := remote.Publish(context.Background(), source)
	if err == nil {
		t.Fatal("Publish() error = nil, want an error when the artifact is stored under another name")
	}
	if !strings.Contains(err.Error(), "unexpected name") {
		t.Fatalf("Publish() error = %v, want an unexpected-name failure", err)
	}
}

func TestGitLabSecureFilesArtifactLimitCountsOnlyPrefixedFiles(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	store := newGitLabTestStore(t, server)
	for index := range gitLabMaxRemoteArtifacts + 1 {
		fake.put(fmt.Sprintf("other-team/artifact-%d.p12", index), []byte("unrelated"))
	}
	fake.put(gitLabTestPrefix+"/certs/distribution/serial.cer"+EncryptedArtifactSuffix, []byte("ciphertext"))

	destination := newLocalArtifactStore(t)
	if err := store.Fetch(context.Background(), destination); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	files, err := destination.ListEncryptedFiles()
	if err != nil {
		t.Fatalf("ListEncryptedFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("fetched files = %v, want the single prefixed artifact", files)
	}
}

func TestGitLabSecureFilesPublishDoesNotDeleteUnknownFiles(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	remote := newGitLabTestStore(t, server)
	fake.put("keystore.jks", []byte("unrelated"))
	fake.put(gitLabTestPrefix+"/legacy/retired.cer"+EncryptedArtifactSuffix, []byte("retired"))

	source := newLocalArtifactStore(t)
	if err := source.WriteEncryptedFile("certs/distribution/serial.cer", []byte("certificate"), gitLabTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	if err := remote.Publish(context.Background(), source); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	fake.mu.Lock()
	deletes := append([]string(nil), fake.deletes...)
	remaining := len(fake.files)
	fake.mu.Unlock()
	if len(deletes) != 0 {
		t.Fatalf("deletes = %v, want no deletion of files this CLI did not replace", deletes)
	}
	if remaining != 3 {
		t.Fatalf("remaining secure files = %d, want the unrelated files preserved", remaining)
	}
}

func TestGitLabSecureFilesErrorRedactsTokenAndKeepsStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprintf(w, `{"message":"403 Forbidden - token %s is not allowed"}`, gitLabTestToken)
	}))
	t.Cleanup(server.Close)
	remote := newGitLabTestStore(t, server)

	err := remote.Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil, want a failure")
	}
	message := err.Error()
	if strings.Contains(message, gitLabTestToken) {
		t.Fatalf("error leaks the token: %q", message)
	}
	if !strings.Contains(message, "403") {
		t.Fatalf("error = %q, want the HTTP status", message)
	}
	if !strings.Contains(message, "Forbidden") {
		t.Fatalf("error = %q, want the GitLab error title", message)
	}
}

func TestGitLabSecureFilesRejectsNonHTTPSHostBeforeDialing(t *testing.T) {
	dials := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		dials++
	}))
	t.Cleanup(server.Close)

	_, err := NewGitLabSecureFilesStore(GitLabSecureFilesOptions{
		Host:      server.URL,
		ProjectID: gitLabTestProject,
		Prefix:    gitLabTestPrefix,
		Token:     gitLabTestToken,
	})
	if err == nil {
		t.Fatal("NewGitLabSecureFilesStore() error = nil, want an https requirement")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("error = %q, want the https requirement", err)
	}
	if dials != 0 {
		t.Fatalf("server requests = %d, want none", dials)
	}
}

func TestGitLabSecureFilesRejectsHostWithCredentials(t *testing.T) {
	_, err := NewGitLabSecureFilesStore(GitLabSecureFilesOptions{
		Host:      "https://user:secret@gitlab.example.com",
		ProjectID: gitLabTestProject,
		Prefix:    gitLabTestPrefix,
		Token:     gitLabTestToken,
	})
	if err == nil {
		t.Fatal("NewGitLabSecureFilesStore() error = nil, want credentials to be rejected")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaks credentials: %q", err)
	}
}

func TestGitLabSecureFilesRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>sign in</body></html>"))
	}))
	t.Cleanup(server.Close)
	remote := newGitLabTestStore(t, server)

	err := remote.Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil, want an HTML response to fail closed")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("error = %q, want a non-JSON response failure", err)
	}
}

func TestGitLabSecureFilesRejectsCrossHostRedirect(t *testing.T) {
	elsewhere := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("redirect target must not be contacted: %s", r.URL)
	}))
	t.Cleanup(elsewhere.Close)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/api/v4/projects/42/secure_files", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	remote := newGitLabTestStore(t, server)

	err := remote.Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil, want a cross-host redirect to fail closed")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("error = %q, want a redirect failure", err)
	}
}

func TestGitLabSecureFilesRejectsOversizeDownload(t *testing.T) {
	oversize := bytes.Repeat([]byte("A"), MaxEncryptedArtifactBytes+1)
	name := gitLabTestPrefix + "/certs/distribution/serial.cer" + EncryptedArtifactSuffix
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(oversize)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{"id":7,"name":%q,"checksum":"","checksum_algorithm":"sha256"}]`, name)
	}))
	t.Cleanup(server.Close)
	remote := newGitLabTestStore(t, server)

	err := remote.Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil, want an oversize artifact to fail closed")
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("error = %q, want the size limit failure", err)
	}
}

func TestGitLabSecureFilesRejectsOversizeUploadBeforeRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			t.Fatal("oversize artifact must not be uploaded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	remote := newGitLabTestStore(t, server)

	source := newLocalArtifactStore(t)
	relPath := "certs/distribution/serial.cer"
	if err := source.WriteEncryptedFile(relPath, bytes.Repeat([]byte("B"), gitLabSecureFileSizeLimit+1), gitLabTestPassword); err != nil {
		t.Fatalf("WriteEncryptedFile() error: %v", err)
	}
	err := remote.Publish(context.Background(), source)
	if err == nil {
		t.Fatal("Publish() error = nil, want the GitLab size limit enforced")
	}
	if !strings.Contains(err.Error(), "5 MiB") {
		t.Fatalf("error = %q, want the documented GitLab secure file limit", err)
	}
}

func TestGitLabSecureFilesBoundsEachRequestWithTheSuppliedBudget(t *testing.T) {
	_, server := newFakeGitLabServer(t)
	budgets := 0
	store, err := NewGitLabSecureFilesStore(GitLabSecureFilesOptions{
		Host:       server.URL,
		ProjectID:  gitLabTestProject,
		Prefix:     gitLabTestPrefix,
		Token:      gitLabTestToken,
		HTTPClient: server.Client(),
		RequestContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
			budgets++
			bounded, cancel := context.WithCancel(ctx)
			cancel()
			return bounded, cancel
		},
	})
	if err != nil {
		t.Fatalf("NewGitLabSecureFilesStore() error: %v", err)
	}

	err = store.Fetch(context.Background(), newLocalArtifactStore(t))
	if err == nil {
		t.Fatal("Fetch() error = nil, want the exhausted request budget to fail")
	}
	if budgets == 0 {
		t.Fatal("request budget was not applied")
	}
	if strings.Contains(err.Error(), gitLabTestToken) {
		t.Fatalf("error leaks the token: %q", err)
	}
}

func TestGitLabSecureFilesLocatorOmitsToken(t *testing.T) {
	store, err := NewGitLabSecureFilesStore(GitLabSecureFilesOptions{
		Host:      "https://gitlab.example.com",
		ProjectID: "1234",
		Prefix:    "asc-signing",
		Token:     gitLabTestToken,
	})
	if err != nil {
		t.Fatalf("NewGitLabSecureFilesStore() error: %v", err)
	}
	locator := store.Locator()
	if strings.Contains(locator, gitLabTestToken) {
		t.Fatalf("locator leaks the token: %q", locator)
	}
	for _, want := range []string{"gitlab.example.com", "1234", "asc-signing"} {
		if !strings.Contains(locator, want) {
			t.Fatalf("locator = %q, want it to name %q", locator, want)
		}
	}
}

func TestNewGitLabSecureFilesStoreValidatesInputs(t *testing.T) {
	tests := []struct {
		name    string
		options GitLabSecureFilesOptions
		want    string
	}{
		{
			name:    "non numeric project",
			options: GitLabSecureFilesOptions{ProjectID: "group/project", Prefix: gitLabTestPrefix, Token: gitLabTestToken},
			want:    "numeric",
		},
		{
			name:    "traversal prefix",
			options: GitLabSecureFilesOptions{ProjectID: gitLabTestProject, Prefix: "../escape", Token: gitLabTestToken},
			want:    "prefix",
		},
		{
			name:    "empty token",
			options: GitLabSecureFilesOptions{ProjectID: gitLabTestProject, Prefix: gitLabTestPrefix, Token: "  "},
			want:    "token",
		},
		{
			name:    "token with newline",
			options: GitLabSecureFilesOptions{ProjectID: gitLabTestProject, Prefix: gitLabTestPrefix, Token: "abc\ndef"},
			want:    "token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitLabSecureFilesStore(tt.options)
			if err == nil {
				t.Fatal("NewGitLabSecureFilesStore() error = nil, want a validation failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestGitLabSecureFilesDefaultHostIsGitLabCom(t *testing.T) {
	store, err := NewGitLabSecureFilesStore(GitLabSecureFilesOptions{
		ProjectID: gitLabTestProject,
		Prefix:    gitLabTestPrefix,
		Token:     gitLabTestToken,
	})
	if err != nil {
		t.Fatalf("NewGitLabSecureFilesStore() error: %v", err)
	}
	if !strings.Contains(store.Locator(), "gitlab.com") {
		t.Fatalf("locator = %q, want the default host", store.Locator())
	}
}

func TestGitLabSecureFilesRejectsPathEscapingNames(t *testing.T) {
	fake, server := newFakeGitLabServer(t)
	remote := newGitLabTestStore(t, server)
	fake.put(gitLabTestPrefix+"/../escape/evil.cer"+EncryptedArtifactSuffix, []byte("evil"))

	destination := newLocalArtifactStore(t)
	err := remote.Fetch(context.Background(), destination)
	if err == nil {
		t.Fatal("Fetch() error = nil, want an escaping name rejected")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("error = %q, want a traversal failure", err)
	}
	files, err := destination.ListEncryptedFiles()
	if err != nil {
		t.Fatalf("ListEncryptedFiles() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("fetched files = %v, want nothing written", files)
	}
}
