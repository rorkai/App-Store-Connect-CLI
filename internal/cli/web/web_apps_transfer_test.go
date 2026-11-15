package web

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsTransferStatusOutputs(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	t.Setenv("ASC_APP_ID", "app-1")
	originalClient := newWebClientFn
	t.Cleanup(func() { newWebClientFn = originalClient })
	restore := SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restore)
	body := `{"data":{"type":"apps","id":"app-1","relationships":{"appTransferRequest":{"data":null}}},"included":[],"meta":{"future":"preserved"}}`
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/iris/v1/apps/app-1" || r.URL.Query().Get("include") != "appTransferRequest" {
				t.Errorf("request %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
	}
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebAppsTransferStatusCommand(d)
			if err := cmd.FlagSet.Parse([]string{"--output", format}); err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { runErr = cmd.Exec(context.Background(), nil) })
			if runErr != nil || stderr != "" {
				t.Fatalf("err=%v stderr=%s", runErr, stderr)
			}
			if format == "json" {
				var got, want any
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatal(err)
				}
				_ = json.Unmarshal([]byte(body), &want)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("changed JSON: %s", stdout)
				}
			} else if !strings.Contains(stdout, "app-1") || !strings.Contains(stdout, "none") || !strings.Contains(stdout, "unknown") {
				t.Fatalf("summary: %s", stdout)
			}
		})
	}
}
