package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// `subscriptions list --app` follows every group and subscription page whether
// or not --paginate is passed.
func TestSubscriptionsListAppAggregatesEveryPage(t *testing.T) {
	for name, extraArgs := range map[string][]string{"default": nil, "paginate": {"--paginate"}} {
		t.Run(name, func(t *testing.T) {
			setupAuth(t)

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.URL.Query().Get("cursor") == "":
					body := `{"data":[{"type":"subscriptionGroups","id":"group-1"}],"links":{"next":"/v1/apps/app-1/subscriptionGroups?cursor=group-page-2"}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.URL.Query().Get("cursor") == "group-page-2":
					body := `{"data":[{"type":"subscriptionGroups","id":"group-2"}],"links":{}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.URL.Query().Get("cursor") == "":
					body := `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}],"links":{"next":"/v1/subscriptionGroups/group-1/subscriptions?cursor=sub-page-2"}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.URL.Query().Get("cursor") == "sub-page-2":
					body := `{"data":[{"type":"subscriptions","id":"sub-1b","attributes":{"name":"Weekly","productId":"com.example.weekly"}}],"links":{}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-2/subscriptions":
					body := `{"data":[{"type":"subscriptions","id":"sub-2","attributes":{"name":"Yearly","productId":"com.example.yearly"}}],"links":{}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"subscriptions", "list", "--app", "app-1", "--output", "json"}, extraArgs...)
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"id":"sub-1"`) || !strings.Contains(stdout, `"id":"sub-1b"`) || !strings.Contains(stdout, `"id":"sub-2"`) {
				t.Fatalf("expected subscriptions from every group and subscription page, got %q", stdout)
			}
		})
	}
}

func TestSubscriptionsListAppPreservesEmptyDataArray(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body := `{"data":[],"links":{}}`
		return jsonHTTPResponse(http.StatusOK, body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "list",
			"--app", "app-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data array, got %q", stdout)
	}
}

func TestSubscriptionsListGroupIDIgnoresDefaultAppEnv(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "app-default")

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body := `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}],"links":{}}`
		return jsonHTTPResponse(http.StatusOK, body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "list",
			"--group-id", "group-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sub-1"`) {
		t.Fatalf("expected group-scoped subscription response, got %q", stdout)
	}
}
