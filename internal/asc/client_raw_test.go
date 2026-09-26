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

func TestRawRequestRejectsUnsafeTargetsBeforeTransport(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr string
	}{
		{
			name:    "foreign host",
			target:  "https://evil.example.com/v1/apps",
			wantErr: "untrusted host",
		},
		{
			name:    "insecure scheme",
			target:  "http://api.appstoreconnect.apple.com/v1/apps",
			wantErr: "HTTPS",
		},
		{
			name:    "userinfo",
			target:  "https://user:secret@api.appstoreconnect.apple.com/v1/apps",
			wantErr: "userinfo",
		},
		{
			name:    "fragment",
			target:  "https://api.appstoreconnect.apple.com/v1/apps?cursor=two#fragment",
			wantErr: "fragment",
		},
		{
			name:    "empty fragment marker",
			target:  "https://api.appstoreconnect.apple.com/v1/apps#",
			wantErr: "fragment",
		},
		{
			name:    "unsafe path",
			target:  "https://api.appstoreconnect.apple.com/v1/apps/../bundleIds",
			wantErr: "traversal segment",
		},
		{
			name:    "relative authority",
			target:  "@evil.example/v1/apps",
			wantErr: "start with '/'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			client := newTestClient(t, func(*http.Request) {
				requests++
			}, jsonResponse(http.StatusOK, `{}`))

			_, err := client.RawRequest(context.Background(), http.MethodGet, tt.target, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("RawRequest() error = %v, want error containing %q", err, tt.wantErr)
			}
			if requests != 0 {
				t.Fatalf("transport requests = %d, want 0 for rejected target", requests)
			}
		})
	}
}

func TestRawRequestAllowsASCAbsoluteTargetAndPreservesQuery(t *testing.T) {
	const target = BaseURL + "/v1/apps?cursor=one%2Ftwo&limit=2"
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Host != "api.appstoreconnect.apple.com" || req.URL.Path != "/v1/apps" {
			t.Fatalf("request URL = %s, want ASC /v1/apps target", req.URL)
		}
		if req.URL.RawQuery != "cursor=one%2Ftwo&limit=2" {
			t.Fatalf("request query = %q, want the cursor query unchanged", req.URL.RawQuery)
		}
	}, jsonResponse(http.StatusOK, `{"data":[]}`))

	if _, err := client.RawRequest(context.Background(), http.MethodGet, target, nil); err != nil {
		t.Fatalf("RawRequest() error: %v", err)
	}
}

func TestRawRequestRefusesCrossOrigin307And308WithoutLeak(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			const requestBody = `{"data":{"type":"betaGroups","attributes":{"name":"secret body"}}}`
			requests := 0
			redirect := jsonResponse(status, `{"errors":[{"title":"redirect"}]}`)
			redirect.Header.Set("Location", "https://redirect-attacker.example/capture")
			client := newTestClient(t, func(req *http.Request) {
				requests++
				if requests == 1 {
					if req.URL.Host != "api.appstoreconnect.apple.com" {
						t.Fatalf("initial request host = %q, want ASC API host", req.URL.Host)
					}
					body, _ := io.ReadAll(req.Body)
					if string(body) != requestBody {
						t.Fatalf("initial request body = %q, want %q", body, requestBody)
					}
					return
				}

				body, _ := io.ReadAll(req.Body)
				t.Errorf(
					"cross-origin redirect reached %s with body=%q Referer=%q",
					req.URL.Host,
					body,
					req.Referer(),
				)
			}, redirect, jsonResponse(http.StatusOK, `{}`))

			_, err := client.RawRequest(
				context.Background(),
				http.MethodPost,
				"/v1/betaGroups",
				[]byte(requestBody),
			)
			if err == nil {
				t.Fatal("RawRequest() succeeded after a cross-origin redirect")
			}
			if requests != 1 {
				t.Fatalf("transport requests = %d, want only the original ASC request", requests)
			}
		})
	}
}

func TestRawPaginatedGETRejectsRelativeAuthorityBeforeTransport(t *testing.T) {
	const firstPage = `{"data":[],"links":{"next":"@evil.example/v1/apps"}}`
	requests := 0
	client := newTestClient(t, func(req *http.Request) {
		requests++
		if req.URL.Host != "api.appstoreconnect.apple.com" {
			t.Fatalf("request host = %q, want the ASC API host", req.URL.Host)
		}
	}, jsonResponse(http.StatusOK, firstPage))

	_, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if err == nil || !strings.Contains(err.Error(), "start with '/'") {
		t.Errorf("RawPaginatedGET() error = %v, want malformed relative next link rejection", err)
	}
	if requests != 1 {
		t.Fatalf("transport requests = %d, want only the initial ASC request", requests)
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

func TestRawPaginatedGETRecognizesEquivalentNextURLs(t *testing.T) {
	const absolute = `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=loop&limit=1"}}`
	const relative = `{"data":[],"links":{"next":"/v1/apps?limit=1&cursor=loop"}}`
	requests := 0
	client := newTestClient(
		t, func(*http.Request) { requests++ },
		jsonResponse(http.StatusOK, absolute),
		jsonResponse(http.StatusOK, relative),
		jsonResponse(http.StatusOK, `{"data":[]}`),
	)

	_, err := client.RawPaginatedGET(context.Background(), "/v1/apps", nil)
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Errorf("RawPaginatedGET() error = %v, want ErrRepeatedPaginationURL", err)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want cycle detection before a third request", requests)
	}
}
