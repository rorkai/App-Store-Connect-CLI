package cmdtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProfilesAndCertificatesListNeverSendPlatformFilter(t *testing.T) {
	setupAuth(t)

	var profileQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected path %s", req.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		profileQuery = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	installProfilesQueryTestClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"profiles", "list", "--profile-type", "IOS_APP_STORE", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("profiles list: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("profiles stderr = %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected profiles list stdout")
	}
	if got := profileQuery.Get("filter[platform]"); got != "" {
		t.Fatalf("profiles list sent filter[platform]=%q", got)
	}

	certificates := certificatesListQuerySurfaceStub(t)
	if _, certStderr, err := runCertificatesListQuerySurface(t, "certificates", "list", "--certificate-type", "IOS_DISTRIBUTION", "--output", "json"); err != nil {
		t.Fatalf("certificates list: %v (stderr=%q)", err, certStderr)
	}
	if got := certificates.query.Get("filter[platform]"); got != "" {
		t.Fatalf("certificates list sent filter[platform]=%q", got)
	}
}
