package signing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveSigningFilesDeletesPartialArtifacts(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "App.mobileprovision")
	certPath := filepath.Join(dir, "cert.cer")
	if err := os.WriteFile(profilePath, []byte("profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, []byte("cert"), 0o600); err != nil {
		t.Fatal(err)
	}

	removeSigningFiles([]string{profilePath, certPath})

	for _, path := range []string{profilePath, certPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("partial artifact %s still present: %v", path, err)
		}
	}
}

func TestBundleIdentifierMatches(t *testing.T) {
	tests := []struct {
		parent    string
		candidate string
		expand    bool
		want      bool
	}{
		{parent: "com.app", candidate: "com.app", expand: false, want: true},
		{parent: "com.app", candidate: "com.app.widget", expand: false, want: false},
		{parent: "com.app", candidate: "com.app.widget", expand: true, want: true},
		{parent: "com.app", candidate: "com.app.clip", expand: true, want: true},
		{parent: "com.app", candidate: "com.apple.other", expand: true, want: false},
		{parent: "com.example.*", candidate: "com.example.app", expand: true, want: false},
	}
	for _, test := range tests {
		if got := bundleIdentifierMatches(test.parent, test.candidate, test.expand); got != test.want {
			t.Fatalf("parent %q candidate %q expand %v = %v, want %v", test.parent, test.candidate, test.expand, got, test.want)
		}
	}
}

func TestListSigningBundleIDsFiltersExtensionsClientSide(t *testing.T) {
	var sawExactFilter bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Query().Get("filter[identifier]") == "com.app" {
			sawExactFilter = true
			_, _ = io.WriteString(w, `{"data":[{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.app","name":"App","platform":"IOS"}}],"links":{}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":[
			{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.app","name":"App","platform":"IOS"}},
			{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.app.widget","name":"Widget","platform":"IOS"}},
			{"type":"bundleIds","id":"id-clip","attributes":{"identifier":"com.app.clip","name":"Clip","platform":"IOS"}},
			{"type":"bundleIds","id":"id-other","attributes":{"identifier":"com.apple.other","name":"Other","platform":"IOS"}}
		],"links":{}}`)
	}))
	t.Cleanup(server.Close)
	client := newSigningFetchServerTestClient(t, server)

	matched, err := listSigningBundleIDs(context.Background(), client, "com.app", true)
	if err != nil {
		t.Fatalf("listSigningBundleIDs: %v", err)
	}
	if !sawExactFilter {
		t.Fatal("expected filter[identifier]=com.app")
	}
	got := make([]string, 0, len(matched))
	for _, item := range matched {
		got = append(got, item.Attributes.Identifier)
	}
	want := []string{"com.app", "com.app.widget", "com.app.clip"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("matched = %v, want %v", got, want)
	}
}
