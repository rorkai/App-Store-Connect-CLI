package asc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRawRequestSendsBodyAndReturnsResponseUnmodified(t *testing.T) {
	const requestBody = `{"data":{"type":"betaGroups","attributes":{"name":"QA"}}}`
	const responseBody = `{"data":{"type":"betaGroups","id":"g1"},"links":{"self":"x"}}`
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/betaGroups" {
			t.Fatalf("request = %s %s, want POST /v1/betaGroups", req.Method, req.URL.Path)
		}
		data, _ := io.ReadAll(req.Body)
		if string(data) != requestBody {
			t.Fatalf("body = %q, want %q", data, requestBody)
		}
		assertAuthorized(t, req)
	}, jsonResponse(http.StatusCreated, responseBody))

	got, err := client.RawRequest(context.Background(), http.MethodPost, "/v1/betaGroups", []byte(requestBody))
	if err != nil {
		t.Fatalf("RawRequest() error: %v", err)
	}
	if string(got) != responseBody {
		t.Fatalf("RawRequest() = %q, want %q", got, responseBody)
	}
}

func TestRawRequestRejectsQueryOnMutation(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
	}, jsonResponse(http.StatusOK, `{}`))

	_, err := client.RawRequest(context.Background(), http.MethodDelete, "/v1/betaGroups/g1?x=1", nil)
	if err == nil || !strings.Contains(err.Error(), "must not contain a query string") {
		t.Fatalf("RawRequest() error = %v, want query rejection", err)
	}
}

func TestRawPaginatedGETMergesPagesAndDropsNext(t *testing.T) {
	client := newTestClient(
		t, nil,
		jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1"}],"included":[{"type":"builds","id":"b1"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?limit=1","next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=1"},"meta":{"paging":{"total":2}}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"2"}],"included":[{"type":"builds","id":"b1"},{"type":"builds","id":"b2"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=1"},"meta":{"paging":{"total":2}}}`),
	)

	got, err := client.RawPaginatedGET(context.Background(), "/v1/apps?limit=1", nil)
	if err != nil {
		t.Fatalf("RawPaginatedGET() error: %v", err)
	}
	want := `{"data":[{"type":"apps","id":"1"},{"type":"apps","id":"2"}],"included":[{"type":"builds","id":"b1"},{"type":"builds","id":"b2"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?limit=1"},"meta":{"paging":{"total":2}}}`
	if string(got) != want {
		t.Fatalf("RawPaginatedGET() = %s, want %s", got, want)
	}
}

func TestRawPaginatedGETRequiresDataArray(t *testing.T) {
	// A null data member is an empty to-one linkage, not a collection, so it
	// must not be coerced into an empty data array.
	bodies := map[string]string{
		"to-one object": `{"data":{"type":"apps","id":"1"}}`,
		"null data":     `{"data":null}`,
		"missing data":  `{"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/1"}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			client := newTestClient(t, nil, jsonResponse(http.StatusOK, body))

			_, err := client.RawPaginatedGET(context.Background(), "/v1/apps/1", nil)
			if !errors.Is(err, ErrRawPaginationNotCollection) {
				t.Fatalf("RawPaginatedGET() error = %v, want ErrRawPaginationNotCollection", err)
			}
		})
	}
}

func TestRawPaginatedGETAcceptsEmptyDataArray(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(http.StatusOK, `{"data":[]}`))

	got, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if err != nil {
		t.Fatalf("RawPaginatedGET() error = %v", err)
	}
	if string(got) != `{"data":[]}` {
		t.Fatalf("RawPaginatedGET() = %s, want empty collection envelope", got)
	}
}

func TestRawPaginatedGETRejectsMalformedNextLink(t *testing.T) {
	// A non-string links.next must fail loudly; treating it as an absent page
	// would exit 0 with a silently truncated collection.
	client := newTestClient(
		t, nil,
		jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1"}],"links":{"next":42}}`),
	)

	_, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if err == nil {
		t.Fatal("RawPaginatedGET() error = nil, want malformed links.next error")
	}
	if !strings.Contains(err.Error(), "links.next") {
		t.Fatalf("RawPaginatedGET() error = %v, want it to name links.next", err)
	}
}

func TestRawPaginatedGETTreatsNullNextAsLastPage(t *testing.T) {
	client := newTestClient(
		t, nil,
		jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1"}],"links":{"next":null}}`),
	)

	got, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if err != nil {
		t.Fatalf("RawPaginatedGET() error = %v", err)
	}
	if !strings.Contains(string(got), `"id":"1"`) {
		t.Fatalf("RawPaginatedGET() = %s, want the single page", got)
	}
}

func TestRawPaginatedGETRejectsForeignNextHost(t *testing.T) {
	requests := 0
	client := newTestClient(
		t, func(req *http.Request) {
			requests++
		},
		jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"https://evil.example.com/v1/apps?cursor=two"}}`),
		jsonResponse(http.StatusOK, `{"data":[]}`),
	)

	_, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if err == nil || !strings.Contains(err.Error(), "untrusted host") {
		t.Fatalf("RawPaginatedGET() error = %v, want untrusted host rejection", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want the foreign next link to stop pagination", requests)
	}
}

func TestRawPaginatedGETStopsOnRepeatedNextURL(t *testing.T) {
	const loop = `{"data":[{"type":"apps","id":"1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=loop"}}`
	client := newTestClient(
		t, nil,
		jsonResponse(http.StatusOK, loop),
		jsonResponse(http.StatusOK, loop),
		jsonResponse(http.StatusOK, loop),
	)

	_, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("RawPaginatedGET() error = %v, want ErrRepeatedPaginationURL", err)
	}
}
