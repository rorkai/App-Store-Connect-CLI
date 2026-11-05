package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	appTesterMetricsObjectFormBody        = `{"data":[{"type":"appsBetaTesterUsages","dataPoints":[{"start":"2026-08-01T00:00:00Z","end":"2026-08-02T00:00:00Z","values":{"sessionCount":12,"crashCount":1,"feedbackCount":2}}],"dimensions":{"betaTesters":{"data":{"type":"betaTesters","id":"tester-1"}}}}],"links":{"self":"https://api.example.test/v1/apps/app-123/metrics/betaTesterUsages"}}`
	appTesterMetricsStringFormBody        = `{"data":[{"type":"appsBetaTesterUsages","dataPoints":[{"start":"2026-08-01T00:00:00Z","end":"2026-08-02T00:00:00Z","values":{"sessionCount":12,"crashCount":1,"feedbackCount":2}}],"dimensions":{"betaTesters":{"data":"tester-1"}}}],"links":{"self":"https://api.example.test/v1/apps/app-123/metrics/betaTesterUsages"}}`
	appTesterMetricsObjectFormResolveBody = `{
  "data": [
    {
      "dataPoints": [{"start": "2026-08-01", "end": "2026-08-07", "values": {"crashCount": 0, "sessionCount": 3, "feedbackCount": 1}}],
      "dimensions": {"betaTesters": {"data": {"type": "betaTesters", "id": "tester-1"}}}
    }
  ],
  "included": [
    {"type": "betaTesters", "id": "tester-1", "attributes": {"firstName": "Ada", "lastName": "Lovelace", "email": "ada@example.com", "inviteType": "EMAIL", "state": "INSTALLED"}}
  ],
  "links": {"next": ""}
}`
)

func stubAppTesterUsages(t *testing.T, body string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/metrics/betaTesterUsages" {
			t.Fatalf("expected metrics path, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
}

func runAppTesters(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr = captureOutput(t, func() {
		if parseErr := root.Parse(args); parseErr != nil {
			t.Fatalf("parse error: %v", parseErr)
		}
		err = root.Run(context.Background())
	})
	return stdout, stderr, err
}

func TestTestFlightMetricsAppTestersTableRendersRowsForBothDimensionShapes(t *testing.T) {
	for name, body := range map[string]string{"object form": appTesterMetricsObjectFormBody, "string form": appTesterMetricsStringFormBody} {
		t.Run(name, func(t *testing.T) {
			stubAppTesterUsages(t, body)

			stdout, stderr, err := runAppTesters(t, "testflight", "metrics", "app-testers", "--app", "app-123", "--output", "table")
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			for _, want := range []string{"Tester ID", "Start", "End", "Sessions", "Crashes", "Feedback", "tester-1", "2026-08-01T00:00:00Z", "12", "1", "2"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected table output to contain %q, got %q", want, stdout)
				}
			}
		})
	}
}

func TestTestFlightMetricsAppTestersJSONMatchesStubForBothDimensionShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "object form", body: appTesterMetricsObjectFormBody},
		{name: "string form", body: appTesterMetricsStringFormBody},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stubAppTesterUsages(t, test.body)

			stdout, stderr, err := runAppTesters(t, "testflight", "metrics", "app-testers", "--app", "app-123", "--output", "json")
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			want := test.body + "\n"
			if !bytes.Equal([]byte(stdout), []byte(want)) {
				t.Fatalf("JSON output drifted from Apple envelope\n got: %q\nwant: %q", stdout, want)
			}
		})
	}
}

func TestTestFlightMetricsAppTestersObjectFormResolveTesters(t *testing.T) {
	stubAppTesterUsages(t, appTesterMetricsObjectFormResolveBody)

	stdout, stderr, err := runAppTesters(t, "testflight", "metrics", "app-testers", "--app", "app-123", "--resolve-testers")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var printed struct {
		Data    []json.RawMessage `json:"data"`
		Testers map[string]struct {
			ID         string `json:"id"`
			Email      string `json:"email"`
			FirstName  string `json:"firstName"`
			LastName   string `json:"lastName"`
			State      string `json:"state"`
			InviteType string `json:"inviteType"`
		} `json:"testers"`
	}
	if unmarshalErr := json.Unmarshal([]byte(stdout), &printed); unmarshalErr != nil {
		t.Fatalf("failed to parse printed JSON: %v\noutput: %q", unmarshalErr, stdout)
	}
	if len(printed.Data) != 1 {
		t.Fatalf("expected 1 data row, got %d", len(printed.Data))
	}
	tester, ok := printed.Testers["tester-1"]
	if !ok {
		t.Fatalf("expected tester-1 in testers sidecar, got %q", stdout)
	}
	if tester.ID != "tester-1" || tester.Email != "ada@example.com" || tester.FirstName != "Ada" || tester.LastName != "Lovelace" || tester.State != "INSTALLED" || tester.InviteType != "EMAIL" {
		t.Fatalf("unexpected tester-1 sidecar entry: %+v", tester)
	}
}

func TestTestFlightTestersMetricsTableDoesNotUseAppTestersRenderer(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	body := `{"data":[{"type":"betaTesterUsages","dataPoints":[{"start":"2026-08-01T00:00:00Z","end":"2026-08-02T00:00:00Z","values":{"sessionCount":9}}],"dimensions":{"apps":{"data":{"type":"apps","id":"app-1"}}}}],"links":{}}`
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-1/metrics/betaTesterUsages" {
			t.Fatalf("expected per-tester metrics path, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "metrics", "--tester-id", "tester-1", "--app", "app-123", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "Tester ID") {
		t.Fatalf("per-tester metrics table must not use the app-testers renderer, got %q", stdout)
	}
	if !strings.Contains(stdout, "App ID") || !strings.Contains(stdout, "app-1") || strings.Contains(stdout, `"apps"`) {
		t.Fatalf("expected per-tester table keyed by app, got %q", stdout)
	}
}
