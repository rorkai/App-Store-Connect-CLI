package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSubscriptionsLocalizationsSyncUpsertsExactFields(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := make([]string, 0, 4)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionLocalizations" && req.URL.Query().Get("cursor") == "":
			if got := req.URL.Query().Get("fields[subscriptionLocalizations]"); got != "description,locale,name" {
				t.Fatalf("expected sparse localization fields, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Pro","description":"Current"}},{"type":"subscriptionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","name":"Pro","description":"Ancienne"}}],"links":{"next":"/v1/subscriptions/8000000001/subscriptionLocalizations?cursor=2"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionLocalizations" && req.URL.Query().Get("cursor") == "2":
			if got := req.URL.Query().Get("fields[subscriptionLocalizations]"); got != "description,locale,name" {
				t.Fatalf("expected sparse fields on page 2, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionLocalizations":
			var payload struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			want := map[string]any{"locale": "de-DE", "name": "Pro DE"}
			if !reflect.DeepEqual(payload.Data.Attributes, want) {
				t.Fatalf("expected exact create attributes %#v, got %#v", want, payload.Data.Attributes)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionLocalizations","id":"loc-de","attributes":{"locale":"de-DE","name":"Pro DE"}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionLocalizations/loc-fr":
			var payload struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode update payload: %v", err)
			}
			want := map[string]any{"description": ""}
			if !reflect.DeepEqual(payload.Data.Attributes, want) {
				t.Fatalf("expected clear-only update %#v, got %#v", want, payload.Data.Attributes)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","name":"Pro","description":""}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	input := writeLocalizationSyncInput(t, `{
  "fr_FR": {"description": ""},
  "en-US": {"name": "Pro", "description": "Current"},
  "de-DE": {"name": "Pro DE"}
}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var summary struct {
		Total     int `json:"total"`
		Created   int `json:"created"`
		Updated   int `json:"updated"`
		Unchanged int `json:"unchanged"`
		Failed    int `json:"failed"`
		Results   []struct {
			Locale string `json:"locale"`
		} `json:"results"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary: %v\n%s", err, stdout)
	}
	if summary.Total != 3 || summary.Created != 1 || summary.Updated != 1 || summary.Unchanged != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	locales := []string{summary.Results[0].Locale, summary.Results[1].Locale, summary.Results[2].Locale}
	if !reflect.DeepEqual(locales, []string{"de-DE", "en-US", "fr-FR"}) {
		t.Fatalf("expected deterministic locale order, got %v", locales)
	}
	wantRequests := []string{
		"GET /v1/subscriptions/8000000001/subscriptionLocalizations",
		"GET /v1/subscriptions/8000000001/subscriptionLocalizations",
		"POST /v1/subscriptionLocalizations",
		"PATCH /v1/subscriptionLocalizations/loc-fr",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("expected requests %v, got %v", wantRequests, requests)
	}
}

func TestSubscriptionsLocalizationsSyncReconcilesAmbiguousCreate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	creates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			reads++
			if reads == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Pro"}}],"links":{}}`), nil
		case http.MethodPost:
			creates++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	input := writeLocalizationSyncInput(t, `{"en-US":{"name":"Pro"}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var summary struct {
		Reconciled int `json:"reconciled"`
		Failed     int `json:"failed"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" || creates != 1 || reads != 2 {
		t.Fatalf("unexpected reconcile calls: reads=%d creates=%d stderr=%q", reads, creates, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Reconciled != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected reconcile summary: %+v", summary)
	}
}

func TestSubscriptionsLocalizationsSyncReconcilesAmbiguousUpdate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	patches := 0
	currentName := "Old"
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			reads++
			body := `{"data":[{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"` + currentName + `"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case http.MethodPatch:
			patches++
			currentName = "New"
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	input := writeLocalizationSyncInput(t, `{"en-US":{"name":"New"}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var summary struct {
		Reconciled int `json:"reconciled"`
		Failed     int `json:"failed"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" || reads != 2 || patches != 1 {
		t.Fatalf("unexpected reconcile calls: reads=%d patches=%d stderr=%q", reads, patches, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Reconciled != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected reconcile summary: %+v", summary)
	}
}

func TestSubscriptionsLocalizationsSyncContinuesAndWritesFailureArtifact(t *testing.T) {
	setupAuth(t)
	t.Chdir(t.TempDir())
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reads := 0
	creates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			reads++
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case http.MethodPost:
			creates++
			if creates == 1 {
				return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"bad localization"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"English"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	input := writeLocalizationSyncInput(t, `{"de-DE":{"name":"Deutsch"},"en-US":{"name":"English"}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	var summary struct {
		Created             int    `json:"created"`
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T %v", runErr, runErr)
	}
	if stderr != "" || creates != 2 || reads != 2 {
		t.Fatalf("expected continuation after failure, reads=%d creates=%d stderr=%q", reads, creates, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Created != 1 || summary.Failed != 1 || summary.FailureArtifactPath == "" {
		t.Fatalf("unexpected partial summary: %+v", summary)
	}
	payload, err := os.ReadFile(summary.FailureArtifactPath)
	if err != nil {
		t.Fatalf("read failure artifact: %v", err)
	}
	if !strings.Contains(string(payload), `"schemaVersion": 1`) || !strings.Contains(string(payload), `"locale": "de-DE"`) || !strings.Contains(string(payload), `"name": "Deutsch"`) {
		t.Fatalf("unexpected failure artifact: %s", payload)
	}
	var artifact struct {
		RetryInput map[string]map[string]string `json:"retryInput"`
	}
	if err := json.Unmarshal(payload, &artifact); err != nil {
		t.Fatalf("decode failure artifact: %v", err)
	}
	wantRetryInput := map[string]map[string]string{"de-DE": {"name": "Deutsch"}}
	if !reflect.DeepEqual(artifact.RetryInput, wantRetryInput) {
		t.Fatalf("expected replayable input %#v, got %#v", wantRetryInput, artifact.RetryInput)
	}
}

func TestSubscriptionsGroupsLocalizationsSyncClearsCustomAppName(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			if got := req.URL.Query().Get("fields[subscriptionGroupLocalizations]"); got != "customAppName,locale,name" {
				t.Fatalf("expected sparse group fields, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroupLocalizations","id":"group-loc-en","attributes":{"locale":"en-US","name":"Premium","customAppName":"Old"}}],"links":{}}`), nil
		case http.MethodPatch:
			var payload struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			want := map[string]any{"customAppName": ""}
			if !reflect.DeepEqual(payload.Data.Attributes, want) {
				t.Fatalf("expected clear-only group patch %#v, got %#v", want, payload.Data.Attributes)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionGroupLocalizations","id":"group-loc-en","attributes":{"locale":"en-US","name":"Premium","customAppName":""}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	input := writeLocalizationSyncInput(t, `{"en-US":{"customAppName":""}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "groups", "localizations", "sync", "--group-id", "group-1", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" || !strings.Contains(stdout, `"updated":1`) {
		t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestSubscriptionsLocalizationsSyncRejectsDuplicateCanonicalLocale(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	input := writeLocalizationSyncInput(t, `{"en_US":{"name":"One"},"en-US":{"name":"Two"}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %T %v", err, err)
		}
	})
	if stdout != "" || !strings.Contains(stderr, `duplicate canonical locale "en-US"`) {
		t.Fatalf("unexpected validation output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestSubscriptionsLocalizationSyncRejectsUnsafeInvocationBeforeHTTP(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		input   string
		wantErr string
	}{
		{
			name:    "subscription positional",
			args:    []string{"subscriptions", "localizations", "sync", "stray", "--subscription-id", "8000000001"},
			input:   `{"en-US":{"name":"English"}}`,
			wantErr: "does not accept positional arguments",
		},
		{
			name:    "group positional",
			args:    []string{"subscriptions", "groups", "localizations", "sync", "stray", "--group-id", "group-1"},
			input:   `{"en-US":{"name":"English"}}`,
			wantErr: "does not accept positional arguments",
		},
		{
			name:    "unsupported output",
			args:    []string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--output", "yaml"},
			input:   `{"en-US":{"name":"English"}}`,
			wantErr: "unsupported format: yaml",
		},
		{
			name:    "pretty table",
			args:    []string{"subscriptions", "groups", "localizations", "sync", "--group-id", "group-1", "--output", "table", "--pretty"},
			input:   `{"en-US":{"name":"English"}}`,
			wantErr: "--pretty is only valid with JSON output",
		},
		{
			name:    "subscription blank name",
			args:    []string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001"},
			input:   `{"en-US":{"name":"  "}}`,
			wantErr: `field "name" must not be empty`,
		},
		{
			name:    "group blank name",
			args:    []string{"subscriptions", "groups", "localizations", "sync", "--group-id", "group-1"},
			input:   `{"en-US":{"name":""}}`,
			wantErr: `field "name" must not be empty`,
		},
		{
			name:    "trailing second value",
			args:    []string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001"},
			input:   `{"en-US":{"name":"English"}} {}`,
			wantErr: "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			})

			input := writeLocalizationSyncInput(t, tt.input)
			args := append(append([]string{}, tt.args...), "--input", input)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse: %v", err)
				}
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %T %v", err, err)
				}
			})
			if stdout != "" || !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected %q, stdout=%q stderr=%q", tt.wantErr, stdout, stderr)
			}
		})
	}
}

func TestSubscriptionsGroupsLocalizationsSyncReassertsFieldsOnNextPage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	pages := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		pages++
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) < 350*time.Millisecond {
			t.Fatalf("page %d expected fresh request deadline, remaining=%s", pages, time.Until(deadline))
		}
		if got := req.URL.Query().Get("fields[subscriptionGroupLocalizations]"); got != "customAppName,locale,name" {
			t.Fatalf("page %d missing sparse fields: %q", pages, got)
		}
		if pages == 1 {
			time.Sleep(300 * time.Millisecond)
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations?cursor=2"}}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroupLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Premium"}}],"links":{}}`), nil
	})

	input := writeLocalizationSyncInput(t, `{"en-US":{"name":"Premium"}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "groups", "localizations", "sync", "--group-id", "group-1", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if pages != 2 || stderr != "" || !strings.Contains(stdout, `"unchanged":1`) {
		t.Fatalf("unexpected pagination result: pages=%d stdout=%q stderr=%q", pages, stdout, stderr)
	}
}

func TestRunSubscriptionsLocalizationsSyncExitCodes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupAuth(t)
		originalTransport := http.DefaultTransport
		t.Cleanup(func() { http.DefaultTransport = originalTransport })
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"English"}}],"links":{}}`), nil
		})
		input := writeLocalizationSyncInput(t, `{"en-US":{"name":"English"}}`)
		stdout, stderr := captureOutput(t, func() {
			code := rootcmd.Run([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}, "1.2.3")
			if code != rootcmd.ExitSuccess {
				t.Fatalf("expected exit %d, got %d", rootcmd.ExitSuccess, code)
			}
		})
		if stderr != "" || !strings.Contains(stdout, `"unchanged":1`) {
			t.Fatalf("unexpected success output: stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		setupAuth(t)
		t.Chdir(t.TempDir())
		originalTransport := http.DefaultTransport
		t.Cleanup(func() { http.DefaultTransport = originalTransport })
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodGet {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
			}
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"invalid"}]}`), nil
		})
		input := writeLocalizationSyncInput(t, `{"en-US":{"name":"English"}}`)
		stdout, stderr := captureOutput(t, func() {
			code := rootcmd.Run([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}, "1.2.3")
			if code != rootcmd.ExitError {
				t.Fatalf("expected exit %d, got %d", rootcmd.ExitError, code)
			}
		})
		if stderr != "" || !strings.Contains(stdout, `"failed":1`) {
			t.Fatalf("unexpected failure output: stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("usage", func(t *testing.T) {
		setupAuth(t)
		input := writeLocalizationSyncInput(t, `{"en-US":{"name":"English"}}`)
		stdout, stderr := captureOutput(t, func() {
			code := rootcmd.Run([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "yaml"}, "1.2.3")
			if code != rootcmd.ExitUsage {
				t.Fatalf("expected exit %d, got %d", rootcmd.ExitUsage, code)
			}
		})
		if stdout != "" || !strings.Contains(stderr, "unsupported format: yaml") {
			t.Fatalf("unexpected usage output: stdout=%q stderr=%q", stdout, stderr)
		}
	})
}

func TestSubscriptionsLocalizationsSyncBuiltBinaryInvalidOutputExitUsage(t *testing.T) {
	input := writeLocalizationSyncInput(t, `{"en-US":{"name":"English"}}`)
	assertUsageExit(t, []string{
		"subscriptions", "localizations", "sync",
		"--subscription-id", "8000000001",
		"--input", input,
		"--output", "yaml",
	}, "unsupported format: yaml")
}

func TestSubscriptionsLocalizationSyncPreflightsEveryCreateBeforeMutating(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		input     string
		listPath  string
		wantError string
	}{
		{
			name:      "subscription",
			args:      []string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001"},
			input:     `{"de-DE":{"name":"Deutsch"},"en-US":{"description":"Missing name"}}`,
			listPath:  "/v1/subscriptions/8000000001/subscriptionLocalizations",
			wantError: `locale "en-US" does not exist remotely and requires a non-empty name`,
		},
		{
			name:      "group",
			args:      []string{"subscriptions", "groups", "localizations", "sync", "--group-id", "group-1"},
			input:     `{"de-DE":{"name":"Deutsch"},"en-US":{"customAppName":"Missing name"}}`,
			listPath:  "/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations",
			wantError: `locale "en-US" does not exist remotely and requires a non-empty name`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != tt.listPath {
					t.Fatalf("preflight must not mutate: %s %s", req.Method, req.URL.Path)
				}
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
			})

			input := writeLocalizationSyncInput(t, tt.input)
			args := append(append([]string{}, tt.args...), "--input", input)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse: %v", err)
				}
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %T %v", err, err)
				}
			})
			if stdout != "" || !strings.Contains(stderr, tt.wantError) || requests != 1 {
				t.Fatalf("unexpected preflight result: requests=%d stdout=%q stderr=%q", requests, stdout, stderr)
			}
		})
	}
}

func TestSubscriptionsLocalizationsSyncReportsArtifactWriteFailure(t *testing.T) {
	setupAuth(t)
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".asc", []byte("blocks report directory"), 0o600); err != nil {
		t.Fatalf("write report blocker: %v", err)
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case http.MethodPost:
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"bad localization"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	input := writeLocalizationSyncInput(t, `{"en-US":{"name":"English"}}`)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	var summary struct {
		FailureArtifactPath  string `json:"failureArtifactPath"`
		FailureArtifactError string `json:"failureArtifactError"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T %v", runErr, runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.FailureArtifactPath != "" || summary.FailureArtifactError == "" {
		t.Fatalf("expected retained artifact error without path, got %+v", summary)
	}
}

func TestSubscriptionsLocalizationsSyncRendersTableAndMarkdown(t *testing.T) {
	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			setupAuth(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"English"}}],"links":{}}`), nil
			})

			input := writeLocalizationSyncInput(t, `{"en-US":{"name":"English"}}`)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"subscriptions", "localizations", "sync", "--subscription-id", "8000000001", "--input", input, "--output", format}); err != nil {
					t.Fatalf("parse: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run: %v", err)
				}
			})
			if stderr != "" || !strings.Contains(stdout, "Target ID") || !strings.Contains(stdout, "Locale") || !strings.Contains(stdout, "en-US") {
				t.Fatalf("unexpected %s output: stdout=%q stderr=%q", format, stdout, stderr)
			}
			if format == "markdown" && !strings.Contains(stdout, "|") {
				t.Fatalf("expected markdown table, got %q", stdout)
			}
		})
	}
}

func writeLocalizationSyncInput(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "localizations.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write localization input: %v", err)
	}
	return path
}
