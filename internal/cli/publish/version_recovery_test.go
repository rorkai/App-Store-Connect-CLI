package publish

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

func TestFindOrCreatePublishAppStoreVersionUsesFreshCreateDeadline(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_MAX_RETRIES", "0")

	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) != 500*time.Millisecond {
				t.Fatalf("expected lookup request deadline, remaining=%s", time.Until(deadline))
			}
			time.Sleep(300 * time.Millisecond)
			return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersions":
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) != 500*time.Millisecond {
				t.Fatalf("expected fresh create request deadline, remaining=%s", time.Until(deadline))
			}
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	client := newPublishCommandTestClient(t)

	var result *asc.AppStoreVersionResponse
	var err error
	synctest.Test(t, func(*testing.T) {
		result, err = findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	})
	if err != nil || result.Data.ID != "version-1" {
		t.Fatalf("unexpected version result: result=%+v err=%v", result, err)
	}
}

func TestFindOrCreatePublishAppStoreVersionReadsTwiceBeforeReplay(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	sequence := make([]string, 0, 5)
	reads := 0
	creates := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			reads++
			sequence = append(sequence, "read")
			return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersions":
			creates++
			sequence = append(sequence, "create")
			if creates == 1 {
				return publishCommandJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`)
			}
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	client := newPublishCommandTestClient(t)

	result, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	if err != nil || result.Data.ID != "version-1" {
		t.Fatalf("unexpected version result: result=%+v err=%v", result, err)
	}
	wantSequence := []string{"read", "create", "read", "read", "create"}
	if !reflect.DeepEqual(sequence, wantSequence) {
		t.Fatalf("expected bounded replay sequence %v, got %v", wantSequence, sequence)
	}
}

func TestFindOrCreatePublishAppStoreVersionReconcilesAmbiguousCreate(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_MAX_RETRIES", "1")

	reads := 0
	creates := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			reads++
			if reads == 1 {
				return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 350*time.Millisecond {
				t.Fatalf("expected fresh readback deadline, remaining=%s", time.Until(deadline))
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersions":
			creates++
			return publishCommandJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	client := newPublishCommandTestClient(t)

	result, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	if err != nil || result.Data.ID != "version-1" || reads != 2 || creates != 1 {
		t.Fatalf("unexpected reconcile result: result=%+v reads=%d creates=%d err=%v", result, reads, creates, err)
	}
}

func TestFindOrCreatePublishAppStoreVersionReturnsNormalizedExistingMatch(t *testing.T) {
	requests := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
		if got := req.URL.Query().Get("filter[versionString]"); got != "1.2.3" {
			t.Fatalf("expected normalized version filter, got %q", got)
		}
		if got := req.URL.Query().Get("filter[platform]"); got != "IOS" {
			t.Fatalf("expected normalized platform filter, got %q", got)
		}
		return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
	}))
	client := newPublishCommandTestClient(t)

	result, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, " app-1 ", " 1.2.3 ", asc.Platform(" ios "))
	if err != nil || result.Data.ID != "version-existing" || requests != 1 {
		t.Fatalf("unexpected existing result: result=%+v requests=%d err=%v", result, requests, err)
	}
}

func TestFindOrCreatePublishAppStoreVersionRejectsMultipleExactMatches(t *testing.T) {
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
		}
		return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1"},{"type":"appStoreVersions","id":"version-2"}]}`)
	}))
	client := newPublishCommandTestClient(t)

	_, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	if err == nil || !strings.Contains(err.Error(), `2 app store versions match version "1.2.3" on platform "IOS":`) {
		t.Fatalf("expected multiple-match error, got %v", err)
	}
}

func TestFindOrCreatePublishAppStoreVersionRejectsSingleIncompleteMatch(t *testing.T) {
	requests := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
		}
		return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=next"}}`)
	}))
	client := newPublishCommandTestClient(t)

	_, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	if err == nil || !strings.Contains(err.Error(), "sample matches") {
		t.Fatalf("expected incomplete-page ambiguity, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one read and no create, got %d requests", requests)
	}
}

func TestFindOrCreatePublishAppStoreVersionPreservesNonTransientCreateError(t *testing.T) {
	reads := 0
	creates := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			reads++
			return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
		case http.MethodPost:
			creates++
			return publishCommandJSONResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"invalid version"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	client := newPublishCommandTestClient(t)

	_, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected typed 422 error, got %T %v", err, err)
	}
	if reads != 2 || creates != 1 {
		t.Fatalf("expected one readback and no replay, reads=%d creates=%d", reads, creates)
	}
}

func TestFindOrCreatePublishAppStoreVersionExplainsRefusedCreate(t *testing.T) {
	fixture := handlertest.New(t)
	creates := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" && req.URL.Query().Has("filter[versionString]"):
			return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-122","attributes":{"platform":"IOS","versionString":"1.2.2","appVersionState":"WAITING_FOR_REVIEW","appStoreState":"WAITING_FOR_REVIEW"}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersions":
			creates++
			return publishCommandJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state."}]}`)
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	client := newPublishCommandTestClient(t)

	_, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	if !errors.Is(err, asc.ErrConflict) {
		t.Fatalf("error = %v, want App Store Connect's 409", err)
	}
	want := "\n  1.2.2  IOS  WAITING_FOR_REVIEW  version-122\n" +
		`Version 1.2.2 is WAITING_FOR_REVIEW; wait until review finishes or it is released, or inspect it: asc versions view --version-id "version-122"`
	if !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("error =\n%s\nwant suffix%s", err.Error(), want)
	}
	if creates != 1 {
		t.Fatalf("creates = %d, want one and no replay", creates)
	}
}

func TestFindOrCreatePublishAppStoreVersionSkipsDiagnosticsAfterFailedReadBack(t *testing.T) {
	fixture := handlertest.New(t)
	lookups := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" && req.URL.Query().Has("filter[versionString]"):
			lookups++
			if lookups == 1 {
				return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return publishCommandJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"not allowed"}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersions":
			return publishCommandJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state."}]}`)
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))
	client := newPublishCommandTestClient(t)

	_, err := findOrCreatePublishAppStoreVersion(context.Background(), newPublishTestDeps(), client, "app-1", "1.2.3", asc.PlatformIOS)
	if !errors.Is(err, asc.ErrConflict) {
		t.Fatalf("error = %v, want App Store Connect's 409", err)
	}
	if lookups != 2 {
		t.Fatalf("version lookups = %d, want the lookup and one read-back", lookups)
	}
}
