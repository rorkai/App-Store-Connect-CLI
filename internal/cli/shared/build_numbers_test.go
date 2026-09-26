package shared

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func resetEquivalentVersionNotes() {
	equivalentVersionNoteMu.Lock()
	defer equivalentVersionNoteMu.Unlock()

	equivalentVersionNotes = map[string]struct{}{}
}

func TestParseBuildNumberRejectsNonNumeric(t *testing.T) {
	_, err := parseBuildNumber("1a", "processed build")
	if err == nil {
		t.Fatal("expected error for non-numeric build number")
	}
	if !strings.Contains(err.Error(), "processed build") {
		t.Fatalf("expected error to mention source, got %v", err)
	}
}

func TestParseBuildNumberRejectsEmpty(t *testing.T) {
	_, err := parseBuildNumber(" ", "build upload")
	if err == nil {
		t.Fatal("expected error for empty build number")
	}
}

func TestParseBuildNumberAllowsNumeric(t *testing.T) {
	got, err := parseBuildNumber("42", "processed build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "42" {
		t.Fatalf("expected 42, got %q", got.String())
	}
}

func TestParseBuildNumberAllowsDotSeparatedNumeric(t *testing.T) {
	got, err := parseBuildNumber("1.2.3", "build upload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", got.String())
	}
}

func TestBuildNumberNextIncrementsLastSegment(t *testing.T) {
	parsed, err := parseBuildNumber("1.2.3", "processed build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	next, err := parsed.Next()
	if err != nil {
		t.Fatalf("unexpected error incrementing build number: %v", err)
	}
	if next.String() != "1.2.4" {
		t.Fatalf("expected next build number 1.2.4, got %q", next.String())
	}
}

func TestBuildNumberCompareUsesNumericComponents(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "dotted numeric order", left: "1.10", right: "1.9", want: 1},
		{name: "missing trailing zero is equal", left: "1.2", right: "1.2.0", want: 0},
		{name: "first component wins", left: "10", right: "9.99", want: 1},
		{name: "lower component", left: "2.3.3", right: "2.3.4", want: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, err := parseBuildNumber(test.left, "left")
			if err != nil {
				t.Fatalf("parse left: %v", err)
			}
			right, err := parseBuildNumber(test.right, "right")
			if err != nil {
				t.Fatalf("parse right: %v", err)
			}
			if got := left.Compare(right); got != test.want {
				t.Fatalf("Compare(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestParseBuildTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "rfc3339", input: "2026-02-10T08:00:00Z"},
		{name: "rfc3339nano", input: "2026-02-10T08:00:00.123456789Z"},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid", input: "2026/02/10", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseBuildTimestamp(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.IsZero() {
				t.Fatal("expected non-zero parsed timestamp")
			}
		})
	}
}

func TestVersionQueryVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: nil},
		{name: "whitespace", input: "   ", want: nil},
		{name: "three segments with trailing zero", input: "1.2.0", want: []string{"1.2.0", "1.2"}},
		{name: "two segments", input: "1.2", want: []string{"1.2", "1.2.0"}},
		{name: "two segments with zero minor", input: "1.0", want: []string{"1.0", "1.0.0"}},
		{name: "three significant segments", input: "1.2.3", want: []string{"1.2.3"}},
		{name: "leading zero segment preserved", input: "1.02.0", want: []string{"1.02.0", "1.02"}},
		{name: "leading zero without counterpart preserved", input: "1.02.3", want: []string{"1.02.3"}},
		{name: "single segment", input: "1", want: []string{"1"}},
		{name: "surrounding whitespace", input: "  1.2  ", want: []string{"1.2", "1.2.0"}},
		{name: "four segments", input: "1.2.3.4", want: []string{"1.2.3.4"}},
		{name: "non numeric", input: "1.2.0-beta", want: []string{"1.2.0-beta"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := versionQueryVariants(test.input)
			if !slices.Equal(got, test.want) {
				t.Fatalf("versionQueryVariants(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func newPreReleaseVersionLookupClient(t *testing.T, storedVersion string, calls *[]string) *asc.Client {
	t.Helper()

	return newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/preReleaseVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		requested := req.URL.Query().Get("filter[version]")
		*calls = append(*calls, requested)
		if requested != storedVersion {
			return buildWaitJSONResponse(`{"data": [], "links": {}}`)
		}
		return buildWaitJSONResponse(fmt.Sprintf(`{
			"data": [
				{
					"type": "preReleaseVersions",
					"id": "prv-1",
					"attributes": {"version": %q, "platform": "IOS"}
				}
			],
			"links": {}
		}`, storedVersion))
	})
}

func TestFindPreReleaseVersionIDsMatchesEquivalentVersionFormat(t *testing.T) {
	resetEquivalentVersionNotes()

	var calls []string
	client := newPreReleaseVersionLookupClient(t, "1.2", &calls)

	var ids []string
	var err error
	stderr := captureStderr(t, func() {
		ids, err = FindPreReleaseVersionIDs(context.Background(), client, "app-1", "1.2.0", "IOS")
	})
	if err != nil {
		t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
	}
	if !slices.Equal(ids, []string{"prv-1"}) {
		t.Fatalf("expected [prv-1], got %v", ids)
	}
	if !slices.Equal(calls, []string{"1.2.0", "1.2"}) {
		t.Fatalf("expected requested format to be queried first, got %v", calls)
	}
	if !strings.Contains(stderr, `note: matched version "1.2" for requested "1.2.0"`) {
		t.Fatalf("expected equivalent version note, got %q", stderr)
	}
}

func TestFindPreReleaseVersionIDsPrefersRequestedVersionFormat(t *testing.T) {
	resetEquivalentVersionNotes()

	var calls []string
	client := newPreReleaseVersionLookupClient(t, "1.2.0", &calls)

	var ids []string
	var err error
	stderr := captureStderr(t, func() {
		ids, err = FindPreReleaseVersionIDs(context.Background(), client, "app-1", "1.2.0", "IOS")
	})
	if err != nil {
		t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
	}
	if !slices.Equal(ids, []string{"prv-1"}) {
		t.Fatalf("expected [prv-1], got %v", ids)
	}
	if !slices.Equal(calls, []string{"1.2.0"}) {
		t.Fatalf("expected only the requested format to be queried, got %v", calls)
	}
	if strings.Contains(stderr, "note: matched version") {
		t.Fatalf("did not expect an equivalent version note, got %q", stderr)
	}
}

func TestFindPreReleaseVersionIDsCollectsEquivalentFormatsAcrossPlatforms(t *testing.T) {
	resetEquivalentVersionNotes()

	var calls []string
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/preReleaseVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("filter[platform]"); got != "" {
			return nil, fmt.Errorf("filter[platform] = %q, want empty", got)
		}
		version := req.URL.Query().Get("filter[version]")
		calls = append(calls, version)
		switch version {
		case "1.2.0,1.2":
			return buildWaitJSONResponse(`{"data":[{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.0","platform":"IOS"}},{"type":"preReleaseVersions","id":"prv-mac","attributes":{"version":"1.2","platform":"MAC_OS"}}],"links":{}}`)
		default:
			return buildWaitJSONResponse(`{"data":[],"links":{}}`)
		}
	})

	ids, err := FindPreReleaseVersionIDs(context.Background(), client, "app-1", "1.2.0", "")
	if err != nil {
		t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
	}
	if !slices.Equal(ids, []string{"prv-ios", "prv-mac"}) {
		t.Fatalf("expected both platform trains, got %v", ids)
	}
	if !slices.Equal(calls, []string{"1.2.0,1.2"}) {
		t.Fatalf("expected every equivalent format in one query, got %v", calls)
	}
}

func TestFindPreReleaseVersionIDsNotesEquivalentMatchOnlyOnce(t *testing.T) {
	resetEquivalentVersionNotes()

	var calls []string
	client := newPreReleaseVersionLookupClient(t, "1.2", &calls)

	stderr := captureStderr(t, func() {
		for range 3 {
			if _, err := FindPreReleaseVersionIDs(context.Background(), client, "app-1", "1.2.0", "IOS"); err != nil {
				t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
			}
		}
	})
	if got := strings.Count(stderr, "note: matched version"); got != 1 {
		t.Fatalf("expected exactly 1 note across repeated lookups, got %d in %q", got, stderr)
	}
}

func TestFindPreReleaseVersionIDsReportsNoMatchWithoutNote(t *testing.T) {
	resetEquivalentVersionNotes()

	var calls []string
	client := newPreReleaseVersionLookupClient(t, "9.9", &calls)

	var ids []string
	var err error
	stderr := captureStderr(t, func() {
		ids, err = FindPreReleaseVersionIDs(context.Background(), client, "app-1", "1.2.0", "IOS")
	})
	if err != nil {
		t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no matches, got %v", ids)
	}
	if !slices.Equal(calls, []string{"1.2.0", "1.2"}) {
		t.Fatalf("expected both version formats to be queried, got %v", calls)
	}
	if strings.Contains(stderr, "note: matched version") {
		t.Fatalf("did not expect an equivalent version note, got %q", stderr)
	}
}

func TestFindPreReleaseVersionIDsWithoutVersionFilterQueriesOnce(t *testing.T) {
	resetEquivalentVersionNotes()

	var calls []string
	client := newPreReleaseVersionLookupClient(t, "", &calls)

	ids, err := FindPreReleaseVersionIDs(context.Background(), client, "app-1", "", "IOS")
	if err != nil {
		t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
	}
	if !slices.Equal(ids, []string{"prv-1"}) {
		t.Fatalf("expected [prv-1], got %v", ids)
	}
	if !slices.Equal(calls, []string{""}) {
		t.Fatalf("expected a single unfiltered query, got %v", calls)
	}
}

func TestFindPreReleaseVersionIDsDeduplicatesAndSkipsBlankIDs(t *testing.T) {
	resetEquivalentVersionNotes()

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/preReleaseVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildWaitJSONResponse(`{
			"data": [
				{"type": "preReleaseVersions", "id": "prv-1", "attributes": {"version": "1.2.3"}},
				{"type": "preReleaseVersions", "id": " prv-1 ", "attributes": {"version": "1.2.3"}},
				{"type": "preReleaseVersions", "id": "", "attributes": {"version": "1.2.3"}}
			],
			"links": {}
		}`)
	})

	ids, err := FindPreReleaseVersionIDs(context.Background(), client, "app-1", "1.2.3", "IOS")
	if err != nil {
		t.Fatalf("FindPreReleaseVersionIDs() error: %v", err)
	}
	if !slices.Equal(ids, []string{"prv-1"}) {
		t.Fatalf("expected one normalized ID, got %v", ids)
	}
}

func TestFindPreReleaseVersionIDsWithMaxPagesStopsBeforeFetchingBeyondLimit(t *testing.T) {
	calls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/preReleaseVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildWaitJSONResponse(fmt.Sprintf(
			`{"data":[],"links":{"next":"/v1/preReleaseVersions?cursor=%d"}}`,
			calls,
		))
	})

	_, err := FindPreReleaseVersionIDsWithMaxPages(
		context.Background(), client, "app-1", "1.2.3", "IOS", 2,
	)
	if err == nil || !strings.Contains(err.Error(), "page 3") || !strings.Contains(err.Error(), "2-page safety limit") {
		t.Fatalf("expected page-limit error for page 3, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("requests = %d, want 2 before limit rejection", calls)
	}
}

func TestResolveLatestBuildSelectionCarriesEveryEquivalentUploadVersion(t *testing.T) {
	tests := []struct {
		name       string
		exactTrain bool
	}{
		{name: "exact train exists", exactTrain: true},
		{name: "no train exists yet"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/preReleaseVersions":
					if test.exactTrain && req.URL.Query().Get("filter[version]") == "1.2.0" {
						return buildWaitJSONResponse(`{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.0","platform":"IOS"}}],"links":{}}`)
					}
					return buildWaitJSONResponse(`{"data":[],"links":{}}`)
				case "/v1/builds":
					return buildWaitJSONResponse(`{"data":[],"links":{}}`)
				default:
					return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
				}
			})

			selection, err := resolveLatestBuildSelection(context.Background(), client, LatestBuildSelectionOptions{
				AppID:    "123456789",
				Version:  "1.2.0",
				Platform: "IOS",
			}, true)
			if err != nil {
				t.Fatalf("resolveLatestBuildSelection() error: %v", err)
			}
			if !slices.Equal(selection.BuildUploadVersions, []string{"1.2.0", "1.2"}) {
				t.Fatalf("BuildUploadVersions = %v, want every equivalent spelling", selection.BuildUploadVersions)
			}
		})
	}
}

func TestFindMostRecentlyUploadedBuildPropagatesLaterHTTPErrorAfterOrderingAnomaly(t *testing.T) {
	const (
		pageTwoURL   = "https://api.appstoreconnect.apple.com/v1/builds?cursor=page-two"
		pageThreeURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=page-three"
	)

	requestCount := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return buildWaitJSONResponse(`{
				"data": [{"type":"builds","id":"build-old","attributes":{"version":"1","uploadedDate":"2026-09-01T00:00:00Z"}}],
				"links": {"next": "` + pageTwoURL + `"}
			}`)
		case 2:
			return buildWaitJSONResponse(`{
				"data": [{"type":"builds","id":"build-new","attributes":{"version":"2","uploadedDate":"2026-09-02T00:00:00Z"}}],
				"links": {"next": "` + pageThreeURL + `"}
			}`)
		case 3:
			return buildWaitJSONStatusResponse(http.StatusBadRequest, `{"errors":[{"code":"BAD_REQUEST","detail":"later page unavailable"}]}`)
		default:
			t.Fatalf("unexpected request #%d: %s", requestCount, req.URL.String())
			return nil, nil
		}
	})

	got, err := findMostRecentlyUploadedBuild(context.Background(), client, "app-1")
	if err == nil {
		t.Fatal("expected later pagination HTTP error, got nil")
	}
	if !strings.Contains(err.Error(), "later page unavailable") {
		t.Fatalf("expected later page error to be preserved, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected no build alongside pagination error, got %#v", got)
	}
}
