package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type reviewSubmissionsSurfaceResult struct {
	stdout   string
	stderr   string
	err      string
	method   string
	path     string
	rawQuery string
}

func TestReviewSubmissionsNestedListMatchesFlatSurface(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	surfaces := [][]string{
		{"review", "submissions-list"},
		{"review", "submissions", "list"},
	}
	results := make([]reviewSubmissionsSurfaceResult, 0, len(surfaces))
	for _, surface := range surfaces {
		args := append(
			append([]string{}, surface...),
			"--state", "READY_FOR_REVIEW",
			"--app", "app-1",
			"--platform", "IOS",
			"--include", "items",
			"--item-fields", "state",
			"--output", "json",
		)
		results = append(results, runReviewSubmissionsSurface(
			t, args, http.StatusOK,
			`{"data":[{"type":"reviewSubmissions","id":"submission-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}]}`,
		))
	}

	if !reflect.DeepEqual(results[0], results[1]) {
		t.Fatalf("flat result = %+v, nested result = %+v", results[0], results[1])
	}
	if results[0].method != http.MethodGet || results[0].path != "/v1/apps/app-1/reviewSubmissions" {
		t.Fatalf("request = %s %s, want GET /v1/apps/app-1/reviewSubmissions", results[0].method, results[0].path)
	}
	for _, query := range []string{
		"filter%5Bplatform%5D=IOS",
		"filter%5Bstate%5D=READY_FOR_REVIEW",
		"fields%5BreviewSubmissionItems%5D=state",
		"include=items",
	} {
		if !strings.Contains(results[0].rawQuery, query) {
			t.Fatalf("query %q is missing %q", results[0].rawQuery, query)
		}
	}
	if results[0].stderr != "" || results[0].err != "" || !strings.Contains(results[0].stdout, `"id":"submission-1"`) {
		t.Fatalf("unexpected successful result: %+v", results[0])
	}
}

func TestReviewSubmissionsNestedListMatchesFlatTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	response := `{"data":[{"type":"reviewSubmissions","id":"submission-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}]}`
	flat := runReviewSubmissionsSurface(
		t,
		[]string{"review", "submissions-list", "--app", "app-1", "--output", "table"},
		http.StatusOK,
		response,
	)
	nested := runReviewSubmissionsSurface(
		t,
		[]string{"review", "submissions", "list", "--output", "table", "--app", "app-1"},
		http.StatusOK,
		response,
	)

	if !reflect.DeepEqual(flat, nested) {
		t.Fatalf("flat result = %+v, nested result = %+v", flat, nested)
	}
	for _, value := range []string{"ID", "PLATFORM", "STATE", "SUBMISSION-1", "IOS", "READY_FOR_REVIEW"} {
		if !strings.Contains(strings.ToUpper(flat.stdout), value) {
			t.Fatalf("table output missing %q: %s", value, flat.stdout)
		}
	}
}

func TestReviewSubmissionsNestedListMatchesFlatAPIError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	response := `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"Not allowed."}]}`
	flat := runReviewSubmissionsSurface(
		t,
		[]string{"review", "submissions-list", "--app", "app-1"},
		http.StatusForbidden,
		response,
	)
	nested := runReviewSubmissionsSurface(
		t,
		[]string{"review", "submissions", "list", "--app", "app-1"},
		http.StatusForbidden,
		response,
	)

	if !reflect.DeepEqual(flat, nested) {
		t.Fatalf("flat result = %+v, nested result = %+v", flat, nested)
	}
	if flat.stdout != "" || !strings.Contains(flat.err, "review submissions-list") || !strings.Contains(flat.err, "Not allowed") {
		t.Fatalf("unexpected API failure result: %+v", flat)
	}
}

func TestReviewSubmissionsNestedListValidatesBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantLine string
	}{
		{name: "missing app", args: []string{"review", "submissions", "list"}, wantLine: "Error: --app or --global is required (or set ASC_APP_ID)"},
		{name: "invalid state", args: []string{"review", "submissions", "list", "--app", "app-1", "--state", "NOPE"}, wantLine: "Error: --state must be one of: READY_FOR_REVIEW, WAITING_FOR_REVIEW, IN_REVIEW, UNRESOLVED_ISSUES, CANCELING, COMPLETING, COMPLETE"},
		{name: "unexpected argument", args: []string{"review", "submissions", "list", "--app", "app-1", "unexpected"}, wantLine: "Error: unexpected positional arguments"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want usage error", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantLine+"\n") {
				t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantLine+"\n")
			}
			if strings.Count(stderr, test.wantLine) != 1 {
				t.Fatalf("stderr contains duplicate diagnostic: %q", stderr)
			}
		})
	}
}

func runReviewSubmissionsSurface(t *testing.T, args []string, status int, body string) reviewSubmissionsSurfaceResult {
	t.Helper()

	result := reviewSubmissionsSurfaceResult{}
	originalTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = originalTransport }()
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		result.method = req.Method
		result.path = req.URL.Path
		result.rawQuery = req.URL.RawQuery
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	result.stdout, result.stderr = captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			result.err = err.Error()
		}
	})
	return result
}
