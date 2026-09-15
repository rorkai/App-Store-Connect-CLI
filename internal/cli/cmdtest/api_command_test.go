package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAPIHelpDescribesPassthroughContract(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"api", "--help"}, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	for _, want := range []string{
		"asc api <METHOD> <PATH> [flags]",
		"--query",
		"--body",
		"--body-file",
		"--paginate",
		"--confirm",
		"--allow-unknown-path",
		"--pretty",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout)
		}
	}
}

func TestAPIUsageErrorsRunBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing method and path",
			args:    []string{"api"},
			wantErr: "api: METHOD and PATH are required",
		},
		{
			name:    "missing path",
			args:    []string{"api", "GET"},
			wantErr: "api: METHOD and PATH are required",
		},
		{
			name:    "too many positionals",
			args:    []string{"api", "GET", "/v1/apps", "extra"},
			wantErr: "api: unexpected argument \"extra\"",
		},
		{
			name:    "unsupported method",
			args:    []string{"api", "PUT", "/v1/apps"},
			wantErr: "api: METHOD must be one of GET, POST, PATCH, DELETE",
		},
		{
			name:    "relative path",
			args:    []string{"api", "GET", "v1/apps"},
			wantErr: "api: PATH must start with /v1/, /v2/, or /v3/",
		},
		{
			name:    "foreign host",
			args:    []string{"api", "GET", "https://example.com/v1/apps"},
			wantErr: "api: PATH must be an App Store Connect URL",
		},
		{
			name:    "unknown path lists nearest operations",
			args:    []string{"api", "GET", "/v1/apps/123/buildz"},
			wantErr: "api: GET /v1/apps/123/buildz is not in the embedded schema index",
		},
		{
			name:    "unknown method for known path",
			args:    []string{"api", "POST", "/v1/apps/123/builds", "--confirm"},
			wantErr: "api: POST /v1/apps/123/builds is not in the embedded schema index",
		},
		{
			name:    "mutation without confirm",
			args:    []string{"api", "PATCH", "/v1/apps/123", "--body", `{"data":{}}`},
			wantErr: "--confirm is required for PATCH requests",
		},
		{
			name:    "paginate on mutation",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--paginate"},
			wantErr: "api: --paginate is only supported for GET requests",
		},
		{
			name:    "query on mutation",
			args:    []string{"api", "DELETE", "/v1/betaGroups/123", "--confirm", "--query", "limit=1"},
			wantErr: "api: --query is only supported for GET requests",
		},
		{
			name:    "body on GET",
			args:    []string{"api", "GET", "/v1/apps", "--body", `{"data":{}}`},
			wantErr: "api: --body is only supported for POST, PATCH, and DELETE requests",
		},
		{
			name:    "body and body-file together",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `{}`, "--body-file", "payload.json"},
			wantErr: "api: --body and --body-file are mutually exclusive",
		},
		{
			name:    "malformed query pair",
			args:    []string{"api", "GET", "/v1/apps", "--query", "limit"},
			wantErr: "api: --query must be key=value",
		},
		{
			name:    "invalid inline body",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `{"data":`},
			wantErr: "api: --body: invalid JSON",
		},
		{
			name:    "inline body null is not an object",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `null`},
			wantErr: "api: --body must be a JSON object",
		},
		{
			name:    "inline body array is not an object",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `[{"data":{}}]`},
			wantErr: "api: --body must be a JSON object",
		},
		{
			name:    "inline body scalar is not an object",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `"data"`},
			wantErr: "api: --body must be a JSON object",
		},
		{
			name:    "control character in path",
			args:    []string{"api", "GET", "/v1/apps/12\n3/builds"},
			wantErr: "api: PATH contains a control character",
		},
		{
			name:    "gzip sales report endpoint rejected",
			args:    []string{"api", "GET", "/v1/salesReports", "--query", "filter[vendorNumber]=123"},
			wantErr: "api: GET /v1/salesReports responds with application/a-gzip rather than a JSON envelope, which `asc api` cannot pass through; use `asc analytics sales` instead",
		},
		{
			name:    "gzip finance report endpoint rejected",
			args:    []string{"api", "GET", "/v1/financeReports", "--query", "filter[vendorNumber]=123"},
			wantErr: "api: GET /v1/financeReports responds with application/a-gzip rather than a JSON envelope, which `asc api` cannot pass through; use `asc finance reports` instead",
		},
		{
			name:    "csv offer code values endpoint rejected",
			args:    []string{"api", "GET", "/v1/subscriptionOfferCodeOneTimeUseCodes/CODE_ID/values"},
			wantErr: "api: GET /v1/subscriptionOfferCodeOneTimeUseCodes/{id}/values responds with text/csv rather than a JSON envelope, which `asc api` cannot pass through; use `asc subscriptions offers offer-codes values` instead",
		},
		{
			name:    "vendor json metrics endpoint rejected",
			args:    []string{"api", "GET", "/v1/apps/123/perfPowerMetrics"},
			wantErr: "api: GET /v1/apps/{id}/perfPowerMetrics responds with application/vnd.apple.xcode-metrics+json rather than a JSON envelope, which `asc api` cannot pass through; use `asc performance metrics list` instead",
		},
		{
			name:    "non json endpoint rejected even with allow-unknown-path",
			args:    []string{"api", "GET", "/v1/diagnosticSignatures/SIG_ID/logs", "--allow-unknown-path"},
			wantErr: "api: GET /v1/diagnosticSignatures/{id}/logs responds with application/vnd.apple.diagnostic-logs+json rather than a JSON envelope, which `asc api` cannot pass through; use `asc performance diagnostics view` instead",
		},
		{
			name:    "table output rejected",
			args:    []string{"api", "GET", "/v1/apps", "--output", "table"},
			wantErr: "--output must be one of: json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			assertUsageExit(t, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before validation finished")
			}
		})
	}
}

func TestAPIUnknownPathSuggestsNearestOperations(t *testing.T) {
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, errors.New("client factory must not run during validation")
	})
	defer restore()

	_, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"api", "GET", "/v1/apps/123/buildz"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if !strings.Contains(stderr, "GET /v1/apps/{id}/builds") {
		t.Fatalf("stderr = %q, want nearest operation GET /v1/apps/{id}/builds", stderr)
	}
	if !strings.Contains(stderr, "--allow-unknown-path") {
		t.Fatalf("stderr = %q, want --allow-unknown-path hint", stderr)
	}
}

func TestAPIGetPrintsEnvelopeUnmodified(t *testing.T) {
	setupAuth(t)

	const body = `{"data":[{"type":"apps","id":"123","attributes":{"name":"Demo"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?limit=1"},"meta":{"paging":{"total":1,"limit":1}}}`
	requests := newRequestLog(1)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.RequestURI())
		if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		args := []string{"api", "get", "/v1/apps", "--query", "limit=1", "--query", "fields[apps]=name"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != body+"\n" {
		t.Fatalf("stdout = %q, want unmodified envelope", stdout)
	}
	want := []string{"GET /v1/apps?fields%5Bapps%5D=name&limit=1"}
	if got := requests.Snapshot(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", got, want)
	}
}

func TestAPIGetAcceptsFullURLWithQueryAndPretty(t *testing.T) {
	setupAuth(t)

	requests := newRequestLog(1)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.RequestURI())
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		args := []string{"api", "--pretty", "GET", "https://api.appstoreconnect.apple.com/v1/apps?limit=2", "--query", "sort=name"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != "{\n  \"data\": []\n}\n" {
		t.Fatalf("stdout = %q, want pretty JSON", stdout)
	}
	if got := requests.Snapshot(); len(got) != 1 || got[0] != "GET /v1/apps?limit=2&sort=name" {
		t.Fatalf("requests = %v, want merged query", got)
	}
}

func TestAPIPaginateMergesCollectionPages(t *testing.T) {
	setupAuth(t)

	pages := map[string]string{
		"/v1/apps?limit=1":            `{"data":[{"type":"apps","id":"1"}],"included":[{"type":"builds","id":"b1"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?limit=1","next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=1"},"meta":{"paging":{"total":2,"limit":1}}}`,
		"/v1/apps?cursor=two&limit=1": `{"data":[{"type":"apps","id":"2"}],"included":[{"type":"builds","id":"b1"},{"type":"builds","id":"b2"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=1"},"meta":{"paging":{"total":2,"limit":1}}}`,
	}
	requests := newRequestLog(2)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.URL.RequestURI())
		body, ok := pages[req.URL.RequestURI()]
		if !ok {
			t.Errorf("unexpected request %s", req.URL.RequestURI())
			body = `{"errors":[{"status":"404"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		args := []string{"api", "GET", "/v1/apps", "--query", "limit=1", "--paginate"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if got := requests.Snapshot(); len(got) != 2 {
		t.Fatalf("requests = %v, want two pages", got)
	}

	var merged struct {
		Data     []map[string]string `json:"data"`
		Included []map[string]string `json:"included"`
		Links    map[string]string   `json:"links"`
		Meta     json.RawMessage     `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &merged); err != nil {
		t.Fatalf("unmarshal merged output: %v\nstdout=%s", err, stdout)
	}
	if len(merged.Data) != 2 || merged.Data[0]["id"] != "1" || merged.Data[1]["id"] != "2" {
		t.Fatalf("data = %v, want both pages in order", merged.Data)
	}
	if len(merged.Included) != 2 || merged.Included[0]["id"] != "b1" || merged.Included[1]["id"] != "b2" {
		t.Fatalf("included = %v, want deduplicated union", merged.Included)
	}
	if _, ok := merged.Links["next"]; ok {
		t.Fatalf("links = %v, want next removed", merged.Links)
	}
	if merged.Links["self"] != "https://api.appstoreconnect.apple.com/v1/apps?limit=1" {
		t.Fatalf("links = %v, want first page self link", merged.Links)
	}
	if string(merged.Meta) != `{"paging":{"total":2,"limit":1}}` {
		t.Fatalf("meta = %s, want first page meta", merged.Meta)
	}
}

func TestAPIPaginateRejectsNonCollectionResponse(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"apps","id":"1"}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		args := []string{"api", "GET", "/v1/apps/1", "--paginate"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--paginate requires a collection response with a data array") {
		t.Fatalf("stderr = %q, want collection error", stderr)
	}
}

func TestAPIRejectsMalformedJSONResponse(t *testing.T) {
	// `json` is the command's only output format, so a successful response that
	// is not JSON must fail rather than hand machine consumers invalid bytes.
	for _, pretty := range []bool{false, true} {
		name := "plain"
		if pretty {
			name = "pretty"
		}
		t.Run(name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":[`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			args := []string{"api", "GET", "/v1/apps"}
			if pretty {
				args = append(args, "--pretty")
			}
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(args, "1.2.3"); code == rootcmd.ExitSuccess {
					t.Fatalf("exit code = %d, want a failure", code)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "response is not valid JSON") {
				t.Fatalf("stderr = %q, want an invalid JSON diagnostic", stderr)
			}
		})
	}
}

func TestAPIPostSendsBodyWithConfirm(t *testing.T) {
	setupAuth(t)

	const requestBody = `{"data":{"type":"betaGroups","attributes":{"name":"QA"},"relationships":{"app":{"data":{"type":"apps","id":"123"}}}}}`
	const responseBody = `{"data":{"type":"betaGroups","id":"g1"}}`
	var gotBody string
	var gotContentType string
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.RequestURI() != "/v1/betaGroups" {
			t.Errorf("request = %s %s, want POST /v1/betaGroups", req.Method, req.URL.RequestURI())
		}
		data, _ := io.ReadAll(req.Body)
		gotBody = string(data)
		gotContentType = req.Header.Get("Content-Type")
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		args := []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", requestBody}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != responseBody+"\n" {
		t.Fatalf("stdout = %q, want response body", stdout)
	}
	if gotBody != requestBody {
		t.Fatalf("request body = %q, want %q", gotBody, requestBody)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", gotContentType)
	}
}

func TestAPIPatchReadsBodyFromFile(t *testing.T) {
	setupAuth(t)

	const requestBody = `{"data":{"type":"apps","id":"123","attributes":{"primaryLocale":"en-US"}}}`
	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payloadPath, []byte(requestBody), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "body-file", args: []string{"api", "PATCH", "/v1/apps/123", "--confirm", "--body-file", payloadPath}},
		{name: "body at-file", args: []string{"api", "PATCH", "/v1/apps/123", "--confirm", "--body", "@" + payloadPath}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotBody string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				data, _ := io.ReadAll(req.Body)
				gotBody = string(data)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"apps","id":"123"}}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if gotBody != requestBody {
				t.Fatalf("request body = %q, want %q", gotBody, requestBody)
			}
		})
	}
}

func TestAPIDeleteEmptyResponsePrintsNothing(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete || req.URL.RequestURI() != "/v1/betaGroups/g1" {
			t.Errorf("request = %s %s, want DELETE /v1/betaGroups/g1", req.Method, req.URL.RequestURI())
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"api", "DELETE", "/v1/betaGroups/g1", "--confirm"}, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want both empty", stdout, stderr)
	}
}

func TestAPIAllowUnknownPathSkipsSchemaCheck(t *testing.T) {
	setupAuth(t)

	requests := newRequestLog(1)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.RequestURI())
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	_, stderr := captureOutput(t, func() {
		args := []string{"api", "GET", "/v1/notYetIndexed", "--allow-unknown-path"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if got := requests.Snapshot(); len(got) != 1 || got[0] != "GET /v1/notYetIndexed" {
		t.Fatalf("requests = %v, want the unindexed path", got)
	}
}

func TestAPIRendersAppleErrorsWithStatusExitCode(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '999'"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"api", "GET", "/v1/apps/999"}, "1.2.3"); code != rootcmd.ExitNotFound {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitNotFound)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "There is no resource of type 'apps' with id '999'") {
		t.Fatalf("stderr = %q, want Apple error detail", stderr)
	}
}
