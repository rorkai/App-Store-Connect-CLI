package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's 409 bodies for POST /v1/appStoreVersions, captured live against the
// disposable app on 2026-09-15. A version string that is already used arrives
// as the relationship rejection followed by the duplicate; an app whose
// platform already has an unreleased version gets only the relationship
// rejection.
const (
	diagnosticVersionsDuplicate409    = `{"errors":[{"id":"b068c5c0-b3fa-4d12-aa89-f1b9aa061b28","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}},{"id":"eb1884a7-e427-42db-ac95-26c49c84a5c2","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"The version number has been previously used.","source":{"pointer":"/data/attributes/versionString"}}]}`
	diagnosticVersionsRelationship409 = `{"errors":[{"id":"7c2d9e1f-4b3a-4c6d-8e5f-1a2b3c4d5e6f","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}}]}`
)

// diagnosticVersionsList is a recorded-shape list of an app's IOS versions in
// the order Apple returned them, which is not newest first.
const diagnosticVersionsList = `{"data":[` +
	`{"type":"appStoreVersions","id":"ver-110","attributes":{"versionString":"1.1.0","platform":"IOS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-05-01T10:00:00-07:00"}},` +
	`{"type":"appStoreVersions","id":"ver-120","attributes":{"versionString":"1.2.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-08-01T10:00:00-07:00"}},` +
	`{"type":"appStoreVersions","id":"ver-100","attributes":{"versionString":"1.0.0","platform":"IOS","appStoreState":"REPLACED_WITH_NEW_VERSION","appVersionState":"REPLACED_WITH_NEW_VERSION","createdDate":"2026-01-01T10:00:00-08:00"}}` +
	`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions"}}`

type diagnosticRequest struct {
	Method string
	Path   string
	Query  map[string][]string
}

func newDiagnosticTestClient(t *testing.T, handler func(req diagnosticRequest) (*http.Response, error)) (*asc.Client, *[]diagnosticRequest) {
	t.Helper()
	seen := []diagnosticRequest{}
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		entry := diagnosticRequest{Method: req.Method, Path: req.URL.Path, Query: req.URL.Query()}
		seen = append(seen, entry)
		return handler(entry)
	})
	return client, &seen
}

func diagnosticJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func isDiagnosticListing(req diagnosticRequest) bool {
	_, filtered := req.Query["filter[versionString]"]
	return req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions" && !filtered
}

func TestResolveAppStoreVersionIDAndStateNotFoundListsExistingVersions(t *testing.T) {
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if isDiagnosticListing(req) {
			if got := req.Query["filter[platform]"]; len(got) != 1 || got[0] != "IOS" {
				t.Fatalf("diagnostic listing platform filter = %v, want IOS", got)
			}
			if got := req.Query["limit"]; len(got) != 1 || got[0] != "200" {
				t.Fatalf("diagnostic listing limit = %v, want 200", got)
			}
			return diagnosticJSONResponse(http.StatusOK, diagnosticVersionsList)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.0", "IOS")
	if !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("error = %v, want asc.ErrNotFound", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %+v, want the lookup then one diagnostic listing", *seen)
	}
	want := strings.Join([]string{
		`app store version not found for version "1.0" and platform "IOS"`,
		`Existing App Store versions for app "app-1" on IOS (newest first):`,
		`  1.2.0  IOS  PREPARE_FOR_SUBMISSION     ver-120`,
		`  1.1.0  IOS  READY_FOR_DISTRIBUTION     ver-110`,
		`  1.0.0  IOS  REPLACED_WITH_NEW_VERSION  ver-100`,
		`Retry with one of the listed version strings, or, if editable version 1.2.0 is the release you meant, rename it: asc versions update --version-id "ver-120" --version "1.0"`,
	}, "\n")
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
	}
}

func TestResolveAppStoreVersionIDAndStateNotFoundSuggestsCreateWhenNothingIsEditable(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if isDiagnosticListing(req) {
			return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-110","attributes":{"versionString":"1.1.0","platform":"IOS","appStoreState":"READY_FOR_SALE","createdDate":"2026-05-01T10:00:00-07:00"}}]}`)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "2.0", "IOS")
	if !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("error = %v, want asc.ErrNotFound", err)
	}
	wantTail := `Retry with one of the listed version strings, or create it: asc versions create --app "app-1" --version "2.0" --platform IOS`
	if !strings.HasSuffix(err.Error(), wantTail) {
		t.Fatalf("error =\n%s\nwant suffix\n%s", err.Error(), wantTail)
	}
}

func TestResolveAppStoreVersionIDAndStateNotFoundWithNoVersions(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.0", "MAC_OS")
	if !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("error = %v, want asc.ErrNotFound", err)
	}
	want := strings.Join([]string{
		`app store version not found for version "1.0" and platform "MAC_OS"`,
		`App "app-1" has no App Store versions on MAC_OS. Create one: asc versions create --app "app-1" --version "1.0" --platform MAC_OS`,
	}, "\n")
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
	}
}

func TestAppStoreVersionNotFoundDiagnosticsBoundsListing(t *testing.T) {
	items := make([]string, 0, 12)
	for index := 1; index <= 12; index++ {
		items = append(items, fmt.Sprintf(`{"type":"appStoreVersions","id":"ver-%02d","attributes":{"versionString":"1.%d","platform":"IOS","appStoreState":"REPLACED_WITH_NEW_VERSION","createdDate":"2026-01-%02dT00:00:00Z"}}`, index, index, index))
	}
	body := `{"data":[` + strings.Join(items, ",") + `]}`
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if isDiagnosticListing(req) {
			return diagnosticJSONResponse(http.StatusOK, body)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "9.0", "IOS")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %+v, want one diagnostic page only", *seen)
	}
	message := err.Error()
	if !strings.Contains(message, "  1.12  IOS  REPLACED_WITH_NEW_VERSION  ver-12\n") {
		t.Fatalf("error = %s, want the newest version first", message)
	}
	if strings.Contains(message, "ver-02") || strings.Contains(message, "ver-01") {
		t.Fatalf("error = %s, want only the newest 10 versions", message)
	}
	if !strings.Contains(message, "\n  ... and 2 more; list every version: asc versions list --app \"app-1\" --platform IOS --paginate\n") {
		t.Fatalf("error = %s, want bounded-listing summary", message)
	}
}

func TestAppStoreVersionDiagnosticsReadEveryPageBeforeChoosingNewest(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=page2&filter%5Bplatform%5D=IOS&limit=200"
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if _, filtered := req.Query["filter[versionString]"]; filtered {
			return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
		}
		if _, second := req.Query["cursor"]; second {
			// The newest version, and the only editable one, is on page two.
			return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-new","attributes":{"versionString":"3.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-09-01T00:00:00Z"}}]}`)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-old","attributes":{"versionString":"2.0","platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":"`+nextURL+`"}}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "4.0", "IOS")
	if len(*seen) != 3 {
		t.Fatalf("requests = %+v, want lookup then both listing pages", *seen)
	}
	want := strings.Join([]string{
		`app store version not found for version "4.0" and platform "IOS"`,
		`Existing App Store versions for app "app-1" on IOS (newest first):`,
		`  3.0  IOS  PREPARE_FOR_SUBMISSION  ver-new`,
		`  2.0  IOS  READY_FOR_DISTRIBUTION  ver-old`,
		`Retry with one of the listed version strings, or, if editable version 3.0 is the release you meant, rename it: asc versions update --version-id "ver-new" --version "4.0"`,
	}, "\n")
	if err == nil || err.Error() != want {
		t.Fatalf("error =\n%v\nwant\n%s", err, want)
	}
}

func TestAppStoreVersionDiagnosticsKeepOriginalErrorWhenLaterPageFails(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=page2"
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if _, second := req.Query["cursor"]; second {
			return diagnosticJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"not allowed"}]}`)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-old","attributes":{"versionString":"2.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":"`+nextURL+`"}}`)
	})
	original := asc.ParseErrorWithStatus([]byte(diagnosticVersionsRelationship409), http.StatusConflict)

	err := WithAppStoreVersionCreateConflictDiagnostics(context.Background(), client, "app-1", "3.0", "IOS", original)
	if !errors.Is(err, original) || err.Error() != original.Error() {
		t.Fatalf("error = %v, want original error unchanged when the listing is incomplete", err)
	}
}

func TestAppStoreVersionNotFoundDiagnosticsKeepOriginalErrorWhenListingFails(t *testing.T) {
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if isDiagnosticListing(req) {
			return diagnosticJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"This request is forbidden for security reasons","detail":"The API key in use does not allow this request"}]}`)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.0", "IOS")
	if !errors.Is(err, asc.ErrNotFound) || errors.Is(err, asc.ErrForbidden) {
		t.Fatalf("error = %v, want only the original not-found classification", err)
	}
	if err.Error() != `app store version not found for version "1.0" and platform "IOS"` {
		t.Fatalf("error = %q, want the original message unchanged", err.Error())
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %+v, want lookup then the failed diagnostic listing", *seen)
	}
}

func TestAppStoreVersionNotFoundDiagnosticsListAllPlatformsWithoutPlatform(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if _, filtered := req.Query["filter[platform]"]; filtered {
			t.Fatalf("diagnostic listing query = %v, want no platform filter", req.Query)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[`+
			`{"type":"appStoreVersions","id":"ver-mac","attributes":{"versionString":"3.0","platform":"MAC_OS","appVersionState":"READY_FOR_REVIEW","createdDate":"2026-07-01T00:00:00Z"}},`+
			`{"type":"appStoreVersions","id":"ver-ios","attributes":{"versionString":"2.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-08-01T00:00:00Z"}}`+
			`]}`)
	})

	err := WithAppStoreVersionNotFoundDiagnostics(context.Background(), client, "app-1", "1.0", "", errors.New(`app store version not found for version "1.0"`))
	want := strings.Join([]string{
		`app store version not found for version "1.0"`,
		`Existing App Store versions for app "app-1" (newest first):`,
		`  2.0  IOS     PREPARE_FOR_SUBMISSION  ver-ios`,
		`  3.0  MAC_OS  READY_FOR_REVIEW        ver-mac`,
		`Retry with one of the listed version strings and its platform.`,
	}, "\n")
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
	}
}

func TestAppStoreVersionNotFoundReadDiagnosticsSuggestsOneExactRetry(t *testing.T) {
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if isDiagnosticListing(req) {
			if got := req.Query["filter[platform]"]; len(got) != 1 || got[0] != "IOS" {
				t.Fatalf("diagnostic listing platform filter = %v, want IOS", got)
			}
			return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-100","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}]}`)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndStateForRead(
		context.Background(),
		client,
		"app-1",
		"1.0.0",
		"IOS",
		func(version, platform string) string {
			return fmt.Sprintf("--version %s --platform %s (keep --dir and other flags unchanged)", version, platform)
		},
	)
	want := strings.Join([]string{
		`app store version not found for version "1.0.0" and platform "IOS"`,
		`Existing App Store versions for app "app-1" on IOS (newest first):`,
		`  1.0  IOS  PREPARE_FOR_SUBMISSION  ver-100`,
		`No existing App Store version exactly matches "1.0.0" on IOS; retry with the exact existing version and platform: --version 1.0 --platform IOS (keep --dir and other flags unchanged)`,
	}, "\n")
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err, want)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %+v, want lookup then one diagnostic listing", *seen)
	}
}

func TestAppStoreVersionNotFoundReadDiagnosticsReportsNoExactVersion(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
	})

	err := WithAppStoreVersionNotFoundReadDiagnostics(
		context.Background(),
		client,
		"app-1",
		"1.0.0",
		"",
		func(version, platform string) string {
			return fmt.Sprintf("--version %q --platform %s", version, platform)
		},
		errors.New(`app store version not found for version "1.0.0"`),
	)
	want := "app store version not found for version \"1.0.0\"\nNo existing App Store versions match version \"1.0.0\"; no exact version and platform are available to retry."
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err, want)
	}
}

func TestAppStoreVersionNotFoundReadDiagnosticsUsesCandidatePlatformByDefault(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if _, filtered := req.Query["filter[platform]"]; filtered {
			t.Fatalf("diagnostic listing query = %v, want no platform filter", req.Query)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-100","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}]}`)
	})

	err := WithAppStoreVersionNotFoundReadDiagnostics(
		context.Background(),
		client,
		"app-1",
		"1.0.0",
		"",
		func(version, platform string) string {
			return fmt.Sprintf("--version %s --platform %s (keep --dir and other flags unchanged)", version, platform)
		},
		errors.New(`app store version not found for version "1.0.0"`),
	)
	if !strings.Contains(err.Error(), `No existing App Store version exactly matches "1.0.0"; retry with the exact existing version and platform: --version 1.0 --platform IOS (keep --dir and other flags unchanged)`) {
		t.Fatalf("error =\n%s\nwant exact default-platform retry", err)
	}
}

func TestAppStoreVersionNotFoundReadDiagnosticsUsesOnlyExactProviderValue(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-100","attributes":{"versionString":"1.0\u001b[0m","platform":"IOS"}}]}`)
	})
	err := WithAppStoreVersionNotFoundReadDiagnostics(
		context.Background(), client, "app-1", "1.0.0", "IOS",
		func(version, platform string) string { return "unexpected retry" }, errors.New("version not found"),
	)
	if strings.Contains(err.Error(), "unexpected retry") || !strings.Contains(err.Error(), "no exact version is available to retry") {
		t.Fatalf("expected display-only guidance for a modified provider version, got %v", err)
	}
}

func TestAppStoreVersionNotFoundReadDiagnosticsRefusesOffCollectionPage(t *testing.T) {
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if req.Path != "/v1/apps/app-1/appStoreVersions" {
			t.Fatalf("unexpected request: %+v", req)
		}
		return diagnosticJSONResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/apps/app-2/appStoreVersions?cursor=next"}}`)
	})
	notFound := errors.New("version not found")
	got := WithAppStoreVersionNotFoundReadDiagnostics(
		asc.WithNoRedirectReads(context.Background()), client, "app-1", "1.0.0", "IOS",
		func(version, platform string) string { return "unexpected retry" }, notFound,
	)
	if !errors.Is(got, notFound) || got.Error() != notFound.Error() || len(*seen) != 1 {
		t.Fatalf("error = %v, requests = %v; want original error and only the initial listing", got, *seen)
	}
}

func TestAppStoreVersionDiagnosticsSanitizeAppleText(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-1\u001b[31m","attributes":{"versionString":"1.0\nInjected line","platform":"IOS","appStoreState":"READY_FOR_SALE"}}]}`)
	})

	err := WithAppStoreVersionNotFoundDiagnostics(context.Background(), client, "app-1", "2.0", "IOS", errors.New("not found"))
	message := err.Error()
	if strings.Contains(message, "\u001b") || strings.Contains(message, "\nInjected line") {
		t.Fatalf("error = %q, want Apple-supplied text sanitized onto one row", message)
	}
}

func TestAppStoreVersionCreateConflictDiagnosticsNameInProgressVersion(t *testing.T) {
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		if !isDiagnosticListing(req) {
			t.Fatalf("unexpected request %+v", req)
		}
		if got := req.Query["filter[platform]"]; len(got) != 1 || got[0] != "IOS" {
			t.Fatalf("diagnostic listing platform filter = %v, want IOS", got)
		}
		return diagnosticJSONResponse(http.StatusOK, diagnosticVersionsList)
	})
	original := asc.ParseErrorWithStatus([]byte(diagnosticVersionsRelationship409), http.StatusConflict)

	err := WithAppStoreVersionCreateConflictDiagnostics(context.Background(), client, "app-1", "2.0.0", "IOS", original)
	if !errors.Is(err, asc.ErrConflict) {
		t.Fatalf("error = %v, want the original 409 classification", err)
	}
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict || apiErr.Code != "ENTITY_ERROR.RELATIONSHIP.INVALID" {
		t.Fatalf("error = %v, want the original APIError reachable", err)
	}
	if len(*seen) != 1 {
		t.Fatalf("requests = %+v, want one diagnostic listing", *seen)
	}
	want := strings.Join([]string{
		original.Error(),
		`IOS versions that are not live yet (an unreleased version usually blocks creating another):`,
		`  1.2.0  IOS  PREPARE_FOR_SUBMISSION  ver-120`,
		`To ship "2.0.0" from editable version 1.2.0, rename it: asc versions update --version-id "ver-120" --version "2.0.0"`,
	}, "\n")
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
	}
}

func TestAppStoreVersionCreateConflictDiagnosticsGuidanceByState(t *testing.T) {
	for _, tt := range []struct {
		state string
		want  string
	}{
		{state: "WAITING_FOR_REVIEW", want: `Version 1.2.0 is WAITING_FOR_REVIEW; wait until review finishes or it is released, or inspect it: asc versions view --version-id "ver-120"`},
		{state: "PENDING_DEVELOPER_RELEASE", want: `Version 1.2.0 is approved and waiting for release; release it: asc versions release --version-id "ver-120" --confirm`},
		{state: "REJECTED", want: `To ship "2.0.0" from editable version 1.2.0, rename it: asc versions update --version-id "ver-120" --version "2.0.0"`},
	} {
		t.Run(tt.state, func(t *testing.T) {
			client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
				return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-120","attributes":{"versionString":"1.2.0","platform":"IOS","appVersionState":"`+tt.state+`","createdDate":"2026-08-01T10:00:00-07:00"}}]}`)
			})
			original := asc.ParseErrorWithStatus([]byte(diagnosticVersionsRelationship409), http.StatusConflict)

			err := WithAppStoreVersionCreateConflictDiagnostics(context.Background(), client, "app-1", "2.0.0", "IOS", original)
			if !strings.HasSuffix(err.Error(), "\n"+tt.want) {
				t.Fatalf("error =\n%s\nwant suffix\n%s", err.Error(), tt.want)
			}
		})
	}
}

func TestAppStoreVersionCreateConflictDiagnosticsNameDuplicateVersion(t *testing.T) {
	for _, tt := range []struct {
		name    string
		explain func(context.Context, *asc.Client, string, string, string, error) error
		state   string
		reuse   string
	}{
		{name: "versions create", explain: WithAppStoreVersionCreateConflictDiagnostics, state: "PREPARE_FOR_SUBMISSION", reuse: "Reuse it with --if-exists skip or --if-exists update"},
		{name: "rerun", explain: WithAppStoreVersionCreateConflictRerunDiagnostics, state: "PREPARE_FOR_SUBMISSION", reuse: "Run the command again to use it"},
		{name: "versions create live", explain: WithAppStoreVersionCreateConflictDiagnostics, state: "READY_FOR_DISTRIBUTION", reuse: "Reuse it with --if-exists skip or --if-exists update"},
		{name: "rerun live", explain: WithAppStoreVersionCreateConflictRerunDiagnostics, state: "READY_FOR_DISTRIBUTION", reuse: "Choose a new version number"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
				return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-120","attributes":{"versionString":"1.2.0","platform":"IOS","appVersionState":"`+tt.state+`","createdDate":"2026-08-01T10:00:00-07:00"}}]}`)
			})
			original := asc.ParseErrorWithStatus([]byte(diagnosticVersionsDuplicate409), http.StatusConflict)

			err := tt.explain(context.Background(), client, "app-1", "1.2.0", "IOS", original)
			want := strings.Join([]string{
				original.Error(),
				`Version "1.2.0" already exists on IOS as ver-120 (` + tt.state + `). ` + tt.reuse + `, or inspect it: asc versions view --version-id "ver-120"`,
			}, "\n")
			if err.Error() != want {
				t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
			}
		})
	}
}

func TestAppStoreVersionCreateConflictDiagnosticsWithoutInProgressVersion(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		return diagnosticJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-110","attributes":{"versionString":"1.1.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}]}`)
	})
	original := asc.ParseErrorWithStatus([]byte(diagnosticVersionsRelationship409), http.StatusConflict)

	err := WithAppStoreVersionCreateConflictDiagnostics(context.Background(), client, "app-1", "2.0.0", "IOS", original)
	want := original.Error() + "\n" + `No unreleased IOS version was found; review every version: asc versions list --app "app-1" --platform IOS`
	if err.Error() != want {
		t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
	}
}

func TestAppStoreVersionCreateConflictDiagnosticsSkipOtherFailures(t *testing.T) {
	client, seen := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		t.Fatalf("unexpected diagnostic request %+v", req)
		return nil, nil
	})
	for _, original := range []error{
		asc.ParseErrorWithStatus([]byte(`{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid","detail":"bad version string"}]}`), http.StatusUnprocessableEntity),
		errors.New("network down"),
	} {
		err := WithAppStoreVersionCreateConflictDiagnostics(context.Background(), client, "app-1", "2.0.0", "IOS", original)
		if !errors.Is(err, original) || err.Error() != original.Error() {
			t.Fatalf("error = %v, want original error returned unchanged", err)
		}
	}
	if len(*seen) != 0 {
		t.Fatalf("requests = %+v, want none", *seen)
	}
}

func TestAppStoreVersionCreateConflictDiagnosticsKeepOriginalWhenListingFails(t *testing.T) {
	client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
		return diagnosticJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"not allowed"}]}`)
	})
	original := asc.ParseErrorWithStatus([]byte(diagnosticVersionsRelationship409), http.StatusConflict)

	err := WithAppStoreVersionCreateConflictDiagnostics(context.Background(), client, "app-1", "2.0.0", "IOS", original)
	if !errors.Is(err, original) || err.Error() != original.Error() {
		t.Fatalf("error = %v, want original error returned unchanged", err)
	}
}

func TestAppStoreVersionNotFoundDiagnosticsPointToOtherPlatforms(t *testing.T) {
	const macVersions = `{"data":[` +
		`{"type":"appStoreVersions","id":"ver-mac-2","attributes":{"versionString":"2.0","platform":"MAC_OS","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-08-01T00:00:00Z"}},` +
		`{"type":"appStoreVersions","id":"ver-mac-1","attributes":{"versionString":"1.0","platform":"MAC_OS","appVersionState":"REPLACED_WITH_NEW_VERSION","createdDate":"2026-01-01T00:00:00Z"}}` +
		`]}`
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "version exists on another platform",
			version: "1.0",
			want:    `App "app-1" has no App Store versions on IOS, but version "1.0" exists on MAC_OS. Retry with --platform MAC_OS.`,
		},
		{
			name:    "version exists nowhere",
			version: "3.0",
			want:    `App "app-1" has no App Store versions on IOS; it has versions on MAC_OS. Retry with --platform MAC_OS, or create one on IOS: asc versions create --app "app-1" --version "3.0" --platform IOS`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newDiagnosticTestClient(t, func(req diagnosticRequest) (*http.Response, error) {
				if isDiagnosticListing(req) && len(req.Query["filter[platform]"]) == 0 {
					return diagnosticJSONResponse(http.StatusOK, macVersions)
				}
				return diagnosticJSONResponse(http.StatusOK, `{"data":[]}`)
			})

			_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", test.version, "IOS")
			if !errors.Is(err, asc.ErrNotFound) {
				t.Fatalf("error = %v, want asc.ErrNotFound", err)
			}
			want := fmt.Sprintf("app store version not found for version %q and platform \"IOS\"\n%s", test.version, test.want)
			if err.Error() != want {
				t.Fatalf("error =\n%s\nwant\n%s", err.Error(), want)
			}
		})
	}
}
