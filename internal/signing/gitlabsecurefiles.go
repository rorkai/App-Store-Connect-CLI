package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	// gitLabDefaultHost is the public GitLab instance.
	gitLabDefaultHost = "https://gitlab.com"
	// gitLabSecureFileSizeLimit is GitLab's documented secure file limit.
	gitLabSecureFileSizeLimit = 5 << 20
	gitLabListPageSize        = 100
	gitLabMaxListPages        = 50
	gitLabMaxRemoteArtifacts  = 1024
	gitLabMaxRedirects        = 3
	gitLabErrorBodyLimit      = 8 << 10
	gitLabErrorTitleLimit     = 200
	gitLabRequestTimeout      = 2 * time.Minute
)

var gitLabProjectIDPattern = regexp.MustCompile(`^[0-9]{1,19}$`)

// GitLabSecureFilesOptions configures an experimental GitLab Secure Files
// store for already-encrypted signing artifacts.
type GitLabSecureFilesOptions struct {
	// Host is an https base URL. It defaults to https://gitlab.com.
	Host string
	// ProjectID is the numeric GitLab project ID.
	ProjectID string
	// Prefix scopes the artifacts this store owns inside the project.
	Prefix string
	// Token authenticates API requests. It is sent only as a request header.
	Token string
	// HTTPClient overrides the default client in tests.
	HTTPClient *http.Client
	// MaxArtifacts bounds how many prefixed artifacts may be transported.
	MaxArtifacts int
	// RequestContext bounds one list, download, or delete request.
	RequestContext RequestContextFunc
	// UploadContext bounds one upload request.
	UploadContext RequestContextFunc
}

// GitLabSecureFilesStore transports encrypted signing artifacts through the
// GitLab Secure Files API. It never receives the sync password and never
// decrypts an artifact.
type GitLabSecureFilesStore struct {
	baseURL        *url.URL
	projectID      string
	prefix         string
	token          string
	client         *http.Client
	maxArtifacts   int
	requestContext RequestContextFunc
	uploadContext  RequestContextFunc
}

type gitLabSecureFile struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Checksum          string `json:"checksum"`
	ChecksumAlgorithm string `json:"checksum_algorithm"`
}

// NewGitLabSecureFilesStore validates the locator and credentials of a GitLab
// Secure Files store.
func NewGitLabSecureFilesStore(options GitLabSecureFilesOptions) (*GitLabSecureFilesStore, error) {
	host := strings.TrimSpace(options.Host)
	if host == "" {
		host = gitLabDefaultHost
	}
	baseURL, err := parseGitLabHost(host)
	if err != nil {
		return nil, err
	}
	projectID := strings.TrimSpace(options.ProjectID)
	if !gitLabProjectIDPattern.MatchString(projectID) {
		return nil, errors.New("GitLab project must be a numeric project ID")
	}
	prefix := strings.TrimSpace(options.Prefix)
	if err := ValidateArtifactPrefix(prefix); err != nil {
		return nil, err
	}
	token := strings.TrimSpace(options.Token)
	if token == "" {
		return nil, errors.New("GitLab token must not be empty")
	}
	if err := validateGitLabToken(token); err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: gitLabRequestTimeout}
	if options.HTTPClient != nil {
		clone := *options.HTTPClient
		client = &clone
	}
	store := &GitLabSecureFilesStore{
		baseURL:        baseURL,
		projectID:      projectID,
		prefix:         prefix,
		token:          token,
		maxArtifacts:   options.MaxArtifacts,
		requestContext: options.RequestContext,
		uploadContext:  options.UploadContext,
	}
	if store.maxArtifacts <= 0 {
		store.maxArtifacts = gitLabMaxRemoteArtifacts
	}
	client.CheckRedirect = store.checkRedirect
	store.client = client
	return store, nil
}

func parseGitLabHost(host string) (*url.URL, error) {
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, errors.New("GitLab host is not a valid URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("GitLab host must use https")
	}
	if parsed.User != nil {
		return nil, errors.New("GitLab host must not embed credentials")
	}
	if parsed.Host == "" {
		return nil, errors.New("GitLab host must include a host name")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("GitLab host must not include a query or fragment")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed, nil
}

func validateGitLabToken(token string) error {
	for _, r := range token {
		if unicode.IsControl(r) || unicode.IsSpace(r) || r > unicode.MaxASCII {
			return errors.New("GitLab token must be printable ASCII without whitespace")
		}
	}
	return nil
}

// Locator returns a redacted description of the configured store.
func (s *GitLabSecureFilesStore) Locator() string {
	return "gitlab-secure-files://" + s.baseURL.Host + s.baseURL.Path + "/projects/" + s.projectID + "/" + s.prefix
}

// Fetch downloads every encrypted artifact under the configured prefix into
// the local ciphertext store.
func (s *GitLabSecureFilesStore) Fetch(ctx context.Context, store ArtifactStore) error {
	remote, err := s.listPrefixedFiles(ctx)
	if err != nil {
		return err
	}
	relPaths := make([]string, 0, len(remote))
	for relPath := range remote {
		relPaths = append(relPaths, relPath)
	}
	sort.Strings(relPaths)
	if err := ValidateEncryptedRepositoryPaths(relPaths); err != nil {
		return fmt.Errorf("gitlab secure files: %w", err)
	}
	for _, relPath := range relPaths {
		file := remote[relPath]
		ciphertext, err := s.download(ctx, file)
		if err != nil {
			return err
		}
		if err := verifyGitLabChecksum(file, ciphertext); err != nil {
			return err
		}
		if err := store.WriteEncryptedArtifact(relPath, ciphertext); err != nil {
			return fmt.Errorf("gitlab secure files: store %s: %w", relPath, err)
		}
	}
	return nil
}

// Publish uploads every local encrypted artifact that the remote store does
// not already hold byte-for-byte. Secure files outside the configured prefix
// are never inspected, replaced, or deleted.
func (s *GitLabSecureFilesStore) Publish(ctx context.Context, store ArtifactStore) error {
	local, err := store.ListEncryptedFiles()
	if err != nil {
		return err
	}
	sort.Strings(local)
	if len(local) > s.maxArtifacts {
		return fmt.Errorf("gitlab secure files: %d encrypted artifacts exceed the %d-artifact limit", len(local), s.maxArtifacts)
	}
	remote, err := s.listPrefixedFiles(ctx)
	if err != nil {
		return err
	}

	for _, relPath := range local {
		name, err := remoteArtifactName(s.prefix, relPath)
		if err != nil {
			return fmt.Errorf("gitlab secure files: %w", err)
		}
		ciphertext, err := store.ReadEncryptedArtifact(relPath)
		if err != nil {
			return fmt.Errorf("gitlab secure files: read %s: %w", relPath, err)
		}
		if err := checkGitLabArtifactSize(relPath, len(ciphertext)); err != nil {
			return err
		}
		existing, exists := remote[relPath]
		if exists && sameGitLabChecksum(existing, ciphertext) {
			continue
		}
		if !exists {
			if err := s.upload(ctx, name, ciphertext); err != nil {
				return err
			}
			continue
		}
		if err := s.replace(ctx, existing, name, ciphertext); err != nil {
			return err
		}
	}
	return nil
}

// replace swaps the content of an existing secure file. GitLab names are
// unique per project and the API has no update operation, so the previous
// ciphertext is kept in memory and restored if the replacement upload fails.
func (s *GitLabSecureFilesStore) replace(ctx context.Context, existing gitLabSecureFile, name string, ciphertext []byte) error {
	previous, err := s.download(ctx, existing)
	if err != nil {
		return fmt.Errorf("%w; the previous secure file %s was left unchanged", err, name)
	}
	if err := verifyGitLabChecksum(existing, previous); err != nil {
		return err
	}
	if err := s.deleteFile(ctx, existing); err != nil {
		return err
	}
	if uploadErr := s.upload(ctx, name, ciphertext); uploadErr != nil {
		if restoreErr := s.upload(ctx, name, previous); restoreErr != nil {
			return fmt.Errorf("%w; restoring the previous secure file %s also failed: %w", uploadErr, name, restoreErr)
		}
		return fmt.Errorf("%w; the previous secure file %s was restored", uploadErr, name)
	}
	return nil
}

func checkGitLabArtifactSize(relPath string, size int) error {
	if size == 0 {
		return fmt.Errorf("gitlab secure files: encrypted artifact %s is empty", relPath)
	}
	if size > MaxEncryptedArtifactBytes {
		return fmt.Errorf("gitlab secure files: encrypted artifact %s exceeds the %d-byte size limit", relPath, MaxEncryptedArtifactBytes)
	}
	// GitLab accepts an upload only when its size is strictly below the limit.
	if size >= gitLabSecureFileSizeLimit {
		return fmt.Errorf("gitlab secure files: encrypted artifact %s exceeds the GitLab secure file limit of 5 MiB", relPath)
	}
	return nil
}

func sameGitLabChecksum(file gitLabSecureFile, ciphertext []byte) bool {
	if !strings.EqualFold(strings.TrimSpace(file.ChecksumAlgorithm), "sha256") {
		return false
	}
	digest := sha256.Sum256(ciphertext)
	return strings.EqualFold(strings.TrimSpace(file.Checksum), hex.EncodeToString(digest[:]))
}

func verifyGitLabChecksum(file gitLabSecureFile, ciphertext []byte) error {
	if strings.TrimSpace(file.Checksum) == "" {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(file.ChecksumAlgorithm), "sha256") {
		return nil
	}
	if !sameGitLabChecksum(file, ciphertext) {
		return fmt.Errorf("gitlab secure files: downloaded artifact %s does not match its published checksum", file.Name)
	}
	return nil
}

func (s *GitLabSecureFilesStore) listPrefixedFiles(ctx context.Context) (map[string]gitLabSecureFile, error) {
	files, err := s.listFiles(ctx)
	if err != nil {
		return nil, err
	}
	prefixed := make(map[string]gitLabSecureFile)
	for _, file := range files {
		relPath, scoped := remoteArtifactRelativePath(s.prefix, file.Name)
		if !scoped {
			continue
		}
		if err := validateRemoteArtifactRelativePath(relPath); err != nil {
			return nil, fmt.Errorf("gitlab secure files: %w", err)
		}
		if existing, duplicate := prefixed[relPath]; duplicate {
			return nil, fmt.Errorf("gitlab secure files: secure files %d and %d map to the same artifact path", existing.ID, file.ID)
		}
		prefixed[relPath] = file
		if len(prefixed) > s.maxArtifacts {
			return nil, fmt.Errorf("gitlab secure files: prefix holds more than %d encrypted artifacts", s.maxArtifacts)
		}
	}
	return prefixed, nil
}

func (s *GitLabSecureFilesStore) listFiles(ctx context.Context) ([]gitLabSecureFile, error) {
	var files []gitLabSecureFile
	page := "1"
	for visited := 0; visited < gitLabMaxListPages; visited++ {
		endpoint := s.endpoint("")
		query := url.Values{}
		query.Set("per_page", strconv.Itoa(gitLabListPageSize))
		query.Set("page", page)
		endpoint.RawQuery = query.Encode()

		response, cancel, err := s.do(ctx, http.MethodGet, endpoint, "", nil, "list secure files")
		if err != nil {
			return nil, err
		}
		body, err := s.readJSON(response, "list secure files")
		nextPage := strings.TrimSpace(response.Header.Get("X-Next-Page"))
		response.Body.Close()
		cancel()
		if err != nil {
			return nil, err
		}
		var pageFiles []gitLabSecureFile
		if err := json.Unmarshal(body, &pageFiles); err != nil {
			return nil, errors.New("gitlab secure files: list secure files returned an unexpected JSON payload")
		}
		// Unrelated project files are counted only against the page budget.
		// The artifact limit applies to the configured prefix so a shared
		// project cannot block a small, correctly scoped sync.
		files = append(files, pageFiles...)
		if nextPage == "" || nextPage == page {
			return files, nil
		}
		page = nextPage
	}
	return nil, fmt.Errorf("gitlab secure files: list secure files exceeded %d pages", gitLabMaxListPages)
}

func (s *GitLabSecureFilesStore) download(ctx context.Context, file gitLabSecureFile) ([]byte, error) {
	endpoint := s.endpoint(strconv.FormatInt(file.ID, 10) + "/download")
	response, cancel, err := s.do(ctx, http.MethodGet, endpoint, "", nil, "download secure file")
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, s.responseError("download secure file", response)
	}
	if isGitLabHTML(response) {
		return nil, errors.New("gitlab secure files: download secure file returned HTML instead of file content")
	}
	ciphertext, err := io.ReadAll(io.LimitReader(response.Body, MaxEncryptedArtifactBytes+1))
	if err != nil {
		return nil, fmt.Errorf("gitlab secure files: download secure file: %w", sanitizeGitLabError(err, s.token))
	}
	if len(ciphertext) > MaxEncryptedArtifactBytes {
		return nil, fmt.Errorf("gitlab secure files: secure file %s exceeds the %d-byte size limit", file.Name, MaxEncryptedArtifactBytes)
	}
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("gitlab secure files: secure file %s is empty", file.Name)
	}
	return ciphertext, nil
}

func (s *GitLabSecureFilesStore) upload(ctx context.Context, name string, ciphertext []byte) error {
	// POST /projects/:id/secure_files requires both "name" and "file", and the
	// record is stored under "name", so the prefixed path survives. The
	// multipart part filename is not the stored name.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("name", name); err != nil {
		return fmt.Errorf("gitlab secure files: build upload: %w", err)
	}
	part, err := writer.CreateFormFile("file", path.Base(name))
	if err != nil {
		return fmt.Errorf("gitlab secure files: build upload: %w", err)
	}
	if _, err := part.Write(ciphertext); err != nil {
		return fmt.Errorf("gitlab secure files: build upload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("gitlab secure files: build upload: %w", err)
	}

	response, cancel, err := s.do(ctx, http.MethodPost, s.endpoint(""), writer.FormDataContentType(), body.Bytes(), "upload secure file")
	if err != nil {
		return err
	}
	defer cancel()
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return s.responseError("upload secure file", response)
	}
	created, err := s.readJSON(response, "upload secure file")
	if err != nil {
		return err
	}
	var stored gitLabSecureFile
	if err := json.Unmarshal(created, &stored); err != nil {
		return errors.New("gitlab secure files: upload secure file returned an unexpected JSON payload")
	}
	// Fail closed if the instance stored the artifact under a different name,
	// because a later fetch would not find it under the configured prefix.
	if strings.TrimSpace(stored.Name) != name {
		return fmt.Errorf("gitlab secure files: GitLab stored the artifact under an unexpected name instead of %s", name)
	}
	return nil
}

func (s *GitLabSecureFilesStore) deleteFile(ctx context.Context, file gitLabSecureFile) error {
	endpoint := s.endpoint(strconv.FormatInt(file.ID, 10))
	response, cancel, err := s.do(ctx, http.MethodDelete, endpoint, "", nil, "replace secure file")
	if err != nil {
		return err
	}
	defer cancel()
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusOK {
		return s.responseError("replace secure file", response)
	}
	return nil
}

func (s *GitLabSecureFilesStore) endpoint(suffix string) *url.URL {
	endpoint := *s.baseURL
	endpoint.Path = path.Join(s.baseURL.Path, "api", "v4", "projects", s.projectID, "secure_files", suffix)
	return &endpoint
}

// do issues one bounded request. The returned cancel function releases the
// per-request budget and must be called after the response body is consumed.
func (s *GitLabSecureFilesStore) do(
	ctx context.Context,
	method string,
	endpoint *url.URL,
	contentType string,
	body []byte,
	operation string,
) (*http.Response, context.CancelFunc, error) {
	budget := s.requestContext
	if body != nil {
		budget = s.uploadContext
	}
	requestCtx, cancel := boundedRequestContext(budget, ctx)

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(requestCtx, method, endpoint.String(), reader)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("gitlab secure files: %s: %w", operation, sanitizeGitLabError(err, s.token))
	}
	request.Header.Set("PRIVATE-TOKEN", s.token)
	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := s.client.Do(request)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("gitlab secure files: %s: %w", operation, sanitizeGitLabError(err, s.token))
	}
	return response, cancel, nil
}

func (s *GitLabSecureFilesStore) checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= gitLabMaxRedirects {
		return errors.New("gitlab secure files: too many redirects")
	}
	if !strings.EqualFold(request.URL.Scheme, "https") {
		return errors.New("gitlab secure files: refusing a non-https redirect")
	}
	if !strings.EqualFold(request.URL.Host, s.baseURL.Host) {
		return fmt.Errorf("gitlab secure files: refusing a redirect to another host %q", request.URL.Host)
	}
	return nil
}

func (s *GitLabSecureFilesStore) readJSON(response *http.Response, operation string) ([]byte, error) {
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, s.responseError(operation, response)
	}
	if !isGitLabJSON(response) {
		return nil, fmt.Errorf("gitlab secure files: %s returned a non-JSON response", operation)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxEncryptedArtifactBytes+1))
	if err != nil {
		return nil, fmt.Errorf("gitlab secure files: %s: read response", operation)
	}
	if len(body) > MaxEncryptedArtifactBytes {
		return nil, fmt.Errorf("gitlab secure files: %s returned a response above the %d-byte size limit", operation, MaxEncryptedArtifactBytes)
	}
	return body, nil
}

func (s *GitLabSecureFilesStore) responseError(operation string, response *http.Response) error {
	return gitLabResponseError(operation, response, s.token)
}

func gitLabResponseError(operation string, response *http.Response, token string) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, gitLabErrorBodyLimit))
	title := gitLabErrorTitle(response, body)
	title = redactGitLabSecrets(title, token)
	if title == "" {
		return fmt.Errorf("gitlab secure files: %s failed with HTTP %d", operation, response.StatusCode)
	}
	return fmt.Errorf("gitlab secure files: %s failed with HTTP %d: %s", operation, response.StatusCode, title)
}

func gitLabErrorTitle(response *http.Response, body []byte) string {
	if !isGitLabJSON(response) {
		return "non-JSON response"
	}
	var payload struct {
		Message any `json:"message"`
		Error   any `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "unreadable JSON response"
	}
	title := gitLabErrorValue(payload.Message)
	if title == "" {
		title = gitLabErrorValue(payload.Error)
	}
	return title
}

func gitLabErrorValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if part := gitLabErrorValue(item); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if part := gitLabErrorValue(typed[key]); part != "" {
				parts = append(parts, key+" "+part)
			}
		}
		return strings.Join(parts, "; ")
	default:
		return ""
	}
}

func redactGitLabSecrets(text, token string) string {
	if token != "" {
		text = strings.ReplaceAll(text, token, "[REDACTED]")
	}
	cleaned := make([]rune, 0, len(text))
	for _, r := range text {
		if unicode.IsControl(r) {
			r = ' '
		}
		cleaned = append(cleaned, r)
		if len(cleaned) >= gitLabErrorTitleLimit {
			break
		}
	}
	return strings.TrimSpace(string(cleaned))
}

func sanitizeGitLabError(err error, token string) error {
	if err == nil {
		return nil
	}
	redacted := redactGitLabSecrets(err.Error(), token)
	if redacted == "" {
		return errors.New("request failed")
	}
	return errors.New(redacted)
}

func isGitLabJSON(response *http.Response) bool {
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

func isGitLabHTML(response *http.Response) bool {
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}
