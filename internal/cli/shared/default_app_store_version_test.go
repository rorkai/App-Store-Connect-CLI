package shared

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func defaultVersionTestClient(t *testing.T, pages map[string]string, log *[]string) *asc.Client {
	t.Helper()
	return newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		query := req.URL.Query()
		key := query.Get("filter[appStoreState]") + "|" + query.Get("filter[appVersionState]") + "|" + query.Get("filter[platform]")
		if log != nil {
			*log = append(*log, key)
		}
		body, ok := pages[key]
		if !ok {
			t.Fatalf("unexpected version query %q", key)
		}
		return appResolutionJSONResponse(body)
	})
}

const defaultVersionEditableFilter = "DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED,WAITING_FOR_REVIEW"

func TestResolveDefaultAppStoreVersionPrefersNewestEditableVersion(t *testing.T) {
	var log []string
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[
			{"type":"appStoreVersions","id":"ver-old","attributes":{"versionString":"1.1.0","platform":"IOS","appVersionState":"DEVELOPER_REJECTED","createdDate":"2026-01-01T00:00:00Z"}},
			{"type":"appStoreVersions","id":"ver-new","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, &log)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-new" || resolved.VersionString != "1.2.3" || resolved.Platform != "IOS" || resolved.State != "PREPARE_FOR_SUBMISSION" || resolved.Source != DefaultAppStoreVersionSourceEditable {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
	if len(log) != 1 {
		t.Fatalf("expected one versions request, got %v", log)
	}
	want := "Using version 1.2.3 (PREPARE_FOR_SUBMISSION) for platform IOS; pass --version to override"
	if got := resolved.Note("--version"); got != want {
		t.Fatalf("Note() = %q, want %q", got, want)
	}
}

func TestResolveDefaultAppStoreVersionFallsBackToLiveVersion(t *testing.T) {
	var log []string
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[],"links":{"next":""}}`,
		"READY_FOR_SALE||": `{"data":[
			{"type":"appStoreVersions","id":"ver-live-old","attributes":{"versionString":"1.0.0","platform":"IOS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2025-01-01T00:00:00Z"}},
			{"type":"appStoreVersions","id":"ver-live","attributes":{"versionString":"1.1.0","platform":"IOS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2025-06-01T00:00:00Z"}}
		],"links":{"next":""}}`,
		"|READY_FOR_DISTRIBUTION|": `{"data":[],"links":{"next":""}}`,
	}, &log)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-live" || resolved.VersionString != "1.1.0" || resolved.State != "READY_FOR_DISTRIBUTION" || resolved.Source != DefaultAppStoreVersionSourceLive {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
	if len(log) != 3 {
		t.Fatalf("expected editable then both live requests, got %v", log)
	}
}

// READY_FOR_REVIEW is a draft whose metadata is still editable, so it must beat
// an older live version rather than losing to it.
func TestResolveDefaultAppStoreVersionTreatsReadyForReviewAsEditable(t *testing.T) {
	var log []string
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[
			{"type":"appStoreVersions","id":"ver-draft","attributes":{"versionString":"2.0.0","platform":"IOS","appVersionState":"READY_FOR_REVIEW","createdDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":""}}`,
		"READY_FOR_SALE||": `{"data":[
			{"type":"appStoreVersions","id":"ver-live","attributes":{"versionString":"1.0.0","platform":"IOS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2025-01-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, &log)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-draft" || resolved.State != "READY_FOR_REVIEW" || resolved.Source != DefaultAppStoreVersionSourceEditable {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
	if len(log) != 1 {
		t.Fatalf("expected only the editable request, got %v", log)
	}
}

// The editable preference is app-wide: a single editable version resolves even
// when another platform is live, because the editable version is the one being
// worked on. Failing with an ambiguity error here would reintroduce the
// friction this default exists to remove, so the live tier is never queried.
func TestResolveDefaultAppStoreVersionPrefersEditableOverLiveOnAnotherPlatform(t *testing.T) {
	var log []string
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[
			{"type":"appStoreVersions","id":"ver-ios","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":""}}`,
		"READY_FOR_SALE||": `{"data":[
			{"type":"appStoreVersions","id":"ver-mac-live","attributes":{"versionString":"9.0.0","platform":"MAC_OS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-03-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, &log)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-ios" || resolved.Platform != "IOS" || resolved.Source != DefaultAppStoreVersionSourceEditable {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
	if len(log) != 1 {
		t.Fatalf("expected only the editable request, got %v", log)
	}
	// The cross-platform choice is announced rather than silent.
	if got := resolved.Note("--version"); !strings.Contains(got, "for platform IOS") {
		t.Fatalf("Note() = %q, want it to name the resolved platform", got)
	}
}

func TestResolveDefaultAppStoreVersionHonorsPlatformFilter(t *testing.T) {
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|MAC_OS": `{"data":[
			{"type":"appStoreVersions","id":"ver-mac","attributes":{"versionString":"2.0.0","platform":"MAC_OS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, nil)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "MAC_OS")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-mac" || resolved.Platform != "MAC_OS" {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

func TestResolveDefaultAppStoreVersionRequiresPlatformWhenAmbiguous(t *testing.T) {
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[
			{"type":"appStoreVersions","id":"ver-ios","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}},
			{"type":"appStoreVersions","id":"ver-mac","attributes":{"versionString":"2.0.0","platform":"MAC_OS","appVersionState":"DEVELOPER_REJECTED","createdDate":"2026-01-15T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, nil)

	_, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	var ambiguous *AmbiguousDefaultAppStoreVersionError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousDefaultAppStoreVersionError, got %v", err)
	}
	message := err.Error()
	for _, want := range []string{"--platform", "IOS 1.2.3 (PREPARE_FOR_SUBMISSION)", "MAC_OS 2.0.0 (DEVELOPER_REJECTED)"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in error %q", want, message)
		}
	}
}

func TestResolveDefaultAppStoreVersionErrorsWhenNoVersionExists(t *testing.T) {
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[],"links":{"next":""}}`,
		"READY_FOR_SALE||":                       `{"data":[],"links":{"next":""}}`,
		"|READY_FOR_DISTRIBUTION|":               `{"data":[],"links":{"next":""}}`,
	}, nil)

	_, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err == nil {
		t.Fatal("expected error")
	}
	var ambiguous *AmbiguousDefaultAppStoreVersionError
	if errors.As(err, &ambiguous) {
		t.Fatalf("expected a not-found error, got ambiguity: %v", err)
	}
	if !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound, got %v", err)
	}
	for _, want := range []string{"no editable or live App Store version", `"app-1"`, "--version"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in error %q", want, err.Error())
		}
	}
}

// Apple returns appStoreState and appVersionState inconsistently, and the
// READY_FOR_DISTRIBUTION-to-READY_FOR_SALE remapping is client-side only, so a
// live version exposed under only the modern spelling must still resolve.
func TestResolveDefaultAppStoreVersionFindsLiveVersionByModernState(t *testing.T) {
	var log []string
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[],"links":{"next":""}}`,
		"READY_FOR_SALE||":                       `{"data":[],"links":{"next":""}}`,
		"|READY_FOR_DISTRIBUTION|": `{"data":[
			{"type":"appStoreVersions","id":"ver-modern","attributes":{"versionString":"3.0.0","platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-01-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, &log)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-modern" || resolved.VersionString != "3.0.0" || resolved.Source != DefaultAppStoreVersionSourceLive {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

// A version Apple reports under both live spellings must be counted once,
// otherwise the duplicate would look like a second candidate.
func TestResolveDefaultAppStoreVersionDeduplicatesLiveCandidates(t *testing.T) {
	const both = `{"data":[
		{"type":"appStoreVersions","id":"ver-live","attributes":{"versionString":"1.0.0","platform":"IOS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2025-01-01T00:00:00Z"}}
	],"links":{"next":""}}`
	client := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[],"links":{"next":""}}`,
		"READY_FOR_SALE||":                       both,
		"|READY_FOR_DISTRIBUTION|":               both,
	}, nil)

	resolved, err := ResolveDefaultAppStoreVersion(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("ResolveDefaultAppStoreVersion() error: %v", err)
	}
	if resolved.ID != "ver-live" {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}
