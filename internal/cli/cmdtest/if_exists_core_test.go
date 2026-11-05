package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's real 409 body for a duplicate versionString on POST /v1/appStoreVersions,
// captured live against app 6759231657 on 2026-09-15. Apple reports two errors
// and the duplicate is the second one, so the conflict matcher has to inspect
// every entry in errors[] rather than only the first.
const versionsDuplicate409 = `{"errors":[{"id":"b068c5c0-b3fa-4d12-aa89-f1b9aa061b28","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}},{"id":"eb1884a7-e427-42db-ac95-26c49c84a5c2","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"The version number has been previously used.","source":{"pointer":"/data/attributes/versionString"}}]}`

// Apple's 409 body when an appStoreReviewDetail already exists for the version,
// captured live against app 6759231657 on 2026-09-15. Apple answers this
// existence conflict with STATE_ERROR.ALREADY_EXISTS.
const reviewDetailExists409 = `{"errors":[{"id":"a48854d3-ef2e-4eea-9451-228c907bf1ad","status":"409","code":"STATE_ERROR.ALREADY_EXISTS","title":"Resource already exists.","detail":"The given app version already has an existing review."}]}`

// Apple's 409 on POST /v1/appStoreVersions when the app cannot take a new
// version yet; it is not an existence conflict and must keep failing.
const versionsRelationship409 = `{"errors":[{"id":"7c2d9e1f-4b3a-4c6d-8e5f-1a2b3c4d5e6f","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}}]}`

// A 409 that is not an existence conflict and must keep failing.
const versionsState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"You cannot create a new version while another version is in review."}]}`

const existingVersionsList = `{"data":[{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION","copyright":"old"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions"}}`

const existingReviewDetail = `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"contactFirstName":"Old","notes":"old notes","demoAccountRequired":false}}}`

const sourceVersionsList = `{"data":[{"type":"appStoreVersions","id":"version-source","attributes":{"versionString":"1.9.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}]}`

const sourceLocalizationsList = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source-en","attributes":{"locale":"en-US","description":"Source description","keywords":"one,two"}}]}`

const destinationLocalizationsList = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-dest-en","attributes":{"locale":"en-US"}}]}`

type ifExistsRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func runIfExistsCommand(t *testing.T, args []string, handler func(req ifExistsRequest) (*http.Response, error)) (string, string, []ifExistsRequest, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	log := newRequestLog(4)
	var seenMu sync.Mutex
	var seen []ifExistsRequest
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := ""
		if req.Body != nil {
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			body = string(payload)
		}
		entry := ifExistsRequest{Method: req.Method, Path: req.URL.Path, Query: req.URL.RawQuery, Body: body}
		log.Add(req.Method + " " + req.URL.Path)
		seenMu.Lock()
		seen = append(seen, entry)
		seenMu.Unlock()
		return handler(entry)
	}))

	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, seen, runErr
}

type ifExistsReply struct {
	status int
	body   string
}

// TestIfExistsConflictsWithoutAMatchKeepFailing covers the create commands
// that take --if-exists: the default fail mode, a 409 that is not an existence
// conflict, and an existence 409 whose read-back proves nothing all surface the
// original conflict, print nothing, and send no request beyond the listed ones.
func TestIfExistsConflictsWithoutAMatchKeepFailing(t *testing.T) {
	const (
		capabilitiesPost = "POST /v1/bundleIdCapabilities"
		capabilitiesGet  = "GET /v1/bundleIds/bundle-1/bundleIdCapabilities"
		reviewItemsPost  = "POST /v1/reviewSubmissionItems"
		reviewItemsGet   = "GET /v1/reviewSubmissions/submission-1/items"
		reviewDetailPost = "POST /v1/appStoreReviewDetails"
		reviewDetailGet  = "GET /v1/appStoreVersions/version-1/appStoreReviewDetail"
		localizationPost = "POST /v1/appStoreVersionLocalizations"
		localizationGet  = "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations"
		territoriesGet   = "GET /v1/territories"
		availabilityPost = "POST /v2/appAvailabilities"
		availabilityGet  = "GET /v1/apps/app-1/appAvailabilityV2"
	)
	capabilityArgs := []string{"bundle-ids", "capabilities", "add", "--bundle", "bundle-1", "--capability", "ICLOUD", "--output", "json"}
	reviewItemArgs := []string{"review", "items-add", "--submission", "submission-1", "--item-type", "appStoreVersions", "--item-id", "version-1", "--output", "json"}
	reviewDetailArgs := []string{"review", "details-create", "--version-id", "version-1", "--notes", "new notes", "--output", "json"}
	localizationArgs := []string{"localizations", "create", "--version", "version-1", "--locale", "ja", "--description", "new description", "--whats-new", "new notes", "--output", "json"}
	with := func(base []string, extra ...string) []string {
		return append(append([]string{}, base...), extra...)
	}
	// A matching item next to an errors member is not trustworthy evidence.
	malformedReviewItems := `{"errors":[],"data":[{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items"}}`
	notFound := `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`

	tests := []struct {
		name         string
		args         []string
		replies      map[string]ifExistsReply
		wantErr      string
		wantRequests []string
	}{
		{
			name:         "capabilities add default fail keeps Apple's detail",
			args:         capabilityArgs,
			replies:      map[string]ifExistsReply{capabilitiesPost: {http.StatusConflict, capabilityExists409}},
			wantErr:      "The capability is already enabled for this bundle ID.",
			wantRequests: []string{capabilitiesPost},
		},
		{
			name:         "capabilities add skip does not read back a rejected type",
			args:         with(capabilityArgs, "--if-exists", "skip"),
			replies:      map[string]ifExistsReply{capabilitiesPost: {http.StatusConflict, capabilityTypeRejected409}},
			wantRequests: []string{capabilitiesPost},
		},
		{
			name: "capabilities add skip read-back finds nothing",
			args: with(capabilityArgs, "--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				capabilitiesPost: {http.StatusConflict, capabilityExists409},
				capabilitiesGet:  {http.StatusOK, noCapabilities},
			},
			wantRequests: []string{capabilitiesPost, capabilitiesGet},
		},
		{
			name:         "review items-add default fail",
			args:         reviewItemArgs,
			replies:      map[string]ifExistsReply{reviewItemsPost: {http.StatusConflict, reviewItemExists409}},
			wantRequests: []string{reviewItemsPost},
		},
		{
			name:         "review items-add skip does not read back a state conflict",
			args:         with(reviewItemArgs, "--if-exists", "skip"),
			replies:      map[string]ifExistsReply{reviewItemsPost: {http.StatusConflict, reviewItemState409}},
			wantRequests: []string{reviewItemsPost},
		},
		{
			name: "review items-add skip ignores a REMOVED item",
			args: with(reviewItemArgs, "--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				reviewItemsPost: {http.StatusConflict, reviewItemExists409},
				reviewItemsGet:  {http.StatusOK, reviewItemsRemovedVersion},
			},
			wantRequests: []string{reviewItemsPost, reviewItemsGet},
		},
		{
			name: "review items-add skip read-back finds another item",
			args: with(reviewItemArgs, "--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				reviewItemsPost: {http.StatusConflict, reviewItemExists409},
				reviewItemsGet:  {http.StatusOK, reviewItemsOtherVersion},
			},
			wantRequests: []string{reviewItemsPost, reviewItemsGet},
		},
		{
			name: "review items-add skip rejects a malformed read-back",
			args: with(reviewItemArgs, "--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				reviewItemsPost: {http.StatusConflict, reviewItemExists409},
				reviewItemsGet:  {http.StatusOK, malformedReviewItems},
			},
			wantRequests: []string{reviewItemsPost, reviewItemsGet},
		},
		{
			name: "review details-create default fail",
			args: []string{
				"review", "details-create", "--version-id", "version-1",
				"--contact-first-name", "Dev", "--contact-last-name", "Support",
				"--contact-email", "dev@example.com", "--contact-phone", "+1 408 555 0100",
				"--notes", "new notes", "--output", "json",
			},
			replies:      map[string]ifExistsReply{reviewDetailPost: {http.StatusConflict, reviewDetailExists409}},
			wantRequests: []string{reviewDetailPost},
		},
		{
			name:         "review details-create skip does not read back a state conflict",
			args:         with(reviewDetailArgs, "--if-exists", "skip"),
			replies:      map[string]ifExistsReply{reviewDetailPost: {http.StatusConflict, versionsState409}},
			wantRequests: []string{reviewDetailPost},
		},
		{
			name: "review details-create skip read-back finds nothing",
			args: with(reviewDetailArgs, "--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				reviewDetailPost: {http.StatusConflict, reviewDetailExists409},
				reviewDetailGet:  {http.StatusNotFound, notFound},
			},
			wantRequests: []string{reviewDetailPost, reviewDetailGet},
		},
		{
			name:         "localizations create default fail keeps Apple's detail",
			args:         localizationArgs,
			replies:      map[string]ifExistsReply{localizationPost: {http.StatusConflict, localizationDuplicate409}},
			wantErr:      "already exists. Try updating.",
			wantRequests: []string{localizationPost},
		},
		{
			name:         "localizations create update does not read back a state conflict",
			args:         with(localizationArgs, "--if-exists", "update"),
			replies:      map[string]ifExistsReply{localizationPost: {http.StatusConflict, localizationState409}},
			wantRequests: []string{localizationPost},
		},
		{
			name: "localizations create skip read-back finds nothing",
			args: with(localizationArgs, "--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				localizationPost: {http.StatusConflict, localizationDuplicate409},
				localizationGet:  {http.StatusOK, noLocalizationsList},
			},
			wantRequests: []string{localizationPost, localizationGet},
		},
		{
			name: "pricing availability create default fail keeps Apple's detail",
			args: availabilityCreateArgs(),
			replies: map[string]ifExistsReply{
				territoriesGet:   {http.StatusOK, territoryCatalog},
				availabilityPost: {http.StatusConflict, availabilityExists409},
			},
			wantErr:      "An 'appAvailabilities' with a relationship to 'apps' with id 'app-1' already exists.",
			wantRequests: []string{territoriesGet, availabilityPost},
		},
		{
			// The bootstrap rejection shares the existence conflict's 409 code.
			name: "pricing availability create skip keeps the bootstrap remediation",
			args: availabilityCreateArgs("--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				territoriesGet:   {http.StatusOK, territoryCatalog},
				availabilityPost: {http.StatusConflict, availabilityBootstrap409},
			},
			wantErr:      "asc web apps availability create",
			wantRequests: []string{territoriesGet, availabilityPost},
		},
		{
			name: "pricing availability create update does not read back a state conflict",
			args: availabilityCreateArgs("--if-exists", "update"),
			replies: map[string]ifExistsReply{
				territoriesGet:   {http.StatusOK, territoryCatalog},
				availabilityPost: {http.StatusConflict, availabilityState409},
			},
			wantRequests: []string{territoriesGet, availabilityPost},
		},
		{
			name: "pricing availability create skip read-back finds nothing",
			args: availabilityCreateArgs("--if-exists", "skip"),
			replies: map[string]ifExistsReply{
				territoriesGet:   {http.StatusOK, territoryCatalog},
				availabilityPost: {http.StatusConflict, availabilityExists409},
				availabilityGet:  {http.StatusNotFound, availabilityNotFound404},
			},
			wantRequests: []string{territoriesGet, availabilityPost, availabilityGet},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, _, seen, runErr := runIfExistsCommand(t, test.args, func(req ifExistsRequest) (*http.Response, error) {
				reply, ok := test.replies[req.Method+" "+req.Path]
				if !ok {
					t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				}
				return jsonResponse(reply.status, reply.body)
			})
			if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
				t.Fatalf("run error = %v, want the original 409 conflict", runErr)
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("run error = %v, want %q", runErr, test.wantErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			got := make([]string, 0, len(seen))
			for _, req := range seen {
				got = append(got, req.Method+" "+req.Path)
			}
			if strings.Join(got, "\n") != strings.Join(test.wantRequests, "\n") {
				t.Fatalf("requests = %q, want %q", got, test.wantRequests)
			}
		})
	}
}

func TestIfExistsRejectsUnsupportedModeBeforeHTTP(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "versions create",
			args: []string{"versions", "create", "--app", "app-1", "--version", "2.0.0", "--if-exists", "merge"},
			want: "Error: --if-exists must be one of fail, skip, update",
		},
		{
			name: "localizations create",
			args: []string{"localizations", "create", "--version", "version-1", "--locale", "ja", "--description", "new description", "--if-exists", "replace", "--output", "json"},
			want: "Error: --if-exists must be one of fail, skip, update",
		},
		{
			name: "pricing availability create",
			args: availabilityCreateArgs("--if-exists", "replace"),
			want: "Error: --if-exists must be one of fail, skip, update",
		},
		{
			// update has no meaning for a submission item.
			name: "review items-add lists only fail and skip",
			args: []string{"review", "items-add", "--submission", "submission-1", "--item-type", "appStoreVersions", "--item-id", "version-1", "--if-exists", "update", "--output", "json"},
			want: `Error: --if-exists must be one of fail, skip (got "update")`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, seen, runErr := runIfExistsCommand(t, test.args, func(req ifExistsRequest) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s; the usage error must precede any HTTP request", req.Method, req.Path)
				return nil, nil
			})
			if !isUsageClassError(runErr) {
				t.Fatalf("run error = %v, want usage-class error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if len(seen) != 0 {
				t.Fatalf("requests = %+v, want none", seen)
			}
		})
	}
}

func TestVersionsCreateIfExistsSkipReturnsExistingVersion(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			if !strings.Contains(req.Query, "filter%5BversionString%5D=2.0.0") || !strings.Contains(req.Query, "filter%5Bplatform%5D=IOS") {
				t.Fatalf("read-back query = %q, want versionString and platform filters", req.Query)
			}
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %d, want POST then read-back GET: %+v", len(seen), seen)
	}
	var result struct {
		ID            string `json:"id"`
		VersionString string `json:"versionString"`
		AlreadyExists bool   `json:"alreadyExists"`
		Action        string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.ID != "version-existing" || result.VersionString != "2.0.0" {
		t.Fatalf("receipt = %+v, want existing version", result)
	}
	if !result.AlreadyExists || result.Action != "skipped" {
		t.Fatalf("receipt = %+v, want alreadyExists=true action=skipped", result)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "version-existing") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want already-exists diagnostic", stderr)
	}
}

func TestVersionsCreateIfExistsUpdatePatchesExistingVersion(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0",
		"--copyright", "2026 Example", "--release-type", "MANUAL",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing":
			if !strings.Contains(req.Body, `"copyright":"2026 Example"`) || !strings.Contains(req.Body, `"releaseType":"MANUAL"`) {
				t.Fatalf("PATCH body = %s, want copyright and releaseType", req.Body)
			}
			if strings.Contains(req.Body, "versionString") || strings.Contains(req.Body, "platform") {
				t.Fatalf("PATCH body = %s, must not resend create-only attributes", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION","copyright":"2026 Example","releaseType":"MANUAL"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	var result struct {
		ID            string `json:"id"`
		AlreadyExists bool   `json:"alreadyExists"`
		Action        string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.ID != "version-existing" || !result.AlreadyExists || result.Action != "updated" {
		t.Fatalf("receipt = %+v, want existing id, alreadyExists=true, action=updated", result)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "--if-exists update") {
		t.Fatalf("stderr = %q, want already-exists diagnostic", stderr)
	}
}

// When the date is the only updatable flag, dropping it would report a skip for
// a version whose scheduled release never changed.
func TestVersionsCreateIfExistsUpdateAppliesEarliestReleaseDate(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0",
		"--earliest-release-date", "2026-02-01T08:00:00+00:00",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing":
			if !strings.Contains(req.Body, `"earliestReleaseDate":"2026-02-01T08:00:00+00:00"`) {
				t.Fatalf("PATCH body = %s, want earliestReleaseDate", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	if !strings.Contains(stdout, `"action":"updated"`) {
		t.Fatalf("stdout = %q, want action=updated", stdout)
	}
}

func TestVersionsCreateIfExistsUpdateWithoutUpdatableFlagsSkips(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want no PATCH when nothing is updatable", seen)
	}
	if !strings.Contains(stdout, `"action":"skipped"`) {
		t.Fatalf("stdout = %q, want action=skipped", stdout)
	}
}

// --if-exists update carries --copy-metadata-from onto the existing version even
// when there is nothing to PATCH on the version itself, matching the documented
// behavior: skip leaves the version untouched, update still copies metadata.
func TestVersionsCreateIfExistsUpdateCopiesMetadataWithoutUpdatableFlags(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--copy-metadata-from", "1.9.0", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			if strings.Contains(req.Query, "filter%5BversionString%5D=1.9.0") {
				return jsonResponse(http.StatusOK, sourceVersionsList)
			}
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-source/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, sourceLocalizationsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-existing/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, destinationLocalizationsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-dest-en":
			if !strings.Contains(req.Body, "Source description") {
				t.Fatalf("copy PATCH body = %s, want the source description", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-dest-en","attributes":{"locale":"en-US"}}}`)
		default:
			t.Fatalf("unexpected request %s %s?%s", req.Method, req.Path, req.Query)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	var result struct {
		Action       string `json:"action"`
		MetadataCopy *struct {
			SourceVersion      string `json:"sourceVersion"`
			CopiedLocales      int    `json:"copiedLocales"`
			CopiedFieldUpdates int    `json:"copiedFieldUpdates"`
		} `json:"metadataCopy"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.MetadataCopy == nil {
		t.Fatalf("receipt = %q, want metadataCopy on --if-exists update", stdout)
	}
	if result.MetadataCopy.SourceVersion != "1.9.0" || result.MetadataCopy.CopiedLocales != 1 {
		t.Fatalf("metadataCopy = %+v, want the 1.9.0 copy to have run", *result.MetadataCopy)
	}
	// The metadata copy PATCHes the existing version's localizations, so the
	// receipt must report a mutation even though the version resource itself
	// had nothing to PATCH.
	if result.Action != "updated" {
		t.Fatalf("action = %q, want updated when the metadata copy ran against the existing version", result.Action)
	}
	if !strings.Contains(stderr, "updated it in place") {
		t.Fatalf("stderr = %q, want the update diagnostic rather than left unchanged", stderr)
	}
	sawCopyPatch := false
	for _, req := range seen {
		if req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-dest-en" {
			sawCopyPatch = true
		}
	}
	if !sawCopyPatch {
		t.Fatalf("requests = %+v, want the metadata copy PATCH", seen)
	}
}

// --if-exists skip must leave the existing version completely untouched, so the
// metadata copy does not run.
func TestVersionsCreateIfExistsSkipDoesNotCopyMetadata(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--copy-metadata-from", "1.9.0", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s?%s", req.Method, req.Path, req.Query)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want only the POST and the read-back", seen)
	}
	if strings.Contains(stdout, "metadataCopy") {
		t.Fatalf("stdout = %q, want no metadataCopy on skip", stdout)
	}
}

func TestVersionsCreateDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case isVersionsCreateDiagnosticListing(req):
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	// Apple's own errors[0] detail is what the CLI has always surfaced for this
	// body; --if-exists fail must keep printing it verbatim.
	if !strings.Contains(runErr.Error(), "You cannot create a new version of the App in the current state.") {
		t.Fatalf("run error = %v, want Apple detail preserved", runErr)
	}
	// The failure path adds one read of the platform's versions so the error
	// can name the version that already uses this string.
	if !strings.Contains(runErr.Error(), `Version "2.0.0" already exists on IOS as version-existing (PREPARE_FOR_SUBMISSION). Reuse it with --if-exists skip or --if-exists update`) {
		t.Fatalf("run error = %v, want the existing version named", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want the POST then one diagnostic listing", seen)
	}
}

// isVersionsCreateDiagnosticListing reports the read that versions create
// makes after a 409: every version on the platform, not a version-string
// read-back.
func isVersionsCreateDiagnosticListing(req ifExistsRequest) bool {
	return req.Method == http.MethodGet &&
		req.Path == "/v1/apps/app-1/appStoreVersions" &&
		!strings.Contains(req.Query, "filter%5BversionString%5D") &&
		strings.Contains(req.Query, "filter%5Bplatform%5D=IOS")
}

func TestVersionsCreateDefaultIfExistsFailPreservesSuccessfulOutput(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "default", args: nil},
		{name: "explicit fail", args: []string{"--if-exists", "fail"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS", "--output", "json"}
			args = append(args, tt.args...)
			stdout, stderr, seen, runErr := runIfExistsCommand(t, args, func(req ifExistsRequest) (*http.Response, error) {
				if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions" {
					if strings.Contains(req.Body, `"build"`) {
						t.Fatalf("POST body = %s, want no build relationship without --build-id", req.Body)
					}
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"version-created","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}}`)
				}
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			const want = `{"id":"version-created","versionString":"2.0.0","platform":"IOS","state":"PREPARE_FOR_SUBMISSION"}` + "\n"
			if stdout != want {
				t.Fatalf("stdout = %q, want legacy output %q", stdout, want)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if len(seen) != 1 {
				t.Fatalf("requests = %+v, want only the POST", seen)
			}
		})
	}
}

func TestVersionsCreateIfExistsSkipStillFailsWhenReadBackFindsNothing(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original 409", runErr)
	}
	if !strings.Contains(runErr.Error(), "You cannot create a new version of the App in the current state.") {
		t.Fatalf("run error = %v, want the original conflict", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 3 || isVersionsCreateDiagnosticListing(seen[1]) || !isVersionsCreateDiagnosticListing(seen[2]) {
		t.Fatalf("requests = %+v, want POST, read-back, then the diagnostic listing", seen)
	}
}

func TestVersionsCreateIfExistsOnlyHandlesDuplicateVersionCode(t *testing.T) {
	for name, body := range map[string]string{
		"state error":           versionsState409,
		"relationship rejected": versionsRelationship409,
	} {
		t.Run(name, func(t *testing.T) {
			stdout, _, seen, runErr := runIfExistsCommand(t, []string{
				"versions", "create", "--app", "app-1", "--version", "2.0.0", "--if-exists", "skip", "--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
					return jsonResponse(http.StatusConflict, body)
				case isVersionsCreateDiagnosticListing(req):
					return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-draft","attributes":{"versionString":"1.9.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.Path)
					return nil, nil
				}
			})
			if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
				t.Fatalf("run error = %v, want the original 409", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			// No version-string read-back: the 409 is not an existence
			// conflict, so skip cannot apply. The only extra read is the
			// diagnostic listing that names the unreleased version.
			if len(seen) != 2 || !isVersionsCreateDiagnosticListing(seen[1]) {
				t.Fatalf("requests = %+v, want the POST then one diagnostic listing", seen)
			}
			if !strings.Contains(runErr.Error(), `To ship "2.0.0" from editable version 1.9.0, rename it: asc versions update --version-id "version-draft" --version "2.0.0"`) {
				t.Fatalf("run error = %v, want the unreleased version named", runErr)
			}
		})
	}
}

// --build-id must travel in the create request itself, so a new version needs
// no second attach call.
func TestVersionsCreateSendsBuildRelationship(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--build-id", "build-1", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions" {
			return jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"version-created","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
	var body asc.AppStoreVersionCreateRequest
	if err := json.Unmarshal([]byte(seen[0].Body), &body); err != nil {
		t.Fatalf("unmarshal POST body: %v; body=%q", err, seen[0].Body)
	}
	relationships := body.Data.Relationships
	if relationships == nil || relationships.App == nil || relationships.App.Data.ID != "app-1" {
		t.Fatalf("POST body = %s, want the app relationship kept", seen[0].Body)
	}
	if relationships.Build == nil || relationships.Build.Data.Type != asc.ResourceTypeBuilds || relationships.Build.Data.ID != "build-1" {
		t.Fatalf("POST body = %s, want build relationship for build-1", seen[0].Body)
	}
	if !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("stdout = %q, want buildId in the receipt", stdout)
	}
}

// A conflict sends nothing the existing version can use from the POST, so
// --if-exists update has to attach the build itself or the flag is lost.
func TestVersionsCreateIfExistsUpdateAttachesBuild(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--build-id", "build-1",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing/relationships/build":
			if !strings.Contains(req.Body, `"type":"builds"`) || !strings.Contains(req.Body, `"id":"build-1"`) {
				t.Fatalf("PATCH body = %s, want build-1", req.Body)
			}
			return jsonResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	var result struct {
		ID            string `json:"id"`
		BuildID       string `json:"buildId"`
		AlreadyExists bool   `json:"alreadyExists"`
		Action        string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.ID != "version-existing" || result.BuildID != "build-1" || !result.AlreadyExists || result.Action != "updated" {
		t.Fatalf("receipt = %+v, want existing id, build-1, alreadyExists=true, action=updated", result)
	}
	if !strings.Contains(stderr, "updated it in place") {
		t.Fatalf("stderr = %q, want updated diagnostic", stderr)
	}
}

// A failed attach must not print a receipt that claims the build landed.
func TestVersionsCreateIfExistsUpdateFailsWhenAttachFails(t *testing.T) {
	stdout, _, _, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--build-id", "build-1",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing/relationships/build":
			return jsonResponse(http.StatusConflict, versionsState409)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "attach build build-1 to existing version version-existing") {
		t.Fatalf("run error = %v, want the attach failure", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no receipt", stdout)
	}
}

// Apple's 409 on POST /v1/appStoreVersions for a --build-id it rejects, seen
// live on 2026-11-03 against a version already in PREPARE_FOR_SUBMISSION: only
// ENTITY_ERROR.RELATIONSHIP.INVALID with this title, no versionString
// DUPLICATE. The detail and pointer here are placeholders.
const versionsBadBuild409 = `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The build relationship is invalid.","source":{"pointer":"/data/relationships/build"}}]}`

// A bad build hides the duplicate code, so update used to stop at Apple's 409
// and tell the user to pass the --if-exists update they already passed. It
// must reach the attach so the build error surfaces, and must not attach when
// no version has that string.
func TestVersionsCreateIfExistsUpdateBadBuildSurfacesBuildError(t *testing.T) {
	for _, tt := range []struct {
		name     string
		readBack string
		want     string
	}{
		{name: "version exists", readBack: existingVersionsList, want: "attach build bad-build to existing version version-existing"},
		{name: "no version", readBack: `{"data":[]}`, want: "The provided entity includes a relationship with an invalid value"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, seen, runErr := runIfExistsCommand(t, []string{
				"versions", "create", "--app", "app-1", "--version", "2.0.0", "--build-id", "bad-build",
				"--if-exists", "update", "--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
					return jsonResponse(http.StatusConflict, versionsBadBuild409)
				case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
					return jsonResponse(http.StatusOK, tt.readBack)
				case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing/relationships/build":
					return jsonResponse(http.StatusConflict, versionsBadBuild409)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.Path)
					return nil, nil
				}
			})
			if runErr == nil || !strings.Contains(runErr.Error(), tt.want) {
				t.Fatalf("run error = %v, want %q", runErr, tt.want)
			}
			if strings.Contains(runErr.Error(), "Reuse it with") {
				t.Fatalf("run error = %v, want no --if-exists hint", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want no receipt", stdout)
			}
			// The version-string read-back runs in both cases; the attach
			// only when it finds the version, the diagnostic listing only
			// when it does not.
			if len(seen) != 3 || seen[1].Method != http.MethodGet || isVersionsCreateDiagnosticListing(seen[1]) {
				t.Fatalf("requests = %+v, want POST, read-back, then attach or listing", seen)
			}
		})
	}
}

// skip leaves an existing version unchanged, and a blank value carries no
// build, so either would drop --build-id without a word.
func TestVersionsCreateRejectsUnusableBuildIDBeforeHTTP(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "if-exists skip", args: []string{"--build-id", "build-1", "--if-exists", "skip"}, want: "--build-id cannot be used with --if-exists skip"},
		{name: "blank", args: []string{"--build-id", "  "}, want: "--build-id must not be empty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"versions", "create", "--app", "app-1", "--version", "2.0.0"}, tt.args...)
			stdout, stderr, seen, runErr := runIfExistsCommand(t, args, func(req ifExistsRequest) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			})
			if !isUsageClassError(runErr) {
				t.Fatalf("run error = %v, want usage-class error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.want)
			}
			if len(seen) != 0 {
				t.Fatalf("requests = %+v, want none", seen)
			}
		})
	}
}

func TestReviewDetailsCreateIfExistsSkipReturnsExistingDetail(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1", "--notes", "new notes",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails":
			return jsonResponse(http.StatusConflict, reviewDetailExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusOK, existingReviewDetail)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then read-back GET", seen)
	}
	if !strings.Contains(stdout, `"id":"detail-existing"`) || !strings.Contains(stdout, `"notes":"old notes"`) {
		t.Fatalf("stdout = %q, want the existing detail envelope unchanged", stdout)
	}
	if !strings.Contains(stderr, "detail-existing") || !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want already-exists diagnostic", stderr)
	}
}

func TestReviewDetailsCreateIfExistsUpdatePatchesExistingDetail(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1",
		"--contact-first-name", "Dev", "--notes", "new notes",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails":
			return jsonResponse(http.StatusConflict, reviewDetailExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusOK, existingReviewDetail)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreReviewDetails/detail-existing":
			if !strings.Contains(req.Body, `"notes":"new notes"`) || !strings.Contains(req.Body, `"contactFirstName":"Dev"`) {
				t.Fatalf("PATCH body = %s, want the same attributes", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"contactFirstName":"Dev","notes":"new notes"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	if !strings.Contains(stdout, `"notes":"new notes"`) {
		t.Fatalf("stdout = %q, want PATCH response", stdout)
	}
}

func TestReviewDetailsCreateIfExistsUpdateReusesExistingDemoCredentials(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1",
		"--demo-account-required=true",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"demoAccountName":"reviewer@example.com","demoAccountPassword":"existing-password","demoAccountRequired":true}}}`)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreReviewDetails/detail-existing":
			if !strings.Contains(req.Body, `"demoAccountRequired":true`) {
				t.Fatalf("PATCH body = %s, want demoAccountRequired=true", req.Body)
			}
			if strings.Contains(req.Body, "demoAccountName") || strings.Contains(req.Body, "demoAccountPassword") {
				t.Fatalf("PATCH body = %s, must not resend existing credentials", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"demoAccountRequired":true}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want read-back GET then PATCH", seen)
	}
	if !strings.Contains(stdout, `"id":"detail-existing"`) {
		t.Fatalf("stdout = %q, want updated detail", stdout)
	}
}

// A metadata copy that changes nothing leaves the existing version untouched,
// so the receipt must stay skipped rather than claim an update.
func TestVersionsCreateIfExistsUpdateKeepsSkippedWhenMetadataCopyChangesNothing(t *testing.T) {
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--copy-metadata-from", "1.9.0", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			if strings.Contains(req.Query, "filter%5BversionString%5D=1.9.0") {
				return jsonResponse(http.StatusOK, sourceVersionsList)
			}
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-source/appStoreVersionLocalizations":
			// The source locale carries none of the copyable fields.
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source-en","attributes":{"locale":"en-US"}}]}`)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-existing/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, destinationLocalizationsList)
		default:
			t.Fatalf("unexpected request %s %s?%s", req.Method, req.Path, req.Query)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	var result struct {
		Action       string `json:"action"`
		MetadataCopy *struct {
			CopiedFieldUpdates int `json:"copiedFieldUpdates"`
		} `json:"metadataCopy"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.MetadataCopy == nil || result.MetadataCopy.CopiedFieldUpdates != 0 {
		t.Fatalf("metadataCopy = %+v, want a copy that applied no field updates", result.MetadataCopy)
	}
	if result.Action != "skipped" {
		t.Fatalf("action = %q, want skipped when the copy changed nothing", result.Action)
	}
	if !strings.Contains(stderr, "left unchanged") {
		t.Fatalf("stderr = %q, want the left-unchanged diagnostic", stderr)
	}
}
