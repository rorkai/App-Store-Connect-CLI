package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
	if got, present := profileQuery["filter[platform]"]; present {
		t.Fatalf("profiles list sent filter[platform]=%q", got)
	}

	certificates := certificatesListQuerySurfaceStub(t)
	if _, certStderr, err := runCertificatesListQuerySurface(t, "certificates", "list", "--certificate-type", "IOS_DISTRIBUTION", "--output", "json"); err != nil {
		t.Fatalf("certificates list: %v (stderr=%q)", err, certStderr)
	}
	if got, present := certificates.query["filter[platform]"]; present {
		t.Fatalf("certificates list sent filter[platform]=%q", got)
	}
}

func TestProfilesCreateNameTooLongWritesUsageErrorBeforeClient(t *testing.T) {
	setupAuth(t)

	clientCalled := false
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalled = true
		return nil, errors.New("client factory must not run during profile-name validation")
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	name := strings.Repeat("n", 65)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"profiles", "create",
			"--name", name,
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--bundle", "BUNDLE_ID",
			"--certificate", "CERT_ID",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("run error = %v, want usage error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "profile name must be at most 64 characters; got 65") {
		t.Fatalf("stderr = %q, want the length-limit error", stderr)
	}
	if clientCalled {
		t.Fatal("client factory ran before profile-name validation")
	}
}
