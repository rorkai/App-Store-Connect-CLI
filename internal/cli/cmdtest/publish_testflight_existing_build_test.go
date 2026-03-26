package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishTestflightExistingBuildIDSkipsUpload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-1"`) {
				t.Fatalf("expected group assignment payload to include group-1, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-1",
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
	if !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploaded":false`) {
		t.Fatalf("expected uploaded=false in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notified":`) {
		t.Fatalf("expected notified to be omitted when --notify is not set, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to be omitted when --notify is not set, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDAllowsInternalGroup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-internal"`) {
				t.Fatalf("expected group assignment payload to include internal group, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
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
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDNotifyUsesBuildBetaNotificationsEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("expected no query string, got %q", req.URL.RawQuery)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-internal"`) {
				t.Fatalf("expected group assignment payload to include internal group, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildBetaNotifications" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read beta notification payload: %v", err)
			}
			if !strings.Contains(string(payload), `"type":"buildBetaNotifications"`) || !strings.Contains(string(payload), `"id":"build-1"`) {
				t.Fatalf("expected build beta notification payload for build-1, got %s", string(payload))
			}
			body := `{"data":{"type":"buildBetaNotifications","id":"notif-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
			"--notify",
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
	if requestCount != 5 {
		t.Fatalf("expected group lookup, build fetch, group assignment, build beta detail fetch, and beta notification; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":true`) {
		t.Fatalf("expected notified=true in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"manual"`) {
		t.Fatalf("expected notificationAction=manual in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDAddsInternalGroupWithAccessToAllBuilds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("expected no query string, got %q", req.URL.RawQuery)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-internal"`) {
				t.Fatalf("expected group assignment payload to include internal group, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 3 {
		t.Fatalf("expected group lookup, build fetch, and group assignment; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploaded":false`) {
		t.Fatalf("expected uploaded=false in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notified":`) {
		t.Fatalf("expected notified to be omitted when --notify is not set, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to be omitted when --notify is not set, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestPublishTestflightExistingBuildIDWithInternalAllBuildsGroupStillNotifies(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("expected no query string, got %q", req.URL.RawQuery)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-internal"`) {
				t.Fatalf("expected group assignment payload to include internal group, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildBetaNotifications" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read beta notification payload: %v", err)
			}
			if !strings.Contains(string(payload), `"type":"buildBetaNotifications"`) || !strings.Contains(string(payload), `"id":"build-1"`) {
				t.Fatalf("expected build beta notification payload for build-1, got %s", string(payload))
			}
			body := `{"data":{"type":"buildBetaNotifications","id":"notif-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
			"--notify",
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
	if requestCount != 5 {
		t.Fatalf("expected group lookup, build fetch, group assignment, build beta detail fetch, and beta notification; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":true`) {
		t.Fatalf("expected notified=true in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"manual"`) {
		t.Fatalf("expected notificationAction=manual in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDNotifySkipsManualNotificationWhenAutoNotifyEnabled(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("expected no query string, got %q", req.URL.RawQuery)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-internal"`) {
				t.Fatalf("expected group assignment payload to include internal group, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
			"--notify",
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
	if requestCount != 4 {
		t.Fatalf("expected group lookup, build fetch, group assignment, and build beta detail fetch; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":false`) {
		t.Fatalf("expected notified=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected notificationAction=auto_notify_enabled in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDAddsInternalAndExternalGroupsWhenInternalHasAllBuilds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}},{"type":"betaGroups","id":"group-external","attributes":{"name":"External","isInternalGroup":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("expected no query string, got %q", req.URL.RawQuery)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			bodyText := string(payload)
			if !strings.Contains(bodyText, `"id":"group-internal"`) {
				t.Fatalf("expected payload to include internal group, got %s", bodyText)
			}
			if !strings.Contains(bodyText, `"id":"group-external"`) {
				t.Fatalf("expected payload to include external group, got %s", bodyText)
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal,group-external",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 3 {
		t.Fatalf("expected group lookup, build fetch, and group assignment; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal","group-external"]`) {
		t.Fatalf("expected internal and external groups in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notified":`) {
		t.Fatalf("expected notified to be omitted when --notify is not set, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to be omitted when --notify is not set, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestPublishTestflightExistingBuildIDNotifyTreatsAutoNotifyConflictAsAlreadyEnabled(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("expected no query string, got %q", req.URL.RawQuery)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-internal"`) {
				t.Fatalf("expected group assignment payload to include internal group, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildBetaNotifications" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"errors":[{"code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"There is a problem with the request entity","detail":"Auto-notify already enabled"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
			"--notify",
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
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":false`) {
		t.Fatalf("expected notified=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected notificationAction=auto_notify_enabled in output, got %q", stdout)
	}
	if requestCount != 5 {
		t.Fatalf("expected 5 requests, got %d", requestCount)
	}
}

func TestPublishTestflightExistingBuildIDNotifyPreservesPartialSuccessMessageWhenNotificationFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildBetaNotifications" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"errors":[{"code":"INTERNAL_ERROR","title":"Service unavailable","detail":"email delivery temporarily unavailable"}]}`
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build", "build-1",
			"--group", "group-internal",
			"--notify",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `publish testflight: beta groups were added to build "build-1", but notifying testers failed`) {
		t.Fatalf("expected partial-success publish error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "failed to add groups") {
		t.Fatalf("did not expect misleading add-groups wrapper, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Service unavailable: email delivery temporarily unavailable") {
		t.Fatalf("expected underlying notification error, got %v", runErr)
	}
	if requestCount != 5 {
		t.Fatalf("expected 5 requests, got %d", requestCount)
	}
}

func TestPublishTestflightExistingBuildNumberResolvesAndWaits(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-1" {
				t.Fatalf("expected filter[app]=app-1, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "42" {
				t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
				t.Fatalf("expected filter[preReleaseVersion.platform]=IOS, got %q", query.Get("filter[preReleaseVersion.platform]"))
			}
			if query.Get("filter[processingState]") != "PROCESSING,FAILED,INVALID,VALID" {
				t.Fatalf("expected all processing states filter, got %q", query.Get("filter[processingState]"))
			}
			body := `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"PROCESSING"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-42" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"PROCESSING"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-42" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-42/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build-number", "42",
			"--group", "group-1",
			"--wait",
			"--poll-interval", "1ms",
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
	if !strings.Contains(stdout, `"buildId":"build-42"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploaded":false`) {
		t.Fatalf("expected uploaded=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"processingState":"VALID"`) {
		t.Fatalf("expected processingState VALID in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildNumberNotifyUsesBuildBetaNotificationsEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-1" {
				t.Fatalf("expected filter[app]=app-1, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "42" {
				t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
				t.Fatalf("expected filter[preReleaseVersion.platform]=IOS, got %q", query.Get("filter[preReleaseVersion.platform]"))
			}
			if query.Get("filter[processingState]") != "PROCESSING,FAILED,INVALID,VALID" {
				t.Fatalf("expected all processing states filter, got %q", query.Get("filter[processingState]"))
			}
			body := `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-42/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-1"`) {
				t.Fatalf("expected group assignment payload to include group-1, got %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-42/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-42","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildBetaNotifications" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read beta notification payload: %v", err)
			}
			if !strings.Contains(string(payload), `"type":"buildBetaNotifications"`) || !strings.Contains(string(payload), `"id":"build-42"`) {
				t.Fatalf("expected build beta notification payload for build-42, got %s", string(payload))
			}
			body := `{"data":{"type":"buildBetaNotifications","id":"notif-42"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "app-1",
			"--build-number", "42",
			"--group", "group-1",
			"--notify",
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
	if requestCount != 5 {
		t.Fatalf("expected build lookup, group assignment, build beta detail fetch, and beta notification; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"buildId":"build-42"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":true`) {
		t.Fatalf("expected notified=true in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"manual"`) {
		t.Fatalf("expected notificationAction=manual in output, got %q", stdout)
	}
}
